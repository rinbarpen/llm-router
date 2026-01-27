import asyncio
from src.llm_router.db.session import create_engine, create_session_factory
from src.llm_router.db.models import Model
from sqlalchemy import select

async def check():
    from pathlib import Path
    project_root = Path(__file__).parent.parent.parent
    db_path = project_root / 'llm_router.db'
    engine = create_engine(f'sqlite+aiosqlite:///{db_path}')
    session_factory = create_session_factory(engine)
    async with session_factory() as session:
        result = await session.execute(select(Model))
        models = result.scalars().all()
        for m in models:
            print(f'Model: {m.name}, Provider ID: {m.provider_id}, Active: {m.is_active}')

if __name__ == "__main__":
    asyncio.run(check())
