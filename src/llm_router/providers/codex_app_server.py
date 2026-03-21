"""Codex app-server JSON-RPC 客户端，用于常驻进程模式"""

from __future__ import annotations

import asyncio
import json
import logging
import time
from typing import Any, AsyncIterator, Optional

logger = logging.getLogger(__name__)


class CodexAppServerError(Exception):
    """Codex app-server 调用错误"""

    pass


class CodexAppServerClient:
    """Codex app-server stdio JSON-RPC 客户端"""

    def __init__(self, process: asyncio.subprocess.Process) -> None:
        self._process = process
        self._next_id = 1
        self._pending: dict[int, asyncio.Future[dict[str, Any]]] = {}
        self._notification_queue: asyncio.Queue[dict[str, Any]] = asyncio.Queue()
        self._read_task: Optional[asyncio.Task[None]] = None
        self._initialized = False

    async def initialize(self) -> None:
        """执行 initialize + initialized 握手"""
        if self._initialized:
            return
        await self._ensure_reader()
        # initialize
        await self._request("initialize", {"clientInfo": {"name": "llm_router", "title": "LLM Router", "version": "1.0.0"}})
        # initialized notification
        await self._send_notification("initialized", {})
        self._initialized = True
        logger.info("Codex app-server 初始化完成")

    async def _ensure_reader(self) -> None:
        """启动 stdout 读取任务"""
        if self._read_task is None or self._read_task.done():
            self._read_task = asyncio.create_task(self._read_loop())

    async def _read_loop(self) -> None:
        """读取 stdout 行并分发响应/通知"""
        if self._process.stdout is None:
            return
        try:
            while True:
                line = await self._process.stdout.readline()
                if not line:
                    break
                line_str = line.decode("utf-8", errors="replace").strip()
                if not line_str:
                    continue
                try:
                    msg = json.loads(line_str)
                except json.JSONDecodeError:
                    logger.debug("Codex app-server 非 JSON 行: %s", line_str[:100])
                    continue
                if "id" in msg:
                    fut = self._pending.pop(msg["id"], None)
                    if fut is not None and not fut.done():
                        fut.set_result(msg)
                else:
                    # 通知放入队列，供 turn_start 等消费
                    try:
                        self._notification_queue.put_nowait(msg)
                    except asyncio.QueueFull:
                        logger.warning("Codex 通知队列已满，丢弃: %s", msg.get("method", ""))
        except asyncio.CancelledError:
            pass
        except Exception as e:
            logger.warning("Codex app-server 读取异常: %s", e)
        finally:
            for fut in self._pending.values():
                if not fut.done():
                    fut.set_exception(CodexAppServerError("连接关闭"))
            self._pending.clear()

    def _next_request_id(self) -> int:
        rid = self._next_id
        self._next_id += 1
        return rid

    async def _request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        """发送 JSON-RPC 请求并等待响应"""
        rid = self._next_request_id()
        fut: asyncio.Future[dict[str, Any]] = asyncio.get_running_loop().create_future()
        self._pending[rid] = fut
        payload = {"method": method, "id": rid, "params": params}
        if self._process.stdin is None:
            raise CodexAppServerError("stdin 不可用")
        self._process.stdin.write((json.dumps(payload) + "\n").encode("utf-8"))
        await self._process.stdin.drain()
        try:
            result = await asyncio.wait_for(fut, timeout=60.0)
        except asyncio.TimeoutError:
            self._pending.pop(rid, None)
            raise CodexAppServerError(f"{method} 超时")
        if "error" in result:
            err = result["error"]
            raise CodexAppServerError(f"{method} 错误: {err.get('message', err)}")
        return result.get("result", {})

    async def _send_notification(self, method: str, params: dict[str, Any]) -> None:
        """发送 JSON-RPC 通知（无 id）"""
        payload = {"method": method, "params": params}
        if self._process.stdin is None:
            raise CodexAppServerError("stdin 不可用")
        self._process.stdin.write((json.dumps(payload) + "\n").encode("utf-8"))
        await self._process.stdin.drain()

    @staticmethod
    def _build_sandbox_policy(
        sandbox_mode: str,
        network_access: bool,
    ) -> dict[str, Any]:
        mode = str(sandbox_mode or "workspace-write").strip().lower().replace("_", "-")
        if mode in ("readonly", "read-only"):
            policy_type = "readOnly"
        elif mode in ("danger-full-access", "full-access", "dangerfullaccess"):
            policy_type = "dangerFullAccess"
        else:
            policy_type = "workspaceWrite"
        return {"type": policy_type, "networkAccess": bool(network_access)}

    async def thread_start(
        self,
        model: str,
        cwd: Optional[str] = None,
        approval_policy: str = "never",
        sandbox: str = "workspace-write",
    ) -> str:
        """创建新 thread，返回 thread_id"""
        params: dict[str, Any] = {"model": model, "approvalPolicy": approval_policy, "sandbox": sandbox}
        if cwd:
            params["cwd"] = cwd
        result = await self._request("thread/start", params)
        thread = result.get("thread", {})
        thread_id = thread.get("id")
        if not thread_id:
            raise CodexAppServerError("thread/start 未返回 thread.id")
        return thread_id

    async def thread_resume(
        self,
        thread_id: str,
        model: Optional[str] = None,
    ) -> None:
        """恢复已有 thread"""
        params: dict[str, Any] = {"threadId": thread_id}
        if model:
            params["model"] = model
        await self._request("thread/resume", params)

    async def turn_start(
        self,
        thread_id: str,
        prompt: str,
        model: Optional[str] = None,
        cwd: Optional[str] = None,
        approval_policy: str = "never",
        sandbox_mode: str = "workspace-write",
        network_access: bool = True,
    ) -> AsyncIterator[tuple[str, dict[str, Any]]]:
        """启动 turn，流式接收 item/completed（assistant_message）和 turn/completed"""
        params: dict[str, Any] = {
            "threadId": thread_id,
            "input": [{"type": "text", "text": prompt}],
            "approvalPolicy": approval_policy,
            "sandboxPolicy": self._build_sandbox_policy(sandbox_mode, network_access),
        }
        if model:
            params["model"] = model
        if cwd:
            params["cwd"] = cwd

        # 需要边读通知边 yield，这里简化：先发 turn/start，再轮询读直到 turn/completed
        result = await self._request("turn/start", params)
        turn = result.get("turn", {})
        if not turn.get("id"):
            raise CodexAppServerError("turn/start 未返回 turn.id")

        # 从通知队列消费直到 turn/completed
        output_parts: list[str] = []
        usage: dict[str, Any] = {}
        timeout_sec = 300.0
        deadline = time.monotonic() + timeout_sec
        while True:
            remaining = max(0.1, deadline - time.monotonic())
            try:
                msg = await asyncio.wait_for(self._notification_queue.get(), timeout=remaining)
            except asyncio.TimeoutError:
                raise CodexAppServerError(f"turn 超时（>{timeout_sec}s）")
            method = msg.get("method", "")
            if method == "item/completed":
                p = msg.get("params", {})
                item = p.get("item", {})
                itype = item.get("item_type") or item.get("type", "")
                # agentMessage: {id, text, phase?} per Codex app-server schema
                if itype in ("agentMessage", "agent_message", "assistant_message"):
                    text = item.get("text", "")
                    if isinstance(text, str) and text.strip():
                        output_parts.append(text.strip())
                        yield (text.strip(), {})
            elif method == "turn/completed":
                p = msg.get("params", {})
                usage = p.get("usage") or p.get("turn", {}).get("usage") or {}
                break

        output_text = "\n\n".join(output_parts).strip()
        yield (output_text, {"usage": usage})

    async def invoke(
        self,
        model: str,
        prompt: str,
        thread_id: Optional[str] = None,
        resume: bool = False,
        cwd: Optional[str] = None,
        approval_policy: str = "never",
        sandbox_mode: str = "workspace-write",
        network_access: bool = True,
    ) -> tuple[str, str, dict[str, Any]]:
        """单次调用：有 thread_id 且 resume 则 resume，否则 start；然后 turn/start，收集输出。返回 (thread_id, output_text, usage)"""
        if thread_id and resume:
            await self.thread_resume(thread_id, model)
        else:
            thread_id = await self.thread_start(
                model,
                cwd=cwd,
                approval_policy=approval_policy,
                sandbox=sandbox_mode,
            )

        output_text = ""
        usage: dict[str, Any] = {}
        async for part, meta in self.turn_start(
            thread_id,
            prompt,
            model=model,
            cwd=cwd,
            approval_policy=approval_policy,
            sandbox_mode=sandbox_mode,
            network_access=network_access,
        ):
            if isinstance(part, str) and part:
                output_text = part if not output_text else output_text + "\n\n" + part
            if meta.get("usage"):
                usage = meta["usage"]

        return (thread_id, output_text, usage)

    async def aclose(self) -> None:
        """关闭连接"""
        if self._read_task and not self._read_task.done():
            self._read_task.cancel()
            try:
                await self._read_task
            except asyncio.CancelledError:
                pass
        if self._process.returncode is None:
            self._process.terminate()
            try:
                await asyncio.wait_for(self._process.wait(), timeout=5.0)
            except asyncio.TimeoutError:
                self._process.kill()
                await self._process.wait()


# 全局 app-server 实例，由 app lifespan 设置/清除
_codex_app_server: Optional[CodexAppServerClient] = None


def get_codex_app_server() -> Optional[CodexAppServerClient]:
    """获取全局 Codex app-server 客户端（若已启动）"""
    return _codex_app_server


def set_codex_app_server(client: Optional[CodexAppServerClient]) -> None:
    """设置全局 Codex app-server 客户端"""
    global _codex_app_server
    _codex_app_server = client


__all__ = [
    "CodexAppServerClient",
    "CodexAppServerError",
    "get_codex_app_server",
    "set_codex_app_server",
]
