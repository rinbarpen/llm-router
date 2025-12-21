from __future__ import annotations

import asyncio
from pathlib import Path
from typing import Any, Dict

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class TransformersProviderClient(BaseProviderClient):
    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        self._pipelines: Dict[int, Any] = {}
        self._locks: Dict[int, asyncio.Lock] = {}

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if request.stream:
            raise ProviderError("Transformers Provider 暂不支持流式输出")

        generator = await self._ensure_pipeline(model)
        prompt = request.prompt or self._messages_to_prompt(request)
        params = self.merge_parameters(model, request)

        try:
            outputs = await asyncio.to_thread(generator, prompt, **params)
        except Exception as exc:  # pragma: no cover - delegate to caller
            raise ProviderError(f"Transformers 推理失败: {exc}") from exc

        if isinstance(outputs, list) and outputs and isinstance(outputs[0], dict):
            text = outputs[0].get("generated_text") or outputs[0].get("summary_text")
        else:
            text = str(outputs)

        return ModelInvokeResponse(output_text=text or "", raw={"result": outputs})

    async def _ensure_pipeline(self, model: Model):
        pipeline = self._pipelines.get(model.id)
        if pipeline is not None:
            return pipeline

        lock = self._locks.setdefault(model.id, asyncio.Lock())
        async with lock:
            pipeline = self._pipelines.get(model.id)
            if pipeline is not None:
                return pipeline

            try:
                from transformers import pipeline as build_pipeline
            except ImportError as exc:
                raise ProviderError("缺少 transformers 依赖，无法加载本地模型") from exc

            model_path = self._resolve_model_path(model)
            task = model.config.get("task", "text-generation")
            pipeline_kwargs = {
                "model": str(model_path),
                "torch_dtype": model.config.get("torch_dtype"),
                "device_map": model.config.get("device_map", "auto"),
                "trust_remote_code": model.config.get("trust_remote_code", False),
            }
            # remove None values
            pipeline_kwargs = {k: v for k, v in pipeline_kwargs.items() if v is not None}

            pipeline = await asyncio.to_thread(build_pipeline, task, **pipeline_kwargs)
            self._pipelines[model.id] = pipeline
            return pipeline

    def _resolve_model_path(self, model: Model) -> Path | str:
        if model.local_path:
            return Path(model.local_path)
        if model.download_uri:
            return model.download_uri
        if model.remote_identifier:
            return model.remote_identifier
        return model.name

    def _messages_to_prompt(self, request: ModelInvokeRequest) -> str:
        messages = request.messages or []
        formatted = []
        for message in messages:
            prefix = message.role.capitalize()
            formatted.append(f"{prefix}: {message.content}")
        return "\n".join(formatted)


