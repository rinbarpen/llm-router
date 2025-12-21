#!/usr/bin/env python3
"""
手动同步配置文件到数据库的脚本
用于在服务运行时重新加载配置
"""

import asyncio
from pathlib import Path

from llm_router.config import load_settings
from llm_router.db import create_engine, create_session_factory, init_db
from llm_router.model_config import apply_model_config, load_model_config
from llm_router.services import ModelDownloader, ModelService, RateLimiterManager


async def sync_config():
    """同步配置文件到数据库"""
    print("加载配置...")
    settings = load_settings()
    
    # 确定配置文件路径
    config_file = settings.model_config_file if settings.model_config_file else Path.cwd() / "router.toml"
    
    if not config_file.exists():
        print(f"错误: 配置文件不存在: {config_file}")
        return
    
    print(f"读取配置文件: {config_file}")
    config_data = load_model_config(config_file)
    print(f"找到 {len(config_data.providers)} 个 Providers 和 {len(config_data.models)} 个 Models")
    
    # 初始化数据库连接
    print("连接数据库...")
    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)
    
    # 创建服务
    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    model_service = ModelService(downloader, rate_limiter)
    
    # 应用配置
    print("同步配置到数据库...")
    await apply_model_config(config_data, model_service, session_factory)
    
    print("✅ 配置同步完成！")
    
    # 验证
    async with session_factory() as session:
        from sqlalchemy import select
        from sqlalchemy.orm import joinedload
        from llm_router.db.models import Provider, Model
        
        providers = await session.scalars(select(Provider))
        models = await session.scalars(select(Model).options(joinedload(Model.provider)))
        
        provider_list = list(providers.all())
        model_list = list(models.unique().all())
        
        print(f"\n数据库中的 Providers: {len(provider_list)}")
        for p in provider_list:
            print(f"  - {p.name} ({p.type})")
        
        print(f"\n数据库中的 Models: {len(model_list)}")
        gemini_models = [m for m in model_list if m.provider and m.provider.name == "gemini"]
        print(f"  Gemini 模型: {len(gemini_models)}")
        for m in gemini_models:
            print(f"    - {m.name} (active: {m.is_active})")
    
    await engine.dispose()


if __name__ == "__main__":
    asyncio.run(sync_config())

