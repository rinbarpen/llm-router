#!/usr/bin/env python3
"""检查 Provider 的 API Key 状态"""

import asyncio
from sqlalchemy import select

from llm_router.config import load_settings
from llm_router.db import create_engine, create_session_factory, init_db
from llm_router.db.models import Provider


async def check_provider_api_keys():
    """检查所有 Provider 的 API Key"""
    settings = load_settings()
    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)
    
    async with session_factory() as session:
        result = await session.scalars(select(Provider))
        providers = result.all()
        
        print("Provider API Key 状态:")
        print("-" * 60)
        for p in providers:
            has_key = p.api_key is not None and p.api_key != ""
            print(f"{p.name:20} | API Key: {'✅ 已设置' if has_key else '❌ 未设置'}")
            if not has_key:
                print(f"  └─ 需要从环境变量读取: {p.settings.get('api_key_env', 'N/A')}")
    
    await engine.dispose()


if __name__ == "__main__":
    asyncio.run(check_provider_api_keys())

