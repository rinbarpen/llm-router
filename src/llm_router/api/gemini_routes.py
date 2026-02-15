"""Gemini 原生 API 兼容端点：/v1beta/models/{model}:generateContent 及 streamGenerateContent"""

from __future__ import annotations

import json
import logging
from typing import Any, AsyncIterator

from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import Response, StreamingResponse
from starlette.status import HTTP_400_BAD_REQUEST, HTTP_404_NOT_FOUND

from ..schemas import ChatMessage, ModelInvokeRequest, ModelInvokeResponse
from ..services import RouterEngine, RoutingError
from .request_utils import read_json_body

logger = logging.getLogger(__name__)

GEMINI_PROVIDER = "gemini"


def _get_service(request: Request):
    service = getattr(request.app.state, "model_service", None)
    if service is None:
        raise RuntimeError("ModelService 尚未初始化")
    return service


def _get_router_engine(request: Request) -> RouterEngine:
    engine = getattr(request.app.state, "router_engine", None)
    if engine is None:
        raise RuntimeError("RouterEngine 尚未初始化")
    return engine


def _gemini_contents_to_messages(contents: list, system_instruction: dict | None) -> list[ChatMessage]:
    """将 Gemini contents 和 systemInstruction 转为 ChatMessage 列表"""
    messages: list[ChatMessage] = []

    if system_instruction:
        parts = system_instruction.get("parts") or []
        system_texts = [
            p.get("text", "")
            for p in parts
            if isinstance(p, dict) and "text" in p
        ]
        if system_texts:
            messages.append(ChatMessage(role="system", content="\n".join(system_texts)))

    for item in contents or []:
        role = item.get("role", "user")
        parts = item.get("parts") or []
        texts = []
        for p in parts:
            if isinstance(p, dict) and "text" in p:
                texts.append(p["text"])
            elif isinstance(p, dict) and ("inline_data" in p or "inlineData" in p):
                idata = p.get("inline_data") or p.get("inlineData") or {}
                mime = idata.get("mime_type") or idata.get("mimeType", "image/png")
                data = idata.get("data", "")
                texts.append(f"data:{mime};base64,{data}")
        if texts:
            content = "\n".join(texts)
            if role == "model":
                messages.append(ChatMessage(role="assistant", content=content))
            else:
                messages.append(ChatMessage(role="user", content=content))

    return messages


def _gemini_generation_config_to_params(gen_config: dict | None) -> dict[str, Any]:
    """将 Gemini generationConfig 转为 parameters"""
    if not gen_config:
        return {}
    params = {}
    if "maxOutputTokens" in gen_config:
        params["max_tokens"] = gen_config["maxOutputTokens"]
    if "temperature" in gen_config:
        params["temperature"] = gen_config["temperature"]
    if "topP" in gen_config:
        params["top_p"] = gen_config["topP"]
    if "topK" in gen_config:
        params["top_k"] = gen_config["topK"]
    if "stopSequences" in gen_config:
        params["stop"] = gen_config["stopSequences"]
    return params


def _build_gemini_response(invoke_response: ModelInvokeResponse, model_name: str) -> dict:
    """将 ModelInvokeResponse 转为 Gemini 响应格式"""
    raw = invoke_response.raw or {}
    candidates = raw.get("candidates")
    if candidates is not None:
        return raw

    parts = [{"text": invoke_response.output_text}]
    return {
        "candidates": [
            {
                "content": {"role": "model", "parts": parts},
                "finishReason": "STOP",
            }
        ],
        "usageMetadata": raw.get("usageMetadata") or {
            "promptTokenCount": raw.get("usage", {}).get("prompt_tokens", 0),
            "candidatesTokenCount": raw.get("usage", {}).get("completion_tokens", 0),
            "totalTokenCount": raw.get("usage", {}).get("total_tokens", 0),
        },
    }


async def gemini_generate_content(request: Request) -> Response:
    """
    Gemini 原生格式：POST /v1beta/models/{model}:generateContent
    模型路径中的 {model} 如 gemini-2.5-pro 映射为 provider=gemini, model=gemini-2.5-pro
    """
    model_name = request.path_params.get("model")
    if not model_name:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="缺少 model 参数")

    body = await read_json_body(request, allow_empty=True)
    if not body:
        body = {}

    contents = body.get("contents") or []
    system_instruction = body.get("systemInstruction")
    generation_config = body.get("generationConfig") or {}

    messages = _gemini_contents_to_messages(contents, system_instruction)
    if not messages:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="contents 不能为空，需至少包含一个用户消息",
        )

    parameters = _gemini_generation_config_to_params(generation_config)

    invoke_request = ModelInvokeRequest(
        messages=messages,
        parameters=parameters,
        stream=False,
    )

    session = request.state.session
    service = _get_service(request)
    model = await service.get_model_by_name(session, GEMINI_PROVIDER, model_name)
    if model is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail=f"模型 {GEMINI_PROVIDER}/{model_name} 不存在",
        )

    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(GEMINI_PROVIDER, model_name):
        raise HTTPException(
            status_code=403,
            detail=f"API Key 不允许调用模型 {GEMINI_PROVIDER}/{model_name}",
        )

    engine = _get_router_engine(request)
    try:
        response = await engine.invoke_by_identifier(
            session, GEMINI_PROVIDER, model_name, invoke_request
        )
    except RoutingError as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))

    gemini_response = _build_gemini_response(response, model_name)
    return Response(
        content=json.dumps(gemini_response, ensure_ascii=False),
        media_type="application/json",
    )


async def gemini_stream_generate_content(request: Request) -> Response:
    """
    Gemini 原生格式流式：POST /v1beta/models/{model}:streamGenerateContent
    返回 SSE 流，与 Gemini 官方格式一致
    """
    model_name = request.path_params.get("model")
    if not model_name:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="缺少 model 参数")

    body = await read_json_body(request, allow_empty=True)
    if not body:
        body = {}

    contents = body.get("contents") or []
    system_instruction = body.get("systemInstruction")
    generation_config = body.get("generationConfig") or {}

    messages = _gemini_contents_to_messages(contents, system_instruction)
    if not messages:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="contents 不能为空，需至少包含一个用户消息",
        )

    parameters = _gemini_generation_config_to_params(generation_config)

    invoke_request = ModelInvokeRequest(
        messages=messages,
        parameters=parameters,
        stream=True,
    )

    session = request.state.session
    service = _get_service(request)
    model = await service.get_model_by_name(session, GEMINI_PROVIDER, model_name)
    if model is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail=f"模型 {GEMINI_PROVIDER}/{model_name} 不存在",
        )

    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(GEMINI_PROVIDER, model_name):
        raise HTTPException(
            status_code=403,
            detail=f"API Key 不允许调用模型 {GEMINI_PROVIDER}/{model_name}",
        )

    engine = _get_router_engine(request)
    try:
        stream = await engine.stream_by_identifier(
            session, GEMINI_PROVIDER, model_name, invoke_request
        )
    except RoutingError as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))

    async def event_stream() -> AsyncIterator[bytes]:
        async for chunk in stream:
            if chunk.is_final:
                break
            if chunk.raw:
                data_str = json.dumps(chunk.raw, ensure_ascii=False)
                yield f"data: {data_str}\n\n".encode("utf-8")
            elif chunk.text:
                gemini_chunk = {
                    "candidates": [
                        {
                            "content": {"role": "model", "parts": [{"text": chunk.text}]},
                            "finishReason": chunk.finish_reason or "STOP",
                        }
                    ]
                }
                if chunk.usage:
                    gemini_chunk["usageMetadata"] = {
                        "promptTokenCount": chunk.usage.get("prompt_tokens", 0),
                        "candidatesTokenCount": chunk.usage.get("completion_tokens", 0),
                        "totalTokenCount": chunk.usage.get("total_tokens", 0),
                    }
                data_str = json.dumps(gemini_chunk, ensure_ascii=False)
                yield f"data: {data_str}\n\n".encode("utf-8")
        yield b"data: [DONE]\n\n"

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
        },
    )
