from __future__ import annotations

from typing import List

from sqlalchemy import select
from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import JSONResponse, Response
from starlette.status import HTTP_200_OK, HTTP_201_CREATED

from ..db.models import Provider, ProviderType
from ..schemas import (
    ModelCreate,
    ModelInvokeRequest,
    ModelQuery,
    ModelUpdate,
    ProviderCreate,
    ProviderRead,
)
from ..services import ModelService, RouterEngine, RoutingError


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


async def health(_: Request) -> Response:
    return JSONResponse({"status": "ok"}, status_code=HTTP_200_OK)


async def create_provider(request: Request) -> Response:
    payload = ProviderCreate(**await request.json())
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
    payload = ModelCreate(**await request.json())
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
    payload = ModelInvokeRequest(**await request.json())

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
    body = await request.json()
    query_payload = body.get("query", {})
    request_payload = body.get("request", {})

    query = ModelQuery.model_validate(query_payload)
    payload = ModelInvokeRequest.model_validate(request_payload)

    engine = _get_router_engine(request)
    session = request.state.session

    try:
        response = await engine.route_by_tags(session, query, payload)
    except RoutingError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    return JSONResponse(response.model_dump())


async def update_model(request: Request) -> Response:
    provider_name = request.path_params["provider_name"]
    model_name = request.path_params["model_name"]
    payload = ModelUpdate(**await request.json())

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


