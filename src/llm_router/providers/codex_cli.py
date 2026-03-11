from __future__ import annotations

import asyncio
import json
import logging
from typing import Any, AsyncIterator

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from ..services.cli_conversation_store import get_cli_conversation_store
from .base import BaseProviderClient, ProviderError
from .codex_app_server import CodexAppServerError, get_codex_app_server

logger = logging.getLogger(__name__)


class CodexCLIProviderClient(BaseProviderClient):
    """Codex CLI provider client：优先使用 app-server 常驻进程，回退到 codex exec。"""

    DEFAULT_EXECUTABLE = "codex"
    DEFAULT_TIMEOUT_SECONDS = 300.0

    def _use_app_server(self) -> bool:
        return bool(self.provider.settings.get("use_app_server", True))

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        prompt = self._build_prompt(request)
        if not prompt:
            raise ProviderError("Codex CLI 需要非空 prompt/messages")

        model_identifier = self._resolve_model_identifier(model, request)
        conversation_key = request.conversation_id

        # 优先尝试 app-server（有 conversation_key 时可延续会话）
        if self._use_app_server():
            app_server = get_codex_app_server()
            if app_server is not None:
                try:
                    return await self._invoke_via_app_server(
                        model_identifier, prompt, request, conversation_key
                    )
                except CodexAppServerError as exc:
                    logger.warning("Codex app-server 调用失败，回退 exec: %s", exc)

        # 回退到 codex exec
        return await self._invoke_via_exec(model_identifier, prompt, request)

    async def _invoke_via_app_server(
        self,
        model_identifier: str,
        prompt: str,
        request: ModelInvokeRequest,
        conversation_key: str | None,
    ) -> ModelInvokeResponse:
        """通过 app-server 调用，支持会话延续"""
        app_server = get_codex_app_server()
        if app_server is None:
            raise ProviderError("Codex app-server 不可用")

        store = get_cli_conversation_store()
        thread_id: str | None = None
        resume = False
        if conversation_key:
            info = store.get(ProviderType.CODEX_CLI, conversation_key)
            thread_id = info.cli_id if info else None
            resume = thread_id is not None

        try:
            thread_id, output_text, usage = await app_server.invoke(
                model=model_identifier,
                prompt=prompt,
                thread_id=thread_id,
                resume=resume,
            )
        except CodexAppServerError as exc:
            # 上下文超限等，清除映射后由调用方重试或开新会话
            if conversation_key and ("context" in str(exc).lower() or "limit" in str(exc).lower()):
                store.delete(ProviderType.CODEX_CLI, conversation_key)
            raise ProviderError(str(exc)) from exc

        if conversation_key:
            token_count = 0
            if isinstance(usage, dict):
                token_count = usage.get("input_tokens", 0) + usage.get("output_tokens", 0)
            store.set(ProviderType.CODEX_CLI, conversation_key, thread_id, token_count)

        raw: dict[str, Any] = {
            "provider": "codex_cli",
            "model": model_identifier,
            "thread_id": thread_id,
        }
        if usage:
            raw["usage"] = usage
        return ModelInvokeResponse(output_text=output_text, raw=raw)

    async def _invoke_via_exec(
        self,
        model_identifier: str,
        prompt: str,
        request: ModelInvokeRequest,
    ) -> ModelInvokeResponse:
        executable = str(
            self.provider.settings.get("executable", self.DEFAULT_EXECUTABLE)
        ).strip() or self.DEFAULT_EXECUTABLE

        command: list[str] = [executable, "exec", "--json", "-m", model_identifier]
        if bool(self.provider.settings.get("skip_git_repo_check", True)):
            command.append("--skip-git-repo-check")

        extra_args = self.provider.settings.get("args", [])
        if isinstance(extra_args, list):
            command.extend(str(arg) for arg in extra_args if str(arg).strip())
        command.append(prompt)

        timeout = float(
            self.provider.settings.get("timeout", self.DEFAULT_TIMEOUT_SECONDS)
        )
        if timeout <= 0:
            timeout = self.DEFAULT_TIMEOUT_SECONDS

        try:
            process = await asyncio.create_subprocess_exec(
                *command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
        except Exception as exc:
            raise ProviderError(f"启动 Codex CLI 失败: {exc}") from exc

        try:
            stdout_bytes, stderr_bytes = await asyncio.wait_for(
                process.communicate(), timeout=timeout
            )
        except asyncio.TimeoutError as exc:
            process.kill()
            await process.communicate()
            raise ProviderError(
                f"Codex CLI 调用超时（>{timeout:.0f}s）"
            ) from exc

        stdout_text = stdout_bytes.decode("utf-8", errors="replace")
        stderr_text = stderr_bytes.decode("utf-8", errors="replace").strip()
        parsed_events, output_text, usage = self._parse_exec_jsonl(stdout_text)

        if process.returncode != 0:
            message = stderr_text or "无 stderr 输出"
            raise ProviderError(
                f"Codex CLI 调用失败（exit={process.returncode}）: {message}"
            )

        if not output_text:
            message = stderr_text or "Codex CLI 未返回可解析文本输出"
            raise ProviderError(message)

        raw: dict[str, Any] = {
            "provider": "codex_cli",
            "model": model_identifier,
            "events_count": len(parsed_events),
        }
        if usage:
            raw["usage"] = usage
        if stderr_text:
            raw["stderr"] = stderr_text

        return ModelInvokeResponse(output_text=output_text, raw=raw)

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        # 当前 codex exec --json 不提供稳定 token 级增量事件，先按“伪流式”输出。
        response = await self.invoke(model, request)
        usage = response.raw.get("usage") if isinstance(response.raw, dict) else None
        yield ModelStreamChunk(text=response.output_text, raw=response.raw, usage=usage)
        yield ModelStreamChunk(is_final=True, usage=usage)

    def _resolve_model_identifier(
        self, model: Model, request: ModelInvokeRequest
    ) -> str:
        if request.remote_identifier_override:
            return request.remote_identifier_override
        return model.remote_identifier or model.config.get("model") or model.name

    def _build_prompt(self, request: ModelInvokeRequest) -> str:
        if request.prompt and not request.messages:
            return request.prompt

        parts: list[str] = []
        for msg in request.messages or []:
            text = self._extract_text(msg.content)
            if not text:
                continue
            parts.append(f"{msg.role.upper()}: {text}")

        if request.prompt:
            parts.append(f"USER: {request.prompt}")
        return "\n\n".join(parts).strip()

    def _extract_text(self, content: Any) -> str:
        if isinstance(content, str):
            return content.strip()
        if isinstance(content, list):
            pieces: list[str] = []
            for item in content:
                if not isinstance(item, dict):
                    continue
                text = item.get("text")
                if isinstance(text, str) and text.strip():
                    pieces.append(text.strip())
            return "\n".join(pieces).strip()
        return str(content).strip()

    def _parse_exec_jsonl(
        self, stdout_text: str
    ) -> tuple[list[dict[str, Any]], str, dict[str, Any]]:
        events: list[dict[str, Any]] = []
        texts: list[str] = []
        usage: dict[str, Any] = {}

        for line in stdout_text.splitlines():
            raw_line = line.strip()
            if not raw_line:
                continue
            try:
                payload = json.loads(raw_line)
            except json.JSONDecodeError:
                continue
            if not isinstance(payload, dict):
                continue

            events.append(payload)
            event_type = payload.get("type")
            if event_type == "item.completed":
                item = payload.get("item")
                if isinstance(item, dict):
                    text = item.get("text")
                    if isinstance(text, str) and text.strip():
                        texts.append(text.strip())
            elif event_type == "turn.completed":
                candidate_usage = payload.get("usage")
                if isinstance(candidate_usage, dict):
                    usage = candidate_usage

        return events, "\n\n".join(texts).strip(), usage

__all__ = ["CodexCLIProviderClient"]
