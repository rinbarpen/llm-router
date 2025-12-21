import asyncio
from src.llm_router.db.session import create_engine, create_session_factory
from src.llm_router.db.models import Model
from sqlalchemy import select

async def check():
    engine = create_engine('sqlite+aiosqlite:///llm_router.db')
    session_factory = create_session_factory(engine)
    async with session_factory() as session:
        result = await session.execute(select(Model))
        models = result.scalars().all()
        for m in models:
            print(f'Model: {m.name}, Provider ID: {m.provider_id}, Active: {m.is_active}')

if __name__ == "__main__":
    asyncio.run(check())
