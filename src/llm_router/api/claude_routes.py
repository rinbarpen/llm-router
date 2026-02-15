"""Claude 原生 API 兼容端点：/v1/messages"""

from __future__ import annotations

import json
import logging
from typing import Any, AsyncIterator

from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import Response, StreamingResponse
from starlette.status import HTTP_400_BAD_REQUEST, HTTP_404_NOT_FOUND

from ..schemas import ChatMessage, ModelInvokeRequest
from ..services import RouterEngine, RoutingError
from .request_utils import read_json_body

logger = logging.getLogger(__name__)

CLAUDE_PROVIDER = "claude"


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


def _claude_messages_to_chat_messages(
    messages: list, system: str | None
) -> list[ChatMessage]:
    """将 Claude messages 和 system 转为 ChatMessage 列表"""
    result: list[ChatMessage] = []

    if system:
        result.append(ChatMessage(role="system", content=system))

    for msg in messages or []:
        role = msg.get("role", "user")
        content = msg.get("content")
        if content is None:
            continue
        if isinstance(content, str):
            text = content
        elif isinstance(content, list):
            texts = []
            for block in content:
                if isinstance(block, dict):
                    if block.get("type") == "text":
                        texts.append(block.get("text", ""))
                    elif block.get("type") == "image":
                        source = block.get("source") or {}
                        media_type = source.get("media_type") or source.get("mediaType", "image/png")
                        data = source.get("data", "")
                        texts.append(f"data:{media_type};base64,{data}")
            text = "\n".join(texts)
        else:
            text = str(content)
        if text:
            role_str = "user" if role == "user" else "assistant"
            result.append(ChatMessage(role=role_str, content=text))

    return result


def _build_claude_response(invoke_response: Any) -> dict:
    """将 ModelInvokeResponse 转为 Claude 响应格式"""
    raw = invoke_response.raw or {}
    if "content" in raw and "id" in raw:
        return raw

    return {
        "id": raw.get("id", "msg_unknown"),
        "type": "message",
        "role": "assistant",
        "content": [{"type": "text", "text": invoke_response.output_text}],
        "model": raw.get("model", ""),
        "stop_reason": "end_turn",
        "usage": raw.get("usage") or {
            "input_tokens": raw.get("prompt_tokens", 0),
            "output_tokens": raw.get("completion_tokens", 0),
        },
    }


async def claude_messages(request: Request) -> Response:
    """
    Claude 原生格式：POST /v1/messages
    请求体 model 如 claude-4.5-sonnet 映射为 provider=claude, model=claude-4.5-sonnet
    """
    body = await read_json_body(request)
    model_name = body.get("model")
    if not model_name:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="请求体缺少 model 字段",
        )

    messages = body.get("messages") or []
    system = body.get("system")
    max_tokens = body.get("max_tokens", 1024)
    stream = body.get("stream", False)

    if not messages:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="messages 不能为空",
        )

    chat_messages = _claude_messages_to_chat_messages(messages, system)
    if not chat_messages:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="messages 需至少包含一个用户消息",
        )

    parameters = {"max_tokens": max_tokens}
    if "temperature" in body:
        parameters["temperature"] = body["temperature"]
    if "top_p" in body:
        parameters["top_p"] = body["top_p"]
    if "top_k" in body:
        parameters["top_k"] = body["top_k"]
    if "stop_sequences" in body:
        parameters["stop"] = body["stop_sequences"]

    invoke_request = ModelInvokeRequest(
        messages=chat_messages,
        parameters=parameters,
        stream=stream,
    )

    session = request.state.session
    service = _get_service(request)
    model = await service.get_model_by_name(session, CLAUDE_PROVIDER, model_name)
    if model is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail=f"模型 {CLAUDE_PROVIDER}/{model_name} 不存在",
        )

    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(CLAUDE_PROVIDER, model_name):
        raise HTTPException(
            status_code=403,
            detail=f"API Key 不允许调用模型 {CLAUDE_PROVIDER}/{model_name}",
        )

    engine = _get_router_engine(request)

    if stream:
        try:
            stream_iter = await engine.stream_by_identifier(
                session, CLAUDE_PROVIDER, model_name, invoke_request
            )
        except RoutingError as exc:
            raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))

        async def event_stream() -> AsyncIterator[bytes]:
            async for chunk in stream_iter:
                if chunk.is_final:
                    yield b"data: {\"type\":\"message_stop\"}\n\n"
                    return
                if chunk.text:
                    event = {
                        "type": "content_block_delta",
                        "index": 0,
                        "delta": {"type": "text_delta", "text": chunk.text},
                    }
                    yield f"data: {json.dumps(event, ensure_ascii=False)}\n\n".encode("utf-8")
                if chunk.usage:
                    event = {
                        "type": "message_delta",
                        "delta": {},
                        "usage": {
                            "output_tokens": chunk.usage.get("completion_tokens", 0),
                            "input_tokens": chunk.usage.get("prompt_tokens", 0),
                        },
                    }
                    yield f"data: {json.dumps(event, ensure_ascii=False)}\n\n".encode("utf-8")
            yield b"data: {\"type\":\"message_stop\"}\n\n"

        return StreamingResponse(
            event_stream(),
            media_type="text/event-stream",
            headers={
                "Cache-Control": "no-cache",
                "Connection": "keep-alive",
                "anthropic-version": "2023-06-01",
            },
        )

    try:
        response = await engine.invoke_by_identifier(
            session, CLAUDE_PROVIDER, model_name, invoke_request
        )
    except RoutingError as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))

    claude_response = _build_claude_response(response)
    claude_response["model"] = model_name
    return Response(
        content=json.dumps(claude_response, ensure_ascii=False),
        media_type="application/json",
    )
