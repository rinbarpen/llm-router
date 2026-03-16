from __future__ import annotations

import asyncio
import json
from typing import Any, AsyncIterator, Iterable

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from ..services.cli_conversation_store import get_cli_conversation_store
from .base import BaseProviderClient, ProviderError


class CodeCLIProviderClient(BaseProviderClient):
    """Generic local Code CLI provider based on subprocess execution."""

    PROVIDER_NAME = "code_cli"
    DEFAULT_EXECUTABLE = "code"
    DEFAULT_TIMEOUT_SECONDS = 300.0
    DEFAULT_ARGS_TEMPLATE: list[str] = ["exec", "--json", "-m", "{model}", "{prompt}"]
    DEFAULT_PARSER = "codex_jsonl"

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        prompt = self._build_prompt(request)
        if not prompt:
            raise ProviderError(f"{self.PROVIDER_NAME} 需要非空 prompt/messages")

        executable = str(
            self.provider.settings.get("executable", self.DEFAULT_EXECUTABLE)
        ).strip() or self.DEFAULT_EXECUTABLE
        model_identifier = self._resolve_model_identifier(model, request)

        timeout = float(
            self.provider.settings.get("timeout", self.DEFAULT_TIMEOUT_SECONDS)
        )
        if timeout <= 0:
            timeout = self.DEFAULT_TIMEOUT_SECONDS

        conversation_key = request.conversation_id
        store = get_cli_conversation_store()
        info = (
            store.get(self.provider.type, conversation_key)
            if conversation_key
            else None
        )
        cli_session_id = info.cli_id if info else None

        args_template = self._get_template(
            "args_template",
            self.DEFAULT_ARGS_TEMPLATE,
        )
        resume_template = self._get_template("resume_args_template", None)
        can_resume = bool(conversation_key and cli_session_id and resume_template)
        selected_template = resume_template if can_resume and resume_template else args_template

        rendered_args = self._render_template(
            selected_template,
            model=model_identifier,
            prompt=prompt,
            session_id=cli_session_id or "",
        )

        command: list[str] = [executable, *rendered_args]

        try:
            process = await asyncio.create_subprocess_exec(
                *command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
        except Exception as exc:
            raise ProviderError(f"启动 {self.PROVIDER_NAME} CLI 失败: {exc}") from exc

        try:
            stdout_bytes, stderr_bytes = await asyncio.wait_for(
                process.communicate(), timeout=timeout
            )
        except asyncio.TimeoutError as exc:
            process.kill()
            await process.communicate()
            raise ProviderError(
                f"{self.PROVIDER_NAME} CLI 调用超时（>{timeout:.0f}s）"
            ) from exc

        stdout_text = stdout_bytes.decode("utf-8", errors="replace")
        stderr_text = stderr_bytes.decode("utf-8", errors="replace").strip()

        parser = str(self.provider.settings.get("parser", self.DEFAULT_PARSER)).strip().lower()
        output_text, usage, session_id, parsed_count = self._parse_output(stdout_text, parser)

        if process.returncode != 0:
            if conversation_key and cli_session_id:
                store.delete(self.provider.type, conversation_key)
            message = stderr_text
            if not message and stdout_text:
                message = stdout_text.strip()[:500]
                if len(stdout_text) > 500:
                    message += "..."
            if not message:
                message = "无 stderr 输出"
            raise ProviderError(
                f"{self.PROVIDER_NAME} CLI 调用失败（exit={process.returncode}）: {message}"
            )

        if not output_text:
            message = stderr_text or f"{self.PROVIDER_NAME} CLI 未返回可解析文本输出"
            raise ProviderError(message)

        if conversation_key and session_id:
            token_count = 0
            if isinstance(usage, dict):
                token_count = usage.get("input_tokens", 0) + usage.get("output_tokens", 0)
            store.set(self.provider.type, conversation_key, session_id, token_count)

        raw: dict[str, Any] = {
            "provider": self.provider.name,
            "model": model_identifier,
            "parser": parser,
        }
        if parsed_count:
            raw["events_count"] = parsed_count
        if usage:
            raw["usage"] = usage
        if stderr_text:
            raw["stderr"] = stderr_text
        if session_id:
            raw["session_id"] = session_id

        return ModelInvokeResponse(output_text=output_text, raw=raw)

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        response = await self.invoke(model, request)
        usage = response.raw.get("usage") if isinstance(response.raw, dict) else None
        yield ModelStreamChunk(text=response.output_text, raw=response.raw, usage=usage)
        yield ModelStreamChunk(is_final=True, usage=usage)

    async def list_supported_models(self) -> list[str]:
        configured = self.provider.settings.get("supported_models")
        if isinstance(configured, list):
            models = [str(item).strip() for item in configured if str(item).strip()]
            if models:
                return list(dict.fromkeys(models))

        list_args_template = self._get_template("list_models_args_template", [])
        if not list_args_template:
            return []

        executable = str(
            self.provider.settings.get("executable", self.DEFAULT_EXECUTABLE)
        ).strip() or self.DEFAULT_EXECUTABLE
        timeout = float(
            self.provider.settings.get("list_models_timeout", self.DEFAULT_TIMEOUT_SECONDS)
        )
        if timeout <= 0:
            timeout = self.DEFAULT_TIMEOUT_SECONDS

        try:
            process = await asyncio.create_subprocess_exec(
                executable,
                *list_args_template,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
        except Exception as exc:
            raise ProviderError(f"启动 {self.PROVIDER_NAME} CLI 列表命令失败: {exc}") from exc

        try:
            stdout_bytes, stderr_bytes = await asyncio.wait_for(
                process.communicate(), timeout=timeout
            )
        except asyncio.TimeoutError as exc:
            process.kill()
            await process.communicate()
            raise ProviderError(
                f"{self.PROVIDER_NAME} CLI 列表命令超时（>{timeout:.0f}s）"
            ) from exc

        if process.returncode != 0:
            stderr_text = stderr_bytes.decode("utf-8", errors="replace").strip()
            raise ProviderError(
                f"{self.PROVIDER_NAME} CLI 列表命令失败（exit={process.returncode}）: {stderr_text or '无 stderr 输出'}"
            )

        stdout_text = stdout_bytes.decode("utf-8", errors="replace")
        parser = str(self.provider.settings.get("list_models_parser", "lines")).strip().lower()
        return self._parse_model_list(stdout_text, parser)

    def _get_template(
        self,
        key: str,
        default: list[str] | None,
    ) -> list[str]:
        value = self.provider.settings.get(key)
        if isinstance(value, list) and value:
            result = [str(item) for item in value if str(item).strip()]
            if result:
                return result
        return list(default or [])

    def _render_template(self, template: Iterable[str], **kwargs: str) -> list[str]:
        rendered: list[str] = []
        for token in template:
            try:
                value = token.format(**kwargs)
            except Exception:
                value = token
            if value.strip():
                rendered.append(value)
        return rendered

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

    def _parse_output(
        self,
        stdout_text: str,
        parser: str,
    ) -> tuple[str, dict[str, Any], str | None, int]:
        if parser == "single_json":
            return self._parse_single_json(stdout_text)
        return self._parse_codex_jsonl(stdout_text)

    def _parse_codex_jsonl(
        self,
        stdout_text: str,
    ) -> tuple[str, dict[str, Any], str | None, int]:
        texts: list[str] = []
        usage: dict[str, Any] = {}
        session_id: str | None = None
        parsed_count = 0

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
            parsed_count += 1

            event_type = str(payload.get("type", ""))
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

            if not session_id:
                session_id = self._extract_session_id(payload)

        return "\n\n".join(texts).strip(), usage, session_id, parsed_count

    def _parse_single_json(
        self,
        stdout_text: str,
    ) -> tuple[str, dict[str, Any], str | None, int]:
        text_field = str(self.provider.settings.get("text_field", "result"))
        usage_field = str(self.provider.settings.get("usage_field", "usage"))
        session_id_field = str(self.provider.settings.get("session_id_field", "session_id"))

        data: dict[str, Any] | None = None
        try:
            maybe = json.loads(stdout_text.strip())
            if isinstance(maybe, dict):
                data = maybe
        except json.JSONDecodeError:
            for line in stdout_text.splitlines():
                line = line.strip()
                if not line:
                    continue
                try:
                    maybe = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if isinstance(maybe, dict):
                    data = maybe

        if not data:
            return "", {}, None, 0

        output = data.get(text_field)
        if isinstance(output, str):
            output_text = output.strip()
        elif output is None:
            output_text = ""
        else:
            output_text = str(output).strip()

        usage_raw = data.get(usage_field)
        usage = usage_raw if isinstance(usage_raw, dict) else {}

        sid_raw = data.get(session_id_field)
        session_id = sid_raw.strip() if isinstance(sid_raw, str) and sid_raw.strip() else None

        return output_text, usage, session_id, 1

    def _extract_session_id(self, payload: dict[str, Any]) -> str | None:
        for key in ("session_id", "thread_id", "conversation_id"):
            value = payload.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip()

        item = payload.get("item")
        if isinstance(item, dict):
            for key in ("session_id", "thread_id", "conversation_id"):
                value = item.get(key)
                if isinstance(value, str) and value.strip():
                    return value.strip()

        return None

    def _parse_model_list(self, stdout_text: str, parser: str) -> list[str]:
        if parser == "json_array":
            try:
                data = json.loads(stdout_text.strip())
            except json.JSONDecodeError:
                return []
            if isinstance(data, list):
                return list(dict.fromkeys(str(item).strip() for item in data if str(item).strip()))
            return []

        if parser == "jsonl":
            models: list[str] = []
            key = str(self.provider.settings.get("list_models_jsonl_key", "model"))
            for line in stdout_text.splitlines():
                raw_line = line.strip()
                if not raw_line:
                    continue
                try:
                    payload = json.loads(raw_line)
                except json.JSONDecodeError:
                    continue
                if isinstance(payload, dict):
                    value = payload.get(key)
                    if isinstance(value, str) and value.strip():
                        models.append(value.strip())
            return list(dict.fromkeys(models))

        models = [line.strip() for line in stdout_text.splitlines() if line.strip()]
        return list(dict.fromkeys(models))


__all__ = ["CodeCLIProviderClient"]
