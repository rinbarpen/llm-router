from __future__ import annotations

from typing import Any, Type, TypeVar

from pydantic import BaseModel, ValidationError
from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.status import HTTP_400_BAD_REQUEST

T = TypeVar("T", bound=BaseModel)


async def read_json_body(
    request: Request,
    *,
    allow_empty: bool = False,
    error_detail: str = "请求体必须是有效的 JSON 格式",
) -> dict[str, Any]:
    """统一的 JSON 解析入口，确保错误信息一致。"""
    try:
        data = await request.json()
    except ValueError as exc:  # noqa: B902
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=error_detail) from exc

    if not data and not allow_empty:
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="请求体不能为空",
        )

    if not isinstance(data, dict):
        raise HTTPException(
            status_code=HTTP_400_BAD_REQUEST,
            detail="请求体必须是 JSON 对象",
        )
    return data


async def parse_model_body(
    request: Request,
    model_cls: Type[T],
    *,
    allow_empty: bool = False,
    error_detail: str = "请求体必须是有效的 JSON 格式",
) -> T:
    """解析 JSON 并通过 Pydantic 校验，返回模型实例。"""
    data = await read_json_body(request, allow_empty=allow_empty, error_detail=error_detail)
    try:
        return model_cls.model_validate(data)
    except ValidationError as exc:
        raise HTTPException(status_code=HTTP_400_BAD_REQUEST, detail=str(exc)) from exc


__all__ = ["read_json_body", "parse_model_body"]

