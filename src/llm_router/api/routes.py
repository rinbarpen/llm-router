from __future__ import annotations

from datetime import datetime
from typing import List

from sqlalchemy import select
from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import JSONResponse, Response
from starlette.status import (
    HTTP_200_OK,
    HTTP_201_CREATED,
    HTTP_204_NO_CONTENT,
    HTTP_400_BAD_REQUEST,
    HTTP_401_UNAUTHORIZED,
    HTTP_403_FORBIDDEN,
    HTTP_404_NOT_FOUND,
)

from ..db.models import InvocationStatus, Provider, ProviderType
from ..schemas import (
    APIKeyCreate,
    APIKeyRead,
    APIKeyUpdate,
    InvocationQuery,
    InvocationRead,
    ModelCreate,
    ModelInvokeRequest,
    ModelQuery,
    ModelUpdate,
    ProviderCreate,
    ProviderRead,
    StatisticsResponse,
    TimeSeriesResponse,
)
from ..services import APIKeyService, ModelService, MonitorService, RouterEngine, RoutingError
from .session_store import get_session_store


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


def _get_monitor_service(request: Request) -> MonitorService:
    service = getattr(request.app.state, "monitor_service", None)
    if service is None:
        raise RuntimeError("MonitorService 尚未初始化")
    return service


def _get_api_key_service(request: Request) -> APIKeyService:
    service = getattr(request.app.state, "api_key_service", None)
    if service is None:
        raise RuntimeError("APIKeyService 尚未初始化")
    return service


async def health(_: Request) -> Response:
    return JSONResponse({"status": "ok"}, status_code=HTTP_200_OK)


async def create_provider(request: Request) -> Response:
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    try:
        payload = ProviderCreate(**body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    
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


async def create_model(request: Request) -> Response:
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    try:
        payload = ModelCreate(**body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    
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


async def invoke_model(request: Request) -> Response:
    provider_name = request.path_params["provider_name"]
    model_name = request.path_params["model_name"]
    
    # 解析请求体，处理空请求或无效 JSON
    try:
        body = await request.json()
    except ValueError as exc:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式。请提供 prompt 或 messages 字段。"
        )
    
    if not body:
        raise HTTPException(
            status_code=400,
            detail="请求体不能为空。请提供 prompt 或 messages 字段。"
        )
    
    try:
        payload = ModelInvokeRequest(**body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

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

    try:
        response = await engine.invoke_by_identifier(
            session, provider_name, model_name, payload
        )
    except RoutingError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    return JSONResponse(response.model_dump())


async def route_model(request: Request) -> Response:
    # 解析请求体，处理空请求或无效 JSON
    try:
        body = await request.json()
    except ValueError as exc:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式。请提供 query 和 request 字段。"
        )
    
    if not body:
        raise HTTPException(
            status_code=400,
            detail="请求体不能为空。请提供 query 和 request 字段。"
        )
    
    query_payload = body.get("query", {})
    request_payload = body.get("request", {})

    try:
        query = ModelQuery.model_validate(query_payload)
        payload = ModelInvokeRequest.model_validate(request_payload)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    # 检查 API Key 限制
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config:
        # 应用参数限制
        if payload.parameters and api_key_config.parameter_limits:
            payload.parameters = api_key_config.validate_parameters(payload.parameters)
        # 注意：模型限制在 RouterEngine 中通过过滤候选模型来处理

    engine = _get_router_engine(request)
    session = request.state.session

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
    
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    try:
        payload = ModelUpdate(**body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

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


# Monitor endpoints
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


async def get_invocations(request: Request) -> Response:
    """获取调用历史列表"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    query = _parse_invocation_query(request)
    invocations, total = await monitor_service.get_invocations(session, query)
    
    return JSONResponse({
        "items": [inv.model_dump(mode='json') for inv in invocations],
        "total": total,
        "limit": query.limit,
        "offset": query.offset,
    })


async def get_invocation_by_id(request: Request) -> Response:
    """获取单次调用的详细信息"""
    invocation_id = int(request.path_params["id"])
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    invocation = await monitor_service.get_invocation_by_id(session, invocation_id)
    if not invocation:
        raise HTTPException(status_code=HTTP_404_NOT_FOUND, detail="调用记录不存在")
    
    return JSONResponse(invocation.model_dump(mode='json'))


async def get_statistics(request: Request) -> Response:
    """获取统计信息"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    time_range_hours = int(request.query_params.get("time_range_hours", "24"))
    limit = int(request.query_params.get("limit", "10"))
    
    if time_range_hours <= 0 or time_range_hours > 168:  # 最多7天
        raise HTTPException(status_code=400, detail="time_range_hours必须在1-168之间")
    if limit <= 0 or limit > 100:
        raise HTTPException(status_code=400, detail="limit必须在1-100之间")
    
    statistics = await monitor_service.get_statistics(session, time_range_hours, limit)
    return JSONResponse(statistics.model_dump(mode='json'))


async def get_time_series(request: Request) -> Response:
    """获取时间序列数据"""
    session = request.state.session
    monitor_service = _get_monitor_service(request)
    
    granularity = request.query_params.get("granularity", "day")
    if granularity not in ["hour", "day", "week", "month"]:
        raise HTTPException(status_code=400, detail="granularity必须是 hour, day, week 或 month")
    
    time_range_hours = int(request.query_params.get("time_range_hours", "168"))
    if time_range_hours <= 0 or time_range_hours > 720:  # 最多30天
        raise HTTPException(status_code=400, detail="time_range_hours必须在1-720之间")
    
    time_series = await monitor_service.get_time_series(session, granularity, time_range_hours)
    return JSONResponse(time_series.model_dump(mode='json'))


# API Key 管理端点
async def create_api_key(request: Request) -> Response:
    """创建 API Key"""
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    try:
        payload = APIKeyCreate(**body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    
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
        await session.commit()
        data = APIKeyRead.model_validate(api_key)
        return JSONResponse(data.model_dump(), status_code=HTTP_201_CREATED)
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
    data = [APIKeyRead.model_validate(api_key).model_dump() for api_key in api_keys]
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
    return JSONResponse(data.model_dump())


async def update_api_key(request: Request) -> Response:
    """更新 API Key"""
    api_key_id = int(request.path_params["id"])
    
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=400,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    try:
        payload = APIKeyUpdate(**body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    
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
        await session.commit()
        data = APIKeyRead.model_validate(updated)
        return JSONResponse(data.model_dump())
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
    await session.commit()
    return Response(status_code=HTTP_204_NO_CONTENT)


# ==================== 认证路由 ====================

async def login(request: Request) -> Response:
    """登录：使用 API Key 获取 Session Token"""
    from ..config import load_settings
    from .auth import extract_api_key
    
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
    
    if api_key_config is None:
        raise HTTPException(
            status_code=HTTP_403_FORBIDDEN,
            detail="无效的 API Key"
        )
    
    session_store = get_session_store()
    token = session_store.create_session(api_key_config)
    
    return JSONResponse({
        "token": token,
        "expires_in": session_store.default_ttl,
        "message": "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。"
    })


async def bind_model(request: Request) -> Response:
    """绑定模型到 Session Token"""
    from .auth import extract_session_token
    
    token = extract_session_token(request)
    if not token:
        raise HTTPException(
            status_code=HTTP_401_UNAUTHORIZED,
            detail="缺少 Session Token"
        )
    
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    provider_name = body.get("provider_name")
    model_name = body.get("model_name")
    
    if not provider_name or not model_name:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="请提供 provider_name 和 model_name"
        )
    
    # 验证模型是否存在且可用
    session = request.state.session
    service = _get_service(request)
    model = await service.get_model_by_name(session, provider_name, model_name)
    if model is None:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail=f"模型 {provider_name}/{model_name} 不存在"
        )
    if not model.is_active:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"模型 {provider_name}/{model_name} 未激活"
        )
    
    # 检查 API Key 是否允许访问该模型
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config:
        if not api_key_config.is_model_allowed(provider_name, model_name):
            raise HTTPException(
                status_code=HTTP_403_FORBIDDEN,
                detail=f"API Key 不允许访问模型 {provider_name}/{model_name}"
            )
    
    # 绑定模型到 session
    session_store = get_session_store()
    success = session_store.bind_model(token, provider_name, model_name)
    
    if not success:
        raise HTTPException(
            status_code=HTTP_404_NOT_FOUND,
            detail="Session 不存在或已过期"
        )
    
    return JSONResponse({
        "message": f"模型 {provider_name}/{model_name} 已绑定到 session",
        "provider_name": provider_name,
        "model_name": model_name
    })


async def logout(request: Request) -> Response:
    """登出：使 Session Token 失效"""
    from .auth import extract_session_token
    
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


# ==================== OpenAI 兼容 API ====================

async def openai_chat_completions(request: Request) -> Response:
    """OpenAI 兼容的聊天完成端点：POST /v1/chat/completions"""
    import time
    import uuid
    from ..schemas import (
        OpenAICompatibleChatCompletionRequest,
        OpenAICompatibleChatCompletionResponse,
        OpenAICompatibleChoice,
        OpenAICompatibleMessage,
        OpenAICompatibleUsage,
        ChatMessage,
        ModelInvokeRequest,
    )
    
    # 解析请求体
    try:
        body = await request.json()
    except ValueError:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="请求体必须是有效的 JSON 格式"
        )
    
    try:
        openai_request = OpenAICompatibleChatCompletionRequest(**body)
    except ValueError as exc:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail=f"无效的请求参数: {str(exc)}"
        )
    
    if not openai_request.messages:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="messages 字段不能为空"
        )
    
    # 确定使用的模型
    session_data = getattr(request.state, "session_data", None)
    provider_name = None
    model_name = None
    should_bind_model = False
    
    if openai_request.model:
        # 从请求中的 model 字段解析，格式可能是 "provider/model" 或 "model"
        model_parts = openai_request.model.split("/", 1)
        if len(model_parts) == 2:
            provider_name, model_name = model_parts
        else:
            model_name = model_parts[0]
            # 如果 session 中有 provider_name，使用它
            if session_data and session_data.provider_name:
                provider_name = session_data.provider_name
        
        # 如果 session 中没有绑定模型，或者绑定的模型与请求中的不同，需要绑定
        if session_data:
            if not session_data.provider_name or not session_data.model_name:
                should_bind_model = True
            elif session_data.provider_name != provider_name or session_data.model_name != model_name:
                should_bind_model = True
    
    # 如果请求中没有指定模型，尝试从 session 中获取
    if not provider_name or not model_name:
        if session_data and session_data.provider_name and session_data.model_name:
            provider_name = session_data.provider_name
            model_name = session_data.model_name
        else:
            raise HTTPException(
                status_code=HTTP_400_BAD_REQUEST,
                detail="未指定模型。请使用 /auth/bind-model 绑定模型，或在请求的 model 字段中指定模型（格式：provider/model 或 model）"
            )
    
    # 如果需要绑定模型到 session，先验证模型
    if should_bind_model and session_data:
        session = request.state.session
        service = _get_service(request)
        model = await service.get_model_by_name(session, provider_name, model_name)
        if model is None:
            raise HTTPException(
                status_code=HTTP_404_NOT_FOUND,
                detail=f"模型 {provider_name}/{model_name} 不存在"
            )
        if not model.is_active:
            raise HTTPException(
                status_code=HTTP_400_BAD_REQUEST,
                detail=f"模型 {provider_name}/{model_name} 未激活"
            )
        
        # 检查 API Key 是否允许访问该模型
        api_key_config = getattr(request.state, "api_key_config", None)
        if api_key_config:
            if not api_key_config.is_model_allowed(provider_name, model_name):
                raise HTTPException(
                    status_code=HTTP_403_FORBIDDEN,
                    detail=f"API Key 不允许访问模型 {provider_name}/{model_name}"
                )
        
        # 绑定模型到 session
        from .session_store import get_session_store
        from .auth import extract_session_token
        token = extract_session_token(request)
        if token:
            session_store = get_session_store()
            session_store.bind_model(token, provider_name, model_name)
    
    # 检查 API Key 限制
    api_key_config = getattr(request.state, "api_key_config", None)
    if api_key_config:
        if not api_key_config.is_model_allowed(provider_name, model_name):
            raise HTTPException(
                status_code=HTTP_403_FORBIDDEN,
                detail=f"API Key 不允许调用模型 {provider_name}/{model_name}",
            )
    
    # 转换消息格式
    # 只转换支持的角色：system, user, assistant
    supported_roles = {"system", "user", "assistant"}
    messages = [
        ChatMessage(role=msg.role, content=msg.content or "")
        for msg in openai_request.messages
        if msg.role in supported_roles and msg.content  # 只包含有内容且支持角色的消息
    ]
    
    if not messages:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="至少需要一个包含 content 的消息（角色必须是 system, user 或 assistant）"
        )
    
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
    
    # 构建 ModelInvokeRequest
    invoke_request = ModelInvokeRequest(
        messages=messages,
        parameters=parameters,
        stream=openai_request.stream or False,
    )
    
    # 调用模型
    engine = _get_router_engine(request)
    session = request.state.session
    
    try:
        response = await engine.invoke_by_identifier(
            session, provider_name, model_name, invoke_request
        )
    except RoutingError as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc))
    
    # 转换响应格式
    # 从 raw 响应中提取信息
    raw = response.raw or {}
    usage_info = raw.get("usage", {})
    
    # 生成响应 ID
    response_id = f"chatcmpl-{uuid.uuid4().hex[:29]}"
    
    # 构建 OpenAI 兼容的响应
    openai_response = OpenAICompatibleChatCompletionResponse(
        id=response_id,
        created=int(time.time()),
        model=f"{provider_name}/{model_name}",
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
        ) if usage_info else None,
    )
    
    return JSONResponse(openai_response.model_dump(exclude_none=True))


