#!/usr/bin/env python3
"""
为现有数据库添加cost列的迁移脚本
如果cost列已存在，则跳过
"""

import asyncio
import sqlite3
from pathlib import Path


async def add_cost_column(db_path: Path) -> None:
    """为model_invocations表添加cost列（如果不存在）"""
    conn = sqlite3.connect(str(db_path))
    cursor = conn.cursor()
    
    try:
        # 检查cost列是否已存在
        cursor.execute("PRAGMA table_info(model_invocations)")
        columns = [row[1] for row in cursor.fetchall()]
        
        if 'cost' not in columns:
            print(f"Adding 'cost' column to model_invocations table...")
            cursor.execute("ALTER TABLE model_invocations ADD COLUMN cost REAL")
            conn.commit()
            print("✓ Successfully added 'cost' column")
        else:
            print("✓ 'cost' column already exists")
    except Exception as e:
        print(f"✗ Error: {e}")
        conn.rollback()
    finally:
        conn.close()


if __name__ == "__main__":
    import sys
    
    if len(sys.argv) > 1:
        db_path = Path(sys.argv[1])
    else:
        db_path = Path.cwd() / "llm_router.db"
    
    if not db_path.exists():
        print(f"Database file not found: {db_path}")
        sys.exit(1)
    
    asyncio.run(add_cost_column(db_path))

