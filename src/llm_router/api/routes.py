from __future__ import annotations

import asyncio
import csv
import json
import logging
import shutil
import tempfile
import time
import uuid
from datetime import datetime
from pathlib import Path
from typing import Any, AsyncIterator, List, Optional, cast

from pydantic import ValidationError
from sqlalchemy import select
from starlette.exceptions import HTTPException
from starlette.requests import Request

from ..db.models import Provider
from starlette.responses import FileResponse, JSONResponse, Response, StreamingResponse
from starlette.status import (
    HTTP_200_OK,
    HTTP_201_CREATED,
    HTTP_204_NO_CONTENT,
    HTTP_400_BAD_REQUEST,
    HTTP_401_UNAUTHORIZED,
    HTTP_403_FORBIDDEN,
    HTTP_404_NOT_FOUND,
    HTTP_500_INTERNAL_SERVER_ERROR,
)

from ..config import RouterSettings, load_settings, _sqlite_path_from_url
from ..db.models import Provider, ProviderType
from ..model_config import load_model_config
from ..schemas import (
    APIKeyCreate,
    APIKeyRead,
    APIKeyUpdate,
    BindModelRequest,
    ChatMessage,
    InvocationQuery,
    InvocationStatus,
    ModelCreate,
    ModelInvokeRequest,
    ModelQuery,
    ModelUpdate,
    ModelStreamChunk,
    ModelPricingInfo,
    OpenAICompatibleChatCompletionRequest,
    OpenAICompatibleChatCompletionResponse,
    OpenAICompatibleChoice,
    OpenAICompatibleMessage,
    OpenAICompatibleUsage,
    OpenAIModelInfo,
    OpenAIModelList,
    PricingSuggestion,
    PricingSyncRequest,
    PricingSyncResponse,
    RouteDecisionRequest,
    RouteDecisionResponse,
    ProviderCreate,
    ProviderRead,
    ProviderUpdate,
)
from ..services import (
    APIKeyService,
    ModelService,
    MonitorService,
    PricingService,
    RouterEngine,
    RoutingError,
)
from ..db.login_models import LoginRecord
from ..services.login_record_service import get_login_record_service
from .auth import extract_api_key, extract_session_token, is_local_request
from .request_utils import normalize_multimodal_content, parse_model_body, read_json_body
from .session_store import get_session_store

logger = logging.getLogger(__name__)


def _get_service(request: Request) -> ModelService:
    service = getattr(request.app.state, "model_service", None)
    if service is None:
        raise RuntimeError("ModelService 尚未初始化")
    return service


def _get_router_engine(request: Request) -> RouterEngine:
    engine = getattr(request.app.state, "router_engine", None)
    if engine is None:
        raise RuntimeError("RouterEngine 尚未初始化")
    return engine


def _get_api_key_service(request: Request) -> APIKeyService:
    service = getattr(request.app.state, "api_key_service", None)
    if service is None:
        raise RuntimeError("APIKeyService 尚未初始化")
    return service


def _get_monitor_service(request: Request) -> MonitorService:
    """获取监控服务（使用独立的监控数据库）"""
    service = getattr(request.app.state, "monitor_service", None)
    if service is None:
        raise RuntimeError("MonitorService 尚未初始化")
    return service


def _get_pricing_service(request: Request) -> PricingService:
    """获取定价服务"""
    service = getattr(request.app.state, "pricing_service", None)
    if service is None:
        # 如果服务未初始化，创建一个新的实例
        service = PricingService()
        request.app.state.pricing_service = service
    return service


async def _jsonl_stream(
    stream: AsyncIterator[ModelStreamChunk],
) -> AsyncIterator[bytes]:
    async for chunk in stream:
        data = {
            "text": chunk.text,
            "finish_reason": chunk.finish_reason,
            "is_final": chunk.is_final,
        }
        if chunk.raw is not None:
            data["raw"] = chunk.raw
        if chunk.usage:
            data["usage"] = chunk.usage
        yield (json.dumps(data, ensure_ascii=False) + "\n").encode("utf-8")


def _build_openai_stream_choices(chunk: ModelStreamChunk) -> List[dict]:
    if isinstance(chunk.raw, dict):
        choices = chunk.raw.get("choices")
        if isinstance(choices, list) and choices:
            return choices

    delta: dict[str, Any] = dict(chunk.delta)
    if chunk.text is not None:
        delta.setdefault("content", chunk.text)

    return [
        {
            "index": 0,
            "delta": delta,
            "finish_reason": chunk.finish_reason,
        }
    ]


def _safe_float(value: Any, default: float) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def _safe_int(value: Any) -> Optional[int]:
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _normalize_role_tags(role: str, task: str) -> List[str]:
    role_key = role.strip().lower()
    task_key = task.strip().lower()

    role_tags = {
        "supervisor": ["routing", "chat"],
        "planner": ["planning", "reasoning"],
        "writer": ["writing", "chat"],
        "tester": ["qa", "reasoning"],
        "docupdater": ["docs", "writing"],
    }

    tags: List[str] = []
    tags.extend(role_tags.get(role_key, []))

    if "routing" in task_key:
        tags.append("routing")
    if "worker" in task_key:
        tags.append("chat")
    if "doc" in task_key:
        tags.append("docs")

    # 去重且保持顺序
    return list(dict.fromkeys(t for t in tags if t))


def _parse_model_hint(model_hint: str) -> tuple[Optional[str], str]:
    if "/" in model_hint:
        provider, model = model_hint.split("/", 1)
        return provider.strip() or None, model.strip()
    return None, model_hint.strip()


def _build_analysis_messages(payload: RouteDecisionRequest) -> List[ChatMessage]:
    if payload.messages:
        return payload.messages
    if payload.prompt:
        return [ChatMessage(role="user", content=payload.prompt)]

    role = (payload.role or "").strip()
    task = (payload.task or "").strip()
    summary = f"role={role or 'unknown'}; task={task or 'unknown'}"
    return [ChatMessage(role="user", content=summary)]


def _extract_profile_from_analysis(output_text: str) -> str:
    text = (output_text or "").strip().lower()
    if "strong" in text or "stronge" in text:
        return "strong"
    if "weak" in text:
        return "weak"
    return "weak"


def _split_provider_model(identifier: str) -> tuple[str, str]:
    provider_name, model_name = _parse_model_hint(identifier)
    if not provider_name or not model_name:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"模型标识 '{identifier}' 格式无效，需为 provider/model",
        )
    return provider_name, model_name


async def _resolve_profile_model(
    request: Request,
    profile: str,
) -> tuple[str, str]:
    session_data = getattr(request.state, "session_data", None)
    if session_data:
        if profile == "strong" and session_data.strong_provider_name and session_data.strong_model_name:
            return session_data.strong_provider_name, session_data.strong_model_name
        if profile == "weak" and session_data.weak_provider_name and session_data.weak_model_name:
            return session_data.weak_provider_name, session_data.weak_model_name

    settings = getattr(request.app.state, "settings", None) or load_settings()
    model_ref = settings.routing_default_strong_model if profile == "strong" else settings.routing_default_weak_model
    if not model_ref:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"未配置 {profile} 模型，请设置 router.toml routing.default_{profile}_model 或先绑定 session 模型",
        )
    return _split_provider_model(model_ref)


async def _analyze_route_profile(request: Request, payload: RouteDecisionRequest) -> str:
    settings = getattr(request.app.state, "settings", None) or load_settings()
    analyzer_ref = settings.routing_analyzer_model
    if not analyzer_ref:
        return "weak"

    provider_name, model_name = _split_provider_model(analyzer_ref)
    invoke_request = ModelInvokeRequest(
        messages=[
            ChatMessage(
                role="system",
                content="你是路由器，只输出一个词：strong 或 weak。",
            ),
            * _build_analysis_messages(payload),
        ],
        parameters={"temperature": 0},
    )

    engine = _get_router_engine(request)
    session = request.state.session
    try:
        response = await asyncio.wait_for(
            engine.invoke_by_identifier(session, provider_name, model_name, invoke_request),
            timeout=max(settings.routing_analyzer_timeout_ms, 100) / 1000.0,
        )
    except Exception:
        fallback = (settings.routing_auto_fallback_mode or "weak").strip().lower()
        return "strong" if fallback == "strong" else "weak"
    return _extract_profile_from_analysis(response.output_text)


async def _resolve_routing_mode_target(
    request: Request,
    payload: RouteDecisionRequest,
    routing_mode: str,
) -> tuple[str, str]:
    mode = routing_mode.strip().lower()
    if mode == "auto":
        mode = await _analyze_route_profile(request, payload)
    if mode not in {"strong", "weak"}:
        mode = "weak"
    return await _resolve_profile_model(request, mode)



async def health(_: Request) -> Response:
    return JSONResponse({"status": "ok"}, status_code=HTTP_200_OK)


async def create_provider(request: Request) -> Response:
    payload = await parse_model_body(request, ProviderCreate)
    session = request.state.session
    service = _get_service(request)

    provider = await service.upsert_provider(session, payload)
    data = ProviderRead.model_validate(provider)
    return JSONResponse(data.model_dump(), status_code=HTTP_201_CREATED)


async def list_providers(request: Request) -> Response:
    session = request.state.session
    result = await session.scalars(select(Provider).order_by(Provider.id))
    providers = result.all()
    data = [ProviderRead.model_validate(provider).model_dump() for provider in providers]
    return JSONResponse(data)


async def update_provider(request: Request) -> Response:
    provider_name = request.path_params["provider_name"]
    payload = await parse_model_body(request, ProviderUpdate)
    session = request.state.session
    service = _get_service(request)

    stmt = select(Provider).where(Provider.name == provider_name)
    provider = await session.scalar(stmt)
    if provider is None:
        raise HTTPException(status_code=404, detail="Provider不存在")

    try:
        updated = await service.update_provider(session, provider, payload)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    data = ProviderRead.model_validate(updated)
    return JSONResponse(data.model_dump())


async def create_model(request: Request) -> Response:
    payload = await parse_model_body(request, ModelCreate)
    session = request.state.session
    service = _get_service(request)

    try:
        model = await service.register_model(session, payload)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    readable = service.to_model_read(model)
    return JSONResponse(readable.model_dump(), status_code=HTTP_201_CREATED)


def _parse_query(request: Request) -> ModelQuery:
    params = request.query_params
    tags = params.getlist("tag") or []
    raw_tags = params.get("tags")
    if raw_tags:
        tags.extend(part.strip() for part in raw_tags.split(",") if part.strip())

    provider_types: List[ProviderType] = []
    type_values = params.getlist("provider_type") or []
    raw_types = params.get("provider_types")
    if raw_types:
        type_values.extend(part.strip() for part in raw_types.split(",") if part.strip())

    for value in type_values:
        try:
            provider_types.append(ProviderType(value))
        except ValueError:
            raise HTTPException(status_code=400, detail=f"未知的Provider类型: {value}")

    include_inactive = params.get("include_inactive", "false").lower() in {
        "1",
        "true",
        "yes",
    }

    return ModelQuery(
        tags=list(dict.fromkeys(tags)),
        provider_types=provider_types,
        include_inactive=include_inactive,
    )


async def get_models(request: Request) -> Response:
    session = request.state.session
    service = _get_service(request)

    query = _parse_query(request)
    models = await service.list_models(session, query)
    return JSONResponse([model.model_dump() for model in models])


async def get_provider_models(request: Request) -> Response:
    provider_name = request.path_params["provider_name"]
    session = request.state.session
    service = _get_service(request)

    # 复用 _parse_query 但强制限制 provider_name
    query = _parse_query(request)
    # 我们需要修改 list_models 或者在这里手动过滤
    # 鉴于 ModelService.list_models 并不直接支持 provider_name 过滤字符串，
    # 我们先获取该 provider 对应的所有模型
    models = await service.list_models(session, query)
    filtered = [m for m in models if m.provider_name == provider_name]
    
    return JSONResponse([m.model_dump() for m in filtered])


async def invoke_model(request: Request) -> Response:
    provider_name = request.path_params["provider_name"]
    model_name = request.path_params["model_name"]

    body = await read_json_body(
        request,
        error_detail="请求体必须是有效的 JSON 格式。请提供 prompt 或 messages 字段。",
    )
    try:
        payload = ModelInvokeRequest.model_validate(body)
    except ValidationError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    # 检查 API Key 限制
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config:
        # 检查模型限制
        if not api_key_config.is_model_allowed(provider_name, model_name):
            raise HTTPException(
                status_code=403,
                detail=f"API Key 不允许调用模型 {provider_name}/{model_name}",
            )
        # 应用参数限制
        if payload.parameters and api_key_config.parameter_limits:
            payload.parameters = api_key_config.validate_parameters(payload.parameters)

    engine = _get_router_engine(request)
    session = request.state.session

    if not payload.batch and payload.stream:
        try:
            stream = await engine.stream_by_identifier(
                session, provider_name, model_name, payload
            )
        except RoutingError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
        return StreamingResponse(_jsonl_stream(stream), media_type="application/jsonl")

    try:
        response = await engine.invoke_by_identifier(
            session, provider_name, model_name, payload
        )
    except RoutingError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    return JSONResponse(response.model_dump())


async def route_decision(request: Request) -> Response:
    """轻量路由决策端点：返回模型配置，不执行模型调用。"""
    body = await read_json_body(
        request,
        error_detail="请求体必须是有效 JSON。请提供 role/task，可选 model_hint。",
    )

    try:
        payload = RouteDecisionRequest.model_validate(body)
    except ValidationError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    session = request.state.session
    service = _get_service(request)

    model_obj = None
    model_read = None

    if payload.model_hint:
        provider_name, model_name = _parse_model_hint(payload.model_hint)
        if model_name:
            if provider_name:
                model_obj = await service.get_model_by_name(session, provider_name, model_name)
                if model_obj and model_obj.is_active and model_obj.provider and model_obj.provider.is_active:
                    model_read = service.to_model_read(model_obj)
            else:
                candidates = await service.list_models(
                    session,
                    ModelQuery(name=model_name, include_inactive=False),
                )
                if candidates:
                    model_read = candidates[0]
                    model_obj = await service.get_model_by_name(
                        session, model_read.provider_name, model_read.name
                    )

    if model_read is None and payload.routing_mode:
        provider_name, model_name = await _resolve_routing_mode_target(
            request,
            payload,
            payload.routing_mode,
        )
        model_obj = await service.get_model_by_name(session, provider_name, model_name)
        if model_obj and model_obj.is_active and model_obj.provider and model_obj.provider.is_active:
            model_read = service.to_model_read(model_obj)
        else:
            raise HTTPException(
                status_code=HTTP_400_BAD_REQUEST,
                detail=f"routing_mode 选定模型不可用: {provider_name}/{model_name}",
            )

    if model_read is None:
        tags = _normalize_role_tags(payload.role or "", payload.task or "")
        candidates = await service.list_models(
            session,
            ModelQuery(tags=tags, include_inactive=False),
        )
        if not candidates:
            candidates = await service.list_models(session, ModelQuery(include_inactive=False))
        if not candidates:
            raise HTTPException(status_code=400, detail="无可用模型可用于路由决策")

        model_read = candidates[0]
        model_obj = await service.get_model_by_name(
            session, model_read.provider_name, model_read.name
        )

    if model_obj is None or model_obj.provider is None:
        raise HTTPException(status_code=400, detail="路由模型加载失败")

    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(model_read.provider_name, model_read.name):
        raise HTTPException(
            status_code=HTTP_403_FORBIDDEN,
            detail=f"API Key 不允许调用模型 {model_read.provider_name}/{model_read.name}",
        )

    default_params = model_obj.default_params or {}
    selected_temperature = payload.temperature
    if selected_temperature is None:
        selected_temperature = _safe_float(default_params.get("temperature"), 0.0)

    selected_max_tokens = payload.max_tokens
    if selected_max_tokens is None:
        selected_max_tokens = _safe_int(default_params.get("max_tokens"))

    response = RouteDecisionResponse(
        model=f"{model_read.provider_name}/{model_read.name}",
        base_url=model_obj.provider.base_url,
        api_key=model_obj.provider.api_key,
        temperature=float(selected_temperature),
        max_tokens=selected_max_tokens,
        provider=model_read.provider_name,
    )

    return JSONResponse(response.model_dump(exclude_none=True))


async def route_model(request: Request) -> Response:
    body = await read_json_body(
        request,
        error_detail="请求体必须是有效的 JSON 格式。请提供 query 和 request 字段。",
    )

    query_payload = body.get("query", {})
    request_payload = body.get("request", {})

    try:
        query = ModelQuery.model_validate(query_payload)
        payload = ModelInvokeRequest.model_validate(request_payload)
    except ValidationError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    # 检查 API Key 限制
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config:
        # 应用参数限制
        if payload.parameters and api_key_config.parameter_limits:
            payload.parameters = api_key_config.validate_parameters(payload.parameters)
        # 注意：模型限制在 RouterEngine 中通过过滤候选模型来处理

    engine = _get_router_engine(request)
    session = request.state.session

    if not payload.batch and payload.stream:
        try:
            stream = await engine.stream_by_tags(
                session, query, payload, api_key_config=api_key_config
            )
        except RoutingError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
        return StreamingResponse(_jsonl_stream(stream), media_type="application/jsonl")

    try:
        # 传递 api_key_config 给 engine，以便在路由时过滤模型
        response = await engine.route_by_tags(
            session, query, payload, api_key_config=api_key_config
        )
    except RoutingError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    return JSONResponse(response.model_dump())


async def update_model(request: Request) -> Response:
    provider_name = request.path_params["provider_name"]
    model_name = request.path_params["model_name"]

    payload = await parse_model_body(request, ModelUpdate)

    session = request.state.session
    service = _get_service(request)

    model = await service.get_model_by_name(session, provider_name, model_name)
    if model is None:
        raise HTTPException(status_code=404, detail="模型不存在")

    try:
        updated = await service.update_model(session, model, payload)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    readable = service.to_model_read(updated)
    return JSONResponse(readable.model_dump())


def _parse_invocation_query(request: Request) -> InvocationQuery:
    """解析调用查询参数"""
    params = request.query_params
    
    model_id = params.get("model_id")
    provider_id = params.get("provider_id")
    model_name = params.get("model_name")
    provider_name = params.get("provider_name")
    status_str = params.get("status")
    start_time_str = params.get("start_time")
    end_time_str = params.get("end_time")
    limit = params.get("limit", "100")
    offset = params.get("offset", "0")
    order_by = params.get("order_by", "started_at")
    order_desc = params.get("order_desc", "true")
    
    status = None
    if status_str:
        try:
            status = InvocationStatus(status_str)
        except ValueError:
            raise HTTPException(status_code=400, detail=f"无效的状态值: {status_str}")
    
    start_time = None
    if start_time_str:
        try:
            start_time = datetime.fromisoformat(start_time_str.replace("Z", "+00:00"))
        except ValueError:
            raise HTTPException(status_code=400, detail=f"无效的开始时间格式: {start_time_str}")
    
    end_time = None
    if end_time_str:
        try:
            end_time = datetime.fromisoformat(end_time_str.replace("Z", "+00:00"))
        except ValueError:
            raise HTTPException(status_code=400, detail=f"无效的结束时间格式: {end_time_str}")
    
    return InvocationQuery(
        model_id=int(model_id) if model_id else None,
        provider_id=int(provider_id) if provider_id else None,
        model_name=model_name,
        provider_name=provider_name,
        status=status,
        start_time=start_time,
        end_time=end_time,
        limit=int(limit),
        offset=int(offset),
        order_by=order_by,  # type: ignore
        order_desc=order_desc.lower() in ("true", "1", "yes"),
    )


async def download_database(request: Request) -> Response:
    """下载监控数据库文件（只读副本，用于前端直接读取）"""
    settings = load_settings()
    db_path = _sqlite_path_from_url(settings.monitor_database_url)

    if not db_path or not db_path.exists():
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail="数据库文件不存在")

    # 创建临时只读副本（避免锁定数据库）
    temp_dir = Path(tempfile.gettempdir())
    temp_db_path = temp_dir / f"llm_datas_{db_path.stem}.db"

    try:
        # 复制数据库文件
        shutil.copy2(db_path, temp_db_path)

        # 返回文件
        return FileResponse(
            path=str(temp_db_path),
            filename="llm_datas.db",
            media_type="application/x-sqlite3",
            headers={
                "Cache-Control": "no-cache, no-store, must-revalidate",
                "Pragma": "no-cache",
                "Expires": "0",
                "Content-Disposition": 'attachment; filename="llm_datas.db"',
            },
        )
    except Exception as e:
        logger.error(f"下载数据库文件失败: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"下载数据库文件失败: {str(e)}")


async def export_data_json(request: Request) -> Response:
    """导出监控数据为JSON"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)

    # 解析查询参数
    time_range_hours = int(request.query_params.get("time_range_hours", "24"))
    time_range_hours = max(1, min(time_range_hours, 168))  # 限制在1-168小时

    # 调用统计数据
    statistics = await monitor_service.get_statistics(session, time_range_hours, limit=100)

    # 获取调用历史
    query = _parse_invocation_query(request)
    query.limit = min(query.limit, 1000)  # 最多导出1000条
    invocations, total = await monitor_service.get_invocations(session, query)

    # 构建导出数据
    export_data = {
        "export_time": datetime.utcnow().isoformat(),
        "time_range_hours": time_range_hours,
        "statistics": statistics.model_dump(mode='json'),
        "invocations": [inv.model_dump(mode='json') for inv in invocations],
        "total_invocations": total,
    }

    # 返回JSON文件
    return JSONResponse(
        export_data,
        headers={
            "Content-Disposition": f'attachment; filename="llm_router_export_{datetime.utcnow().strftime("%Y%m%d_%H%M%S")}.json"',
            "Cache-Control": "no-cache, no-store, must-revalidate",
        },
    )


async def export_data_excel(request: Request) -> Response:
    """导出监控数据为CSV（兼容Excel）"""
    session = request.state.session

    # 解析查询参数
    time_range_hours = int(request.query_params.get("time_range_hours", "24"))
    time_range_hours = max(1, min(time_range_hours, 168))

    # 获取监控数据
    monitor_service = _get_monitor_service(request)
    query = _parse_invocation_query(request)
    query.limit = min(query.limit, 1000)
    invocations, total = await monitor_service.get_invocations(session, query)

    # 创建临时文件
    temp_dir = Path(tempfile.gettempdir())
    temp_file = temp_dir / f"llm_router_export_{datetime.utcnow().strftime('%Y%m%d_%H%M%S')}.csv"

    # 写入CSV
    with open(temp_file, 'w', newline='', encoding='utf-8-sig') as csvfile:
        writer = csv.writer(csvfile)
        # 写入表头
        headers = [
            "ID", "Model", "Provider", "Status", "Started At",
            "Completed At", "Duration (ms)", "Prompt Tokens",
            "Completion Tokens", "Total Tokens", "Cost (USD)", "Error"
        ]
        writer.writerow(headers)

        # 写入数据
        for inv in invocations:
            row = [
                inv.id,
                inv.model_name,
                inv.provider_name,
                inv.status,
                inv.started_at.isoformat() if inv.started_at else "",
                inv.completed_at.isoformat() if inv.completed_at else "",
                inv.duration_ms,
                inv.prompt_tokens,
                inv.completion_tokens,
                inv.total_tokens,
                inv.cost,
                inv.error_message or ""
            ]
            writer.writerow(row)

    # 返回文件
    return FileResponse(
        path=str(temp_file),
        filename=f"llm_router_export_{datetime.utcnow().strftime('%Y%m%d_%H%M%S')}.csv",
        media_type="text/csv",
        headers={
            "Cache-Control": "no-cache, no-store, must-revalidate",
            "Content-Disposition": f'attachment; filename="llm_router_export_{datetime.utcnow().strftime("%Y%m%d_%H%M%S")}.csv"',
        },
    )


async def get_model(request: Request) -> Response:
    """获取单个模型的详细信息"""
    provider_name = request.path_params["provider_name"]
    model_name = request.path_params["model_name"]
    session = request.state.session
    service = _get_service(request)
    
    model = await service.get_model_by_name(session, provider_name, model_name)
    if not model:
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail=f"模型 {provider_name}/{model_name} 不存在")
    
    return JSONResponse(service.to_model_read(model).model_dump())


# API Key 管理端点
async def create_api_key(request: Request) -> Response:
    """创建 API Key"""
    payload = await parse_model_body(request, APIKeyCreate)
    
    session = request.state.session
    api_key_service = _get_api_key_service(request)

    try:
        api_key = await api_key_service.create_api_key(
            session,
            key=payload.key,
            name=payload.name,
            is_active=payload.is_active,
            allowed_models=payload.allowed_models,
            allowed_providers=payload.allowed_providers,
            parameter_limits=payload.parameter_limits,
        )
        # 注意：DBSessionMiddleware 会在请求结束时自动 commit
        data = APIKeyRead.model_validate(api_key)
        return JSONResponse(data.model_dump(mode="json"), status_code=HTTP_201_CREATED)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))


async def list_api_keys(request: Request) -> Response:
    """列出所有 API Key"""
    session = request.state.session
    api_key_service = _get_api_key_service(request)
    
    include_inactive = request.query_params.get("include_inactive", "false").lower() in {
        "1",
        "true",
        "yes",
    }
    
    api_keys = await api_key_service.list_api_keys(session, include_inactive=include_inactive)
    data = [APIKeyRead.model_validate(api_key).model_dump(mode="json") for api_key in api_keys]
    return JSONResponse(data)


async def get_api_key(request: Request) -> Response:
    """获取单个 API Key"""
    api_key_id = int(request.path_params["id"])
    session = request.state.session
    api_key_service = _get_api_key_service(request)
    
    api_key = await api_key_service.get_api_key_by_id(session, api_key_id)
    if not api_key:
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail="API Key 不存在")
    
    data = APIKeyRead.model_validate(api_key)
    return JSONResponse(data.model_dump(mode="json"))


async def update_api_key(request: Request) -> Response:
    """更新 API Key"""
    api_key_id = int(request.path_params["id"])

    payload = await parse_model_body(request, APIKeyUpdate)
    
    session = request.state.session
    api_key_service = _get_api_key_service(request)
    
    api_key = await api_key_service.get_api_key_by_id(session, api_key_id)
    if not api_key:
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail="API Key 不存在")
    
    try:
        updated = await api_key_service.update_api_key(
            session,
            api_key,
            name=payload.name,
            is_active=payload.is_active,
            allowed_models=payload.allowed_models,
            allowed_providers=payload.allowed_providers,
            parameter_limits=payload.parameter_limits,
        )
        # 注意：DBSessionMiddleware 会在请求结束时自动 commit
        data = APIKeyRead.model_validate(updated)
        return JSONResponse(data.model_dump(mode="json"))
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))


async def delete_api_key(request: Request) -> Response:
    """删除 API Key"""
    api_key_id = int(request.path_params["id"])
    session = request.state.session
    api_key_service = _get_api_key_service(request)
    
    api_key = await api_key_service.get_api_key_by_id(session, api_key_id)
    if not api_key:
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail="API Key 不存在")
    
    await api_key_service.delete_api_key(session, api_key)
    # 注意：DBSessionMiddleware 会在请求结束时自动 commit
    return Response(status_code=HTTP_204_NO_CONTENT)


# ==================== 认证路由 ====================

async def login(request: Request) -> Response:
    """登录：使用 API Key 获取 Session Token"""
    try:
        body = await request.json()
    except ValueError:
        # 如果 JSON 解析失败，body 为空字典，继续尝试从 header 获取
        body = {}
    
    api_key = body.get("api_key") or extract_api_key(request)
    
    if not api_key:
        raise HTTPException(
            status_code=HTTP_401_UNAUTHORIZED,
            detail="请提供 API Key（通过请求体 api_key 字段或 Authorization 头）"
        )
    
    settings = load_settings()
    api_key_config = settings.get_api_key_config(api_key)

    # 支持运行时通过 POST /api-keys 创建的 key：settings 未命中时查数据库
    if api_key_config is None and hasattr(request.app.state, "api_key_service") and hasattr(request.app.state, "session_factory"):
        async with request.app.state.session_factory() as db_session:
            api_key_record = await request.app.state.api_key_service.get_api_key_by_key(db_session, api_key)
            if api_key_record is not None and api_key_record.is_active:
                api_key_config = request.app.state.api_key_service.to_api_key_config(api_key_record)

    if api_key_config is None:
        raise HTTPException(
            status_code=HTTP_403_FORBIDDEN,
            detail="无效的 API Key"
        )

    session_store = get_session_store()
    token = session_store.create_session(api_key_config)

    # 记录登录成功到 Redis
    ip_address = request.client.host if request.client else "unknown"
    record = LoginRecord(
        ip_address=ip_address,
        auth_type="api_key",
        is_success=True,
        is_local=is_local_request(request),
    )
    try:
        login_service = get_login_record_service()
        asyncio.create_task(login_service.create_login_record(record))
    except Exception:
        pass

    return JSONResponse({
        "token": token,
        "expires_in": session_store.default_ttl,
        "message": "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。"
    })


async def bind_model(request: Request) -> Response:
    """绑定默认/strong/weak 模型到 session。"""
    token = extract_session_token(request)
    if not token:
        raise HTTPException(
            status_code=HTTP_401_UNAUTHORIZED,
            detail="缺少 Session Token",
        )

    payload = await parse_model_body(request, BindModelRequest)
    session_store = get_session_store()
    session_data = session_store.get_session(token)
    if session_data is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail="Session 不存在或已过期",
        )

    service = _get_service(request)
    session = request.state.session
    model = await service.get_model_by_name(session, payload.provider_name, payload.model_name)
    if model is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail=f"模型 {payload.provider_name}/{payload.model_name} 不存在",
        )
    if not model.is_active:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"模型 {payload.provider_name}/{payload.model_name} 未激活",
        )

    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(payload.provider_name, payload.model_name):
        raise HTTPException(
            status_code=HTTP_403_FORBIDDEN,
            detail=f"API Key 不允许访问模型 {payload.provider_name}/{payload.model_name}",
        )

    ok = session_store.bind_profile_model(
        token,
        payload.provider_name,
        payload.model_name,
        payload.binding_type,
    )
    if not ok:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail="Session 不存在或已过期",
        )

    return JSONResponse(
        {
            "message": f"模型 {payload.provider_name}/{payload.model_name} 已绑定到 session ({payload.binding_type})",
            "provider_name": payload.provider_name,
            "model_name": payload.model_name,
            "binding_type": payload.binding_type,
        }
    )


async def logout(request: Request) -> Response:
    """登出：使 Session Token 失效"""
    token = extract_session_token(request)
    if not token:
        raise HTTPException(
            status_code=HTTP_401_UNAUTHORIZED,
            detail="缺少 Session Token"
        )
    
    session_store = get_session_store()
    deleted = session_store.delete_session(token)
    
    if deleted:
        return JSONResponse({"message": "登出成功"})
    else:
        return JSONResponse({"message": "Session 不存在或已过期"}, status_code=HTTP_404_NOT_FOUND)


async def get_login_records(request: Request) -> Response:
    """获取登录记录列表（从 Redis）"""
    limit = min(int(request.query_params.get("limit", 100)), 500)
    offset = max(int(request.query_params.get("offset", 0)), 0)
    auth_type = request.query_params.get("auth_type") or None
    is_success_str = request.query_params.get("is_success")
    is_success = None
    if is_success_str is not None:
        is_success = is_success_str.lower() in ("true", "1", "yes")

    try:
        login_service = get_login_record_service()
        records, total = await login_service.get_login_records(
            limit=limit,
            offset=offset,
            auth_type=auth_type,
            is_success=is_success,
        )
    except Exception as e:
        logger.warning("获取登录记录失败（Redis 可能未连接）: %s", e)
        # Redis 不可用时返回 200 + 空数据，避免前端报错；前端可根据 redis_available 提示
        return JSONResponse({
            "records": [],
            "total": 0,
            "redis_available": False,
        })

    return JSONResponse({
        "records": [r.model_dump(mode="json") for r in records],
        "total": total,
        "redis_available": True,
    })


# ==================== OpenAI 兼容 API ====================

async def openai_chat_completions(request: Request) -> Response:
    """
    标准 OpenAI 兼容的聊天完成端点：POST /v1/chat/completions
    
    model 参数在请求体中指定，格式为 "provider_name/model_name"
    例如: {"model": "openrouter/glm-4.5-air", "messages": [...]}
    
    支持以下 model 格式:
    1. "provider/model" - 例如 "openrouter/glm-4.5-air"
    2. 如果 session 已绑定模型，可以省略 model 参数
    """
    try:
        body = await read_json_body(request)
    except HTTPException as exc:
        if exc.status_code == HTTP_400_BAD_REQUEST:
            logger.warning("openai_chat_completions 400 (JSON/body): %s", exc.detail)
        raise

    try:
        openai_request = OpenAICompatibleChatCompletionRequest.model_validate(body)
    except ValidationError as exc:
        detail = f"无效的请求参数: {str(exc)}"
        logger.warning("openai_chat_completions 400: %s", detail)
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=detail
        ) from exc

    if not openai_request.messages:
        detail = "messages 字段不能为空"
        logger.warning("openai_chat_completions 400: %s", detail)
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=detail
        )

    # 转换消息格式，支持多模态 content（OpenAI 格式列表）
    supported_roles = {"system", "user", "assistant"}
    messages = []
    for msg in openai_request.messages:
        if msg.role not in supported_roles or not msg.content:
            continue
        content = normalize_multimodal_content(msg.content)
        if content:
            messages.append(ChatMessage(role=cast(Any, msg.role), content=content))
    
    if not messages:
        detail = "至少需要一个包含 content 的消息（角色必须是 system, user 或 assistant）"
        logger.warning("openai_chat_completions 400: %s", detail)
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=detail
        )

    # 解析/路由 model 参数（显式 model > request routing_mode > session 默认绑定）
    session_data = getattr(request.state, "session_data", None)
    provider_name: Optional[str] = None
    model_name: Optional[str] = None
    explicit_model = False

    if openai_request.model:
        model_str = openai_request.model.strip()
        if "/" not in model_str:
            detail = "model 参数必须包含 provider，格式为 'provider/model'，例如 'openrouter/glm-4.5-air'"
            logger.warning("openai_chat_completions 400: %s", detail)
            raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=detail)
        provider_name, model_name = _split_provider_model(model_str)
        explicit_model = True
    elif openai_request.routing_mode:
        route_payload = RouteDecisionRequest(
            routing_mode=openai_request.routing_mode,
            messages=messages,
            prompt=None,
        )
        provider_name, model_name = await _resolve_routing_mode_target(
            request,
            route_payload,
            openai_request.routing_mode,
        )
    elif session_data and session_data.provider_name and session_data.model_name:
        provider_name = session_data.provider_name
        model_name = session_data.model_name

    if not provider_name or not model_name:
        detail = "未指定可用模型。请传 model=provider/model，或提供 routing_mode，或先绑定 session 默认模型"
        logger.warning("openai_chat_completions 400: %s", detail)
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=detail)

    logger.info(f"解析的模型: provider={provider_name}, model={model_name}")

    # 检查 API Key 限制
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config and not api_key_config.is_model_allowed(provider_name, model_name):
        raise HTTPException(
            status_code=HTTP_403_FORBIDDEN,
            detail=f"API Key 不允许调用模型 {provider_name}/{model_name}",
        )

    session = request.state.session
    service = _get_service(request)
    model_for_call = await service.get_model_by_name(session, provider_name, model_name)
    if model_for_call is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail=f"模型 {provider_name}/{model_name} 不存在",
        )
    if not model_for_call.is_active:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"模型 {provider_name}/{model_name} 未激活",
        )

    # 仅在显式 model 模式下保持旧行为：自动回写 session 默认模型
    if explicit_model and session_data:
        should_bind_model = (
            not session_data.provider_name
            or not session_data.model_name
            or session_data.provider_name != provider_name
            or session_data.model_name != model_name
        )
        if should_bind_model:
            token = extract_session_token(request)
            if token:
                session_store = get_session_store()
                session_store.bind_model(token, provider_name, model_name)

    # 转换参数
    parameters = {}
    if openai_request.temperature is not None:
        parameters["temperature"] = openai_request.temperature
    if openai_request.top_p is not None:
        parameters["top_p"] = openai_request.top_p
    if openai_request.max_tokens is not None:
        parameters["max_tokens"] = openai_request.max_tokens
    if openai_request.stop is not None:
        parameters["stop"] = openai_request.stop if isinstance(openai_request.stop, list) else [openai_request.stop]
    if openai_request.presence_penalty is not None:
        parameters["presence_penalty"] = openai_request.presence_penalty
    if openai_request.frequency_penalty is not None:
        parameters["frequency_penalty"] = openai_request.frequency_penalty
    if openai_request.top_k is not None:
        parameters["top_k"] = openai_request.top_k
    if openai_request.repetition_penalty is not None:
        parameters["repetition_penalty"] = openai_request.repetition_penalty
    
    # 应用参数限制
    if api_key_config and api_key_config.parameter_limits:
        parameters = api_key_config.validate_parameters(parameters)
    
    # 如果请求体中有 model 字段且与路径不同，用它来覆盖数据库中的 remote_identifier
    remote_identifier_override = None
    if openai_request.model and openai_request.model != f"{provider_name}/{model_name}":
        # 用户提供了不同的 model 标识符，作为 remote_identifier_override
        remote_identifier_override = openai_request.model
    
    # 构建 ModelInvokeRequest
    invoke_request = ModelInvokeRequest(
        messages=messages,
        parameters=parameters,
        stream=openai_request.stream or False,
        remote_identifier_override=remote_identifier_override,
    )
    
    # 调用模型
    engine = _get_router_engine(request)

    if invoke_request.stream:
        response_id = f"chatcmpl-{uuid.uuid4().hex[:29]}"
        created_ts = int(time.time())
        
        # 从数据库获取模型对象，使用数据库中实际存储的模型名称
        model = await service.get_model_by_name(session, provider_name, model_name)
        if not model:
            raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail=f"模型 {provider_name}/{model_name} 不存在")
        # 使用数据库中实际存储的模型名称
        model_label = model.name
        
        try:
            stream = await engine.stream_by_identifier(
                session, provider_name, model_name, invoke_request
            )
        except RoutingError as exc:
            detail = str(exc)
            logger.warning("openai_chat_completions 400 (streaming RoutingError): %s", detail)
            raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=detail)

        async def event_stream() -> AsyncIterator[bytes]:
            async for chunk in stream:
                if chunk.is_final:
                    yield b"data: [DONE]\n\n"
                    break
                payload = {
                    "id": response_id,
                    "object": "chat.completion.chunk",
                    "created": created_ts,
                    "model": model_label,
                    "choices": _build_openai_stream_choices(chunk),
                }
                if chunk.usage:
                    payload["usage"] = dict(chunk.usage)
                    if chunk.cost is not None:
                        payload["usage"]["cost"] = chunk.cost
                data_str = json.dumps(payload, ensure_ascii=False)
                yield f"data: {data_str}\n\n".encode("utf-8")
            else:
                yield b"data: [DONE]\n\n"

        return StreamingResponse(event_stream(), media_type="text/event-stream")

    try:
        response = await engine.invoke_by_identifier(
            session, provider_name, model_name, invoke_request
        )
    except RoutingError as exc:
        detail = str(exc)
        logger.warning("openai_chat_completions 400 (RoutingError): %s", detail)
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=detail)

    # 转换响应格式
    # 从 raw 响应中提取信息
    raw = response.raw or {}
    usage_info = raw.get("usage", {})
    
    # 从数据库获取模型对象，使用数据库中实际存储的模型名称
    model = await service.get_model_by_name(session, provider_name, model_name)
    if not model:
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail=f"模型 {provider_name}/{model_name} 不存在")
    # 使用数据库中实际存储的模型名称
    model_label = model.name
    
    # 生成响应 ID
    response_id = f"chatcmpl-{uuid.uuid4().hex[:29]}"
    
    # 构建 OpenAI 兼容的响应
    openai_response = OpenAICompatibleChatCompletionResponse(
        id=response_id,
        created=int(time.time()),
        model=model_label,
        choices=[
            OpenAICompatibleChoice(
                index=0,
                message=OpenAICompatibleMessage(
                    role="assistant",
                    content=response.output_text,
                ),
                finish_reason="stop",  # 可以根据实际情况调整
            )
        ],
        usage=OpenAICompatibleUsage(
            prompt_tokens=usage_info.get("prompt_tokens", 0),
            completion_tokens=usage_info.get("completion_tokens", 0),
            total_tokens=usage_info.get("total_tokens", 0),
            cost=response.cost,
        ) if usage_info else None,
    )
    
    return JSONResponse(openai_response.model_dump(exclude_none=True))


async def openai_list_models(request: Request) -> Response:
    session = request.state.session
    service = _get_service(request)

    query = ModelQuery(include_inactive=False)
    models = await service.list_models(session, query)

    # 去重模型名称，因为不同 provider 可能有同名模型
    unique_names = sorted(list(set(m.name for m in models)))
    data = [OpenAIModelInfo(id=name) for name in unique_names]

    return JSONResponse(OpenAIModelList(data=data).model_dump())


async def sync_config_from_file(request: Request) -> Response:
    """从配置文件同步配置到数据库"""
    from .app import reload_config_from_file
    from ..services import RateLimiterManager
    
    # 获取必要的服务和配置
    # 安全地获取 settings，如果不存在则重新加载
    settings: RouterSettings = getattr(request.app.state, "settings", None)
    if settings is None:
        settings = load_settings()
    
    model_service = _get_service(request)
    api_key_service = _get_api_key_service(request)
    session_factory = getattr(request.app.state, "session_factory", None)
    if session_factory is None:
        raise HTTPException(
            status_code=HTTP_500_INTERNAL_SERVER_ERROR,
            detail="服务尚未完全初始化，请稍后重试"
        )
    
    rate_limiter: RateLimiterManager = getattr(request.app.state, "rate_limiter", None)
    if rate_limiter is None:
        raise HTTPException(
            status_code=HTTP_500_INTERNAL_SERVER_ERROR,
            detail="服务尚未完全初始化，请稍后重试"
        )
    
    # 确定配置文件路径
    config_file: Path | None = None
    if settings.model_config_file and settings.model_config_file.exists():
        config_file = settings.model_config_file
    else:
        # 默认查找当前目录的 router.toml
        default_config_file = Path.cwd() / "router.toml"
        if default_config_file.exists():
            config_file = default_config_file
    
    if not config_file or not config_file.exists():
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail="配置文件不存在，请确保 router.toml 文件存在"
        )
    
    try:
        # 调用同步函数
        await reload_config_from_file(
            config_file,
            model_service,
            api_key_service,
            session_factory,
            rate_limiter,
            settings,
        )
        return JSONResponse({
            "success": True,
            "message": f"配置已从 {config_file} 同步到数据库",
            "config_file": str(config_file)
        })
    except Exception as e:
        logger.error(f"同步配置失败: {e}", exc_info=True)
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"同步配置失败: {str(e)}"
        )


# 定价相关API端点
async def get_latest_pricing(request: Request) -> Response:
    """获取最新定价信息（从网络）"""
    try:
        pricing_service = _get_pricing_service(request)
        all_pricing = await pricing_service.get_all_latest_pricing()
        
        # 转换为API响应格式
        result = {}
        for provider, pricing_list in all_pricing.items():
            result[provider] = [
                {
                    "model_name": p.model_name,
                    "provider": p.provider,
                    "input_price_per_1k": p.input_price_per_1k,
                    "output_price_per_1k": p.output_price_per_1k,
                    "source": p.source,
                    "last_updated": p.last_updated.isoformat(),
                    "notes": p.notes,
                }
                for p in pricing_list
            ]
        
        return JSONResponse(result)
    except Exception as e:
        logger.error(f"获取最新定价失败: {e}", exc_info=True)
        raise HTTPException(
            status_code=HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"获取最新定价失败: {str(e)}"
        )


async def get_pricing_suggestions(request: Request) -> Response:
    """获取定价更新建议（对比当前配置和最新定价）"""
    session = request.state.session
    service = _get_service(request)
    pricing_service = _get_pricing_service(request)
    
    try:
        # 获取所有模型
        query = ModelQuery(include_inactive=False)
        models = await service.list_models(session, query)
        
        suggestions = []
        for model in models:
            try:
                # 获取当前定价配置
                current_config = model.config or {}
                current_input = current_config.get("cost_per_1k_tokens")
                current_output = current_config.get("cost_per_1k_completion_tokens")
                # 获取最新定价
                provider_name = model.provider.name if hasattr(model, "provider") else None
                if not provider_name:
                    from sqlalchemy import select
                    from ..db.models import Provider
                    provider_result = await session.scalar(
                        select(Provider).where(Provider.id == model.provider_id)
                    )
                    provider_name = provider_result.name if provider_result else None
                if provider_name:
                    model_name_to_search = model.name
                    if model.remote_identifier:
                        model_name_to_search = model.remote_identifier.split("/")[-1]
                    latest_pricing = await pricing_service.get_latest_pricing(
                        model_name_to_search,
                        provider_name,
                    )
                    has_update = False
                    if latest_pricing:
                        if current_input is None or abs(current_input - latest_pricing.input_price_per_1k) > 0.0001:
                            has_update = True
                        elif current_output is None or abs(current_output - latest_pricing.output_price_per_1k) > 0.0001:
                            has_update = True
                    suggestions.append(
                        PricingSuggestion(
                            model_id=model.id,
                            model_name=model.name,
                            provider_name=provider_name,
                            current_input_price=current_input,
                            current_output_price=current_output,
                            latest_input_price=latest_pricing.input_price_per_1k if latest_pricing else None,
                            latest_output_price=latest_pricing.output_price_per_1k if latest_pricing else None,
                            has_update=has_update,
                            pricing_info=(
                                ModelPricingInfo(
                                    model_name=latest_pricing.model_name,
                                    provider=latest_pricing.provider,
                                    input_price_per_1k=latest_pricing.input_price_per_1k,
                                    output_price_per_1k=latest_pricing.output_price_per_1k,
                                    source=latest_pricing.source,
                                    last_updated=latest_pricing.last_updated,
                                    notes=latest_pricing.notes,
                                )
                                if latest_pricing
                                else None
                            ),
                        )
                    )
                else:
                    suggestions.append(
                        PricingSuggestion(
                            model_id=model.id,
                            model_name=model.name,
                            provider_name="unknown",
                            current_input_price=current_input,
                            current_output_price=current_output,
                            has_update=False,
                        )
                    )
            except Exception as per_model_error:
                logger.warning(
                    "获取单模型定价建议失败 model_id=%s model_name=%s: %s",
                    model.id,
                    getattr(model, "name", "?"),
                    per_model_error,
                    exc_info=False,
                )
                current_config = model.config or {}
                suggestions.append(
                    PricingSuggestion(
                        model_id=model.id,
                        model_name=model.name,
                        provider_name=(
                            model.provider.name if hasattr(model, "provider") and model.provider else "unknown"
                        ),
                        current_input_price=current_config.get("cost_per_1k_tokens"),
                        current_output_price=current_config.get("cost_per_1k_completion_tokens"),
                        latest_input_price=None,
                        latest_output_price=None,
                        has_update=False,
                        pricing_info=None,
                    )
                )
        return JSONResponse([s.model_dump() for s in suggestions])
    except Exception as e:
        logger.error(f"获取定价建议失败: {e}", exc_info=True)
        raise HTTPException(
            status_code=HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"获取定价建议失败: {str(e)}"
        )


async def sync_model_pricing(request: Request, model_id: int) -> Response:
    """同步指定模型的定价"""
    session = request.state.session
    service = _get_service(request)
    pricing_service = _get_pricing_service(request)
    
    try:
        # 获取模型
        model = await service.get_model_by_id(session, model_id)
        if not model:
            raise HTTPException(
                status_code=HTTP_404_NOT_FOUND,
                detail=f"模型 ID {model_id} 不存在"
            )
        
        # 获取provider信息
        provider = await session.scalar(
            select(Provider).where(Provider.id == model.provider_id)
        )
        if not provider:
            raise HTTPException(
                status_code=HTTP_404_NOT_FOUND,
                detail=f"Provider ID {model.provider_id} 不存在"
            )
        
        # 获取最新定价
        model_name_to_search = model.name
        if model.remote_identifier:
            model_name_to_search = model.remote_identifier.split("/")[-1]
        
        latest_pricing = await pricing_service.get_latest_pricing(
            model_name_to_search,
            provider.name
        )
        
        if not latest_pricing:
            return JSONResponse(PricingSyncResponse(
                success=False,
                message=f"未找到模型 {model.name} 的最新定价信息",
            ).model_dump())
        
        # 更新模型配置
        config = model.config.copy() if model.config else {}
        config["cost_per_1k_tokens"] = latest_pricing.input_price_per_1k
        config["cost_per_1k_completion_tokens"] = latest_pricing.output_price_per_1k
        
        # 更新模型
        await service.update_model(
            session,
            model,
            ModelUpdate(config=config)
        )
        
        return JSONResponse(PricingSyncResponse(
            success=True,
            message=f"模型 {model.name} 的定价已更新",
            updated_pricing=ModelPricingInfo(
                model_name=latest_pricing.model_name,
                provider=latest_pricing.provider,
                input_price_per_1k=latest_pricing.input_price_per_1k,
                output_price_per_1k=latest_pricing.output_price_per_1k,
                source=latest_pricing.source,
                last_updated=latest_pricing.last_updated,
                notes=latest_pricing.notes,
            ),
        ).model_dump())
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"同步模型定价失败: {e}", exc_info=True)
        raise HTTPException(
            status_code=HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"同步模型定价失败: {str(e)}"
        )


async def sync_all_pricing(request: Request) -> Response:
    """同步所有模型的定价"""
    session = request.state.session
    service = _get_service(request)
    pricing_service = _get_pricing_service(request)
    
    try:
        # 获取所有模型
        query = ModelQuery(include_inactive=False)
        models = await service.list_models(session, query)
        
        results = {
            "success": 0,
            "failed": 0,
            "not_found": 0,
            "details": [],
        }
        
        for model in models:
            try:
                # 获取provider信息
                provider = await session.scalar(
                    select(Provider).where(Provider.id == model.provider_id)
                )
                if not provider:
                    results["not_found"] += 1
                    results["details"].append({
                        "model_id": model.id,
                        "model_name": model.name,
                        "status": "failed",
                        "message": "Provider不存在",
                    })
                    continue
                
                # 获取最新定价
                model_name_to_search = model.name
                if model.remote_identifier:
                    model_name_to_search = model.remote_identifier.split("/")[-1]
                
                latest_pricing = await pricing_service.get_latest_pricing(
                    model_name_to_search,
                    provider.name
                )
                
                if not latest_pricing:
                    results["not_found"] += 1
                    results["details"].append({
                        "model_id": model.id,
                        "model_name": model.name,
                        "status": "not_found",
                        "message": "未找到最新定价",
                    })
                    continue
                
                # 更新模型配置
                config = model.config.copy() if model.config else {}
                config["cost_per_1k_tokens"] = latest_pricing.input_price_per_1k
                config["cost_per_1k_completion_tokens"] = latest_pricing.output_price_per_1k
                
                await service.update_model(
                    session,
                    model,
                    ModelUpdate(config=config)
                )
                
                results["success"] += 1
                results["details"].append({
                    "model_id": model.id,
                    "model_name": model.name,
                    "status": "success",
                    "message": "定价已更新",
                })
            except Exception as e:
                results["failed"] += 1
                results["details"].append({
                    "model_id": model.id,
                    "model_name": model.name,
                    "status": "failed",
                    "message": str(e),
                })
                logger.error(f"同步模型 {model.name} 定价失败: {e}")
        
        return JSONResponse({
            "success": True,
            "message": f"批量同步完成: 成功 {results['success']}, 失败 {results['failed']}, 未找到 {results['not_found']}",
            "results": results,
        })
    except Exception as e:
        logger.error(f"批量同步定价失败: {e}", exc_info=True)
        raise HTTPException(
            status_code=HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"批量同步定价失败: {str(e)}"
        )


async def get_statistics(request: Request) -> Response:
    """获取统计信息（从独立的监控数据库）"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)

    try:
        time_range_hours = int(request.query_params.get("time_range_hours", "24"))
        limit = int(request.query_params.get("limit", "10"))
    except ValueError as exc:
        raise HTTPException(status_code=400, detail="time_range_hours 和 limit 必须是整数") from exc

    if time_range_hours <= 0:
        raise HTTPException(status_code=400, detail="time_range_hours 必须大于 0")
    if time_range_hours > 168:
        raise HTTPException(status_code=400, detail="time_range_hours 不能大于 168")
    if limit <= 0:
        raise HTTPException(status_code=400, detail="limit 必须大于 0")
    if limit > 100:
        raise HTTPException(status_code=400, detail="limit 不能大于 100")

    stats = await monitor_service.get_statistics(session, time_range_hours, limit)
    return JSONResponse(stats.model_dump(mode="json"))


async def get_time_series(request: Request) -> Response:
    """获取时间序列数据（从独立的监控数据库）"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    granularity = request.query_params.get("granularity", "day")
    time_range_hours = int(request.query_params.get("time_range_hours", "168"))
    
    try:
        data = await monitor_service.get_time_series(session, granularity, time_range_hours)
        return JSONResponse(data.model_dump(mode="json"))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


async def get_grouped_time_series(request: Request) -> Response:
    """获取按模型或provider分组的时间序列数据（从独立的监控数据库）"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    group_by = request.query_params.get("group_by", "model")
    granularity = request.query_params.get("granularity", "day")
    time_range_hours = int(request.query_params.get("time_range_hours", "168"))
    
    try:
        data = await monitor_service.get_grouped_time_series(session, group_by, granularity, time_range_hours)
        return JSONResponse(data.model_dump(mode="json"))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


async def get_invocations(request: Request) -> Response:
    """获取调用历史列表（从独立的监控数据库）"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    query = _parse_invocation_query(request)
    invocations, total = await monitor_service.get_invocations(session, query)
    return JSONResponse({
        "items": [inv.model_dump(mode="json") for inv in invocations],
        "total": total,
        "limit": query.limit,
        "offset": query.offset,
    })


async def get_invocation(request: Request) -> Response:
    """获取单次调用详情（从独立的监控数据库）"""
    invocation_id = int(request.path_params["id"])
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    inv = await monitor_service.get_invocation_by_id(session, invocation_id)
    if not inv:
        raise HTTPException(status_code=404, detail="调用记录不存在")
    
    return JSONResponse(inv.model_dump(mode="json"))
