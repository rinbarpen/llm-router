from __future__ import annotations

import re
from typing import Any, List, Type, TypeVar

from pydantic import BaseModel, ValidationError
from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.status import HTTP_400_BAD_REQUEST

T = TypeVar("T", bound=BaseModel)


def normalize_multimodal_content(content: str | list | None) -> str | list[dict[str, Any]]:
    """
    将 OpenAI 多模态 content 转为统一格式。
    - 字符串 -> 原样返回
    - 列表 [{type: "text", text: "..."}, {type: "image_url", image_url: {url: "..."}}]
      -> 转为 ChatMessage 可用的格式，image_url 的 data:...;base64,... 保留
    """
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if not isinstance(content, list):
        return str(content)
    result: List[dict[str, Any]] = []
    for part in content:
        if not isinstance(part, dict):
            continue
        t = part.get("type")
        if t == "text":
            text = part.get("text", "")
            if text:
                result.append({"type": "text", "text": text})
        elif t == "image_url":
            img = part.get("image_url") or part.get("imageUrl") or {}
            url = img.get("url", "") if isinstance(img, dict) else ""
            if url:
                result.append({"type": "image_url", "url": url})
    if len(result) == 1 and result[0].get("type") == "text":
        return result[0]["text"]
    return result if result else ""


def _extract_base64_from_data_url(url: str) -> tuple[str, str]:
    """从 data:image/png;base64,xxx 提取 mime_type 和 base64 数据"""
    m = re.match(r"data:([^;]+);base64,(.+)", url.strip())
    if m:
        return m.group(1).strip(), m.group(2).strip()
    return "image/png", ""



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


__all__ = ["read_json_body", "parse_model_body", "normalize_multimodal_content"]

