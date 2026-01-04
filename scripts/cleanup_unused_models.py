#!/usr/bin/env python3
"""
清理数据库中不在配置文件中的模型
"""

import asyncio
import sys
from pathlib import Path

# 添加项目根目录到路径
project_root = Path(__file__).parent.parent
sys.path.insert(0, str(project_root))

from src.llm_router.config import load_settings
from src.llm_router.db import create_engine, create_session_factory, init_db
from src.llm_router.model_config import load_model_config
from src.llm_router.services import ModelService, RateLimiterManager
from src.llm_router.services.download import ModelDownloader
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from src.llm_router.db.models import Model, Provider


async def cleanup_unused_models(dry_run: bool = True) -> None:
    """清理数据库中不在配置文件中的模型"""
    print("加载配置...")
    settings = load_settings()
    
    # 确定配置文件路径
    config_file = settings.model_config_file if settings.model_config_file else Path.cwd() / "router.toml"
    
    if not config_file.exists():
        print(f"错误: 配置文件不存在: {config_file}")
        return
    
    print(f"读取配置文件: {config_file}")
    config_data = load_model_config(config_file)
    
    # 构建配置文件中定义的模型集合 (provider_name, model_name)
    config_models = set()
    for model_cfg in config_data.models:
        config_models.add((model_cfg.provider, model_cfg.name))
    
    print(f"配置文件中定义了 {len(config_models)} 个模型")
    
    # 初始化数据库连接
    print("连接数据库...")
    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)
    
    # 创建服务
    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    model_service = ModelService(downloader, rate_limiter)
    
    async with session_factory() as session:
        # 获取数据库中所有模型，预加载 provider 关系
        from sqlalchemy.orm import selectinload
        stmt = select(Model).options(selectinload(Model.provider))
        result = await session.scalars(stmt)
        db_models = result.unique().all()
        
        print(f"数据库中共有 {len(db_models)} 个模型")
        
        # 找出不在配置文件中的模型
        unused_models = []
        for model in db_models:
            provider_name = model.provider.name if model.provider else "unknown"
            model_key = (provider_name, model.name)
            if model_key not in config_models:
                unused_models.append(model)
        
        if not unused_models:
            print("✅ 没有需要清理的模型，所有模型都在配置文件中")
            return
        
        print(f"\n发现 {len(unused_models)} 个不在配置文件中的模型:")
        for model in unused_models:
            provider_name = model.provider.name if model.provider else "unknown"
            print(f"  - {provider_name}/{model.name} (id: {model.id})")
        
        if dry_run:
            print("\n⚠️  这是预览模式（dry-run），不会实际删除模型")
            print("   要实际删除，请运行: python scripts/cleanup_unused_models.py --execute")
        else:
            print("\n开始删除...")
            for model in unused_models:
                provider_name = model.provider.name if model.provider else "unknown"
                try:
                    await model_service.remove_model(session, model)
                    print(f"  ✅ 已删除: {provider_name}/{model.name}")
                except Exception as e:
                    print(f"  ❌ 删除失败 {provider_name}/{model.name}: {e}")
            
            await session.commit()
            print(f"\n✅ 清理完成！共删除了 {len(unused_models)} 个模型")
    
    await engine.dispose()


if __name__ == "__main__":
    import sys
    
    dry_run = "--execute" not in sys.argv
    if dry_run:
        print("=" * 60)
        print("模型清理工具 - 预览模式")
        print("=" * 60)
    else:
        print("=" * 60)
        print("模型清理工具 - 执行模式")
        print("=" * 60)
    
    asyncio.run(cleanup_unused_models(dry_run=dry_run))

