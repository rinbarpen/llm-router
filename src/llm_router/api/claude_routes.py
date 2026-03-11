"""Claude 原生 API 兼容端点：/v1/messages"""

from __future__ import annotations

import json
import logging
import uuid
from typing import Any, AsyncIterator

from sqlalchemy import select
from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import Response, StreamingResponse
from starlette.status import HTTP_400_BAD_REQUEST, HTTP_404_NOT_FOUND

from ..db.models import Provider, ProviderType
from ..schemas import ChatMessage, ModelInvokeRequest
from ..services import RouterEngine, RoutingError
from .auth import extract_session_token
from .request_utils import normalize_claude_provider_name, read_json_body

logger = logging.getLogger(__name__)

CLAUDE_PROVIDER_CANDIDATES = ("claude_code_cli", "claude")


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


async def _resolve_claude_model(request: Request, model_reference: str) -> tuple[str, Any]:
    service = _get_service(request)
    session = request.state.session

    if "/" in model_reference:
        provider_name, model_name = model_reference.split("/", 1)
        provider_name = normalize_claude_provider_name(provider_name) or provider_name
        model = await service.get_model_by_name(session, provider_name, model_name)
        if model is None:
            raise HTTPException(
                status_code=HTTP_404_NOT_FOUND,
                detail=f"模型 {provider_name}/{model_name} 不存在",
            )
        return provider_name, model

    for provider_name in CLAUDE_PROVIDER_CANDIDATES:
        model = await service.get_model_by_name(session, provider_name, model_reference)
        if model is not None:
            return provider_name, model

    raise HTTPException(
        status_code=HTTP_404_NOT_FOUND,
        detail=f"模型 {model_reference} 不存在（未在 claude_code_cli/claude provider 下找到）",
    )


def _estimate_input_tokens(messages: list, system: str | None) -> int:
    text_parts: list[str] = []
    if isinstance(system, str):
        text_parts.append(system)
    for msg in messages or []:
        if not isinstance(msg, dict):
            continue
        content = msg.get("content")
        if isinstance(content, str):
            text_parts.append(content)
        elif isinstance(content, list):
            for block in content:
                if isinstance(block, dict):
                    block_text = block.get("text")
                    if isinstance(block_text, str):
                        text_parts.append(block_text)
    text = "\n".join(text_parts)
    # 保守估算：中英混合场景取约 3.5 chars/token。
    return max(1, int(len(text) / 3.5)) if text else 1


async def _resolve_claude_code_provider(request: Request) -> Provider:
    """解析用于 count_tokens/batches 的 provider，优先 claude（API Key）"""
    session = request.state.session
    stmt = select(Provider).where(Provider.name == "claude")
    provider = await session.scalar(stmt)
    if provider is not None:
        return provider

    stmt = select(Provider).where(Provider.type == ProviderType.CLAUDE).order_by(Provider.id)
    provider = await session.scalar(stmt)
    if provider is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail="未找到 claude provider（count_tokens/batches 需要 API Key）",
        )
    return provider


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

    conversation_key = (
        body.get("conversation_id")
        or extract_session_token(request)
        or str(uuid.uuid4())
    )
    invoke_request = ModelInvokeRequest(
        messages=chat_messages,
        parameters=parameters,
        stream=stream,
        conversation_id=conversation_key,
    )

    session = request.state.session
    provider_name, model = await _resolve_claude_model(request, model_name)

    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(provider_name, model.name):
        raise HTTPException(
            status_code=403,
            detail=f"API Key 不允许调用模型 {provider_name}/{model.name}",
        )

    engine = _get_router_engine(request)

    if stream:
        try:
            stream_iter = await engine.stream_by_identifier(
                session, provider_name, model.name, invoke_request
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
            session, provider_name, model.name, invoke_request
        )
    except RoutingError as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))

    claude_response = _build_claude_response(response)
    claude_response["model"] = model.name
    return Response(
        content=json.dumps(claude_response, ensure_ascii=False),
        media_type="application/json",
    )


async def claude_count_tokens(request: Request) -> Response:
    body = await read_json_body(request)
    model_name = body.get("model")
    messages = body.get("messages") or []
    system = body.get("system")

    if not model_name:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="请求体缺少 model 字段")
    if not isinstance(messages, list):
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="messages 必须是数组")

    provider_name, model = await _resolve_claude_model(request, model_name)
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(provider_name, model.name):
        raise HTTPException(
            status_code=403,
            detail=f"API Key 不允许调用模型 {provider_name}/{model.name}",
        )

    session = request.state.session
    engine = _get_router_engine(request)
    provider = await session.merge(model.provider)
    client = engine.provider_registry.get(provider)
    client.update_provider(provider)

    if hasattr(client, "count_tokens"):
        try:
            data = await client.count_tokens(model, {"messages": messages, "system": system})  # type: ignore[attr-defined]
            input_tokens = int(data.get("input_tokens", 0))
            return Response(
                content=json.dumps({"input_tokens": input_tokens}, ensure_ascii=False),
                media_type="application/json",
            )
        except Exception as exc:
            raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))

    input_tokens = _estimate_input_tokens(messages, system)
    return Response(
        content=json.dumps({"input_tokens": input_tokens}, ensure_ascii=False),
        media_type="application/json",
    )


async def claude_create_message_batch(request: Request) -> Response:
    body = await read_json_body(request)
    provider = await _resolve_claude_code_provider(request)
    session = request.state.session
    provider = await session.merge(provider)
    engine = _get_router_engine(request)
    client = engine.provider_registry.get(provider)
    client.update_provider(provider)

    if not hasattr(client, "create_message_batch"):
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="当前 provider 不支持 messages batches")

    try:
        data = await client.create_message_batch(body)  # type: ignore[attr-defined]
    except Exception as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))
    return Response(
        content=json.dumps(data, ensure_ascii=False),
        media_type="application/json",
    )


async def claude_get_message_batch(request: Request) -> Response:
    batch_id = request.path_params.get("batch_id")
    if not batch_id:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="缺少 batch_id")
    provider = await _resolve_claude_code_provider(request)
    session = request.state.session
    provider = await session.merge(provider)
    engine = _get_router_engine(request)
    client = engine.provider_registry.get(provider)
    client.update_provider(provider)

    if not hasattr(client, "get_message_batch"):
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="当前 provider 不支持 messages batches")

    try:
        data = await client.get_message_batch(batch_id)  # type: ignore[attr-defined]
    except Exception as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))
    return Response(
        content=json.dumps(data, ensure_ascii=False),
        media_type="application/json",
    )


async def claude_cancel_message_batch(request: Request) -> Response:
    batch_id = request.path_params.get("batch_id")
    if not batch_id:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="缺少 batch_id")
    provider = await _resolve_claude_code_provider(request)
    session = request.state.session
    provider = await session.merge(provider)
    engine = _get_router_engine(request)
    client = engine.provider_registry.get(provider)
    client.update_provider(provider)

    if not hasattr(client, "cancel_message_batch"):
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail="当前 provider 不支持 messages batches")

    try:
        data = await client.cancel_message_batch(batch_id)  # type: ignore[attr-defined]
    except Exception as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))
    return Response(
        content=json.dumps(data, ensure_ascii=False),
        media_type="application/json",
    )
