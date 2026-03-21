"""Claude Code CLI provider - 使用本地 `claude -p` 调用，依赖登录态，支持会话延续。"""

from __future__ import annotations

import asyncio
import json
import os
from typing import Any, AsyncIterator

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from ..services.cli_conversation_store import get_cli_conversation_store
from .base import BaseProviderClient, ProviderError


class ClaudeCodeCLIProviderClient(BaseProviderClient):
    """Claude Code CLI provider client backed by local `claude -p`，支持 --resume 延续会话。"""

    DEFAULT_EXECUTABLE = "claude"
    DEFAULT_TIMEOUT_SECONDS = 300.0

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        prompt = self._build_prompt(request)
        if not prompt:
            raise ProviderError("Claude Code CLI 需要非空 prompt/messages")

        executable = str(
            self.provider.settings.get("executable", self.DEFAULT_EXECUTABLE)
        ).strip() or self.DEFAULT_EXECUTABLE
        workspace_path = self._resolve_cli_workspace_path(request)

        conversation_key = request.conversation_id
        store = get_cli_conversation_store()
        info = store.get(ProviderType.CLAUDE_CODE_CLI, conversation_key) if conversation_key else None
        claude_session_id: str | None = info.cli_id if info else None

        permission_mode = str(
            self.provider.settings.get("permission_mode", "bypassPermissions")
        ).strip() or "bypassPermissions"
        model_id = model.remote_identifier or model.name
        command: list[str] = [
            executable,
            "--output-format",
            "json",
            "--model",
            model_id,
            "--permission-mode",
            permission_mode,
        ]
        if claude_session_id:
            command.extend(["--resume", claude_session_id])
        command.extend(["-p", prompt])

        extra_args = self.provider.settings.get("args", [])
        if isinstance(extra_args, list):
            command.extend(str(arg) for arg in extra_args if str(arg).strip())

        timeout = float(
            self.provider.settings.get("timeout", self.DEFAULT_TIMEOUT_SECONDS)
        )
        if timeout <= 0:
            timeout = self.DEFAULT_TIMEOUT_SECONDS

        env = os.environ.copy()
        env.pop("ANTHROPIC_API_KEY", None)  # 强制 claude_code_cli 使用 claude.ai 登录

        try:
            process = await asyncio.create_subprocess_exec(
                *command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
                env=env,
                cwd=workspace_path,
            )
        except Exception as exc:
            raise ProviderError(f"启动 Claude Code CLI 失败: {exc}") from exc

        try:
            stdout_bytes, stderr_bytes = await asyncio.wait_for(
                process.communicate(), timeout=timeout
            )
        except asyncio.TimeoutError as exc:
            process.kill()
            await process.communicate()
            raise ProviderError(
                f"Claude Code CLI 调用超时（>{timeout:.0f}s）"
            ) from exc

        stdout_text = stdout_bytes.decode("utf-8", errors="replace")
        stderr_text = stderr_bytes.decode("utf-8", errors="replace").strip()

        if process.returncode != 0:
            if conversation_key and claude_session_id:
                # 可能是 session 失效或上下文超限，清除映射
                store.delete(ProviderType.CLAUDE_CODE_CLI, conversation_key)
            # 优先用 stderr，无则用 stdout 前 500 字符（claude 有时把错误写到 stdout）
            message = stderr_text
            if not message and stdout_text:
                message = stdout_text.strip()[:500]
                if len(stdout_text) > 500:
                    message += "..."
            if not message:
                message = (
                    "无 stderr 输出。请确认已安装并登录 claude CLI（claude auth login），"
                    "或手动运行 claude -p 'hello' --output-format json 排查"
                )
            raise ProviderError(
                f"Claude Code CLI 调用失败（exit={process.returncode}）: {message}"
            )

        output_text, usage, session_id = self._parse_json_output(stdout_text)
        if not output_text:
            message = stderr_text or "Claude Code CLI 未返回可解析文本输出"
            raise ProviderError(message)

        # 更新会话映射：首次调用或 resume 后返回新的 session_id
        if conversation_key and session_id:
            token_count = 0
            if isinstance(usage, dict):
                token_count = usage.get("input_tokens", 0) + usage.get("output_tokens", 0)
            store.set(ProviderType.CLAUDE_CODE_CLI, conversation_key, session_id, token_count)

        raw: dict[str, Any] = {
            "provider": "claude_code_cli",
            "model": model.remote_identifier or model.name,
        }
        if usage:
            raw["usage"] = usage
        if stderr_text:
            raw["stderr"] = stderr_text

        return ModelInvokeResponse(output_text=output_text, raw=raw)

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        # 当前 claude -p 的 stream-json 格式较复杂，先按伪流式输出
        response = await self.invoke(model, request)
        usage = response.raw.get("usage") if isinstance(response.raw, dict) else None
        yield ModelStreamChunk(text=response.output_text, raw=response.raw, usage=usage)
        yield ModelStreamChunk(is_final=True, usage=usage)

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

    def _parse_json_output(self, stdout_text: str) -> tuple[str, dict[str, Any], str | None]:
        """解析 claude -p --output-format json 的输出，返回 (output_text, usage, session_id)。"""
        usage: dict[str, Any] = {}
        output_text = ""
        session_id: str | None = None

        try:
            data = json.loads(stdout_text.strip())
        except json.JSONDecodeError:
            # 可能输出被截断或混入非 JSON，尝试提取 result
            for line in stdout_text.splitlines():
                line = line.strip()
                if not line:
                    continue
                try:
                    data = json.loads(line)
                    if isinstance(data, dict):
                        output_text = data.get("result") or output_text
                        if "usage" in data:
                            usage = data.get("usage") or usage
                        if "session_id" in data:
                            session_id = data.get("session_id") or session_id
                except json.JSONDecodeError:
                    continue
            return (output_text.strip() if output_text else "", usage, session_id)

        if isinstance(data, dict):
            output_text = data.get("result") or ""
            usage = data.get("usage") or {}
            sid = data.get("session_id")
            if isinstance(sid, str) and sid.strip():
                session_id = sid.strip()
            if isinstance(output_text, str):
                return (output_text.strip(), usage, session_id)
            return (str(output_text).strip(), usage, session_id)

        return ("", usage, session_id)


__all__ = ["ClaudeCodeCLIProviderClient"]
