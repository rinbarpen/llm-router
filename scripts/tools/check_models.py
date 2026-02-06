#!/usr/bin/env python3
import sqlite3

from sqlalchemy.engine import make_url

from src.llm_router.config import load_settings

settings = load_settings()
# 从 database_url 解析 SQLite 文件路径（仅支持 sqlite）
url = make_url(settings.database_url)
db_path = url.database if url.drivername and "sqlite" in url.drivername else None
if not db_path:
    raise SystemExit("check_models 仅支持 SQLite 数据库")
conn = sqlite3.connect(str(db_path))
cursor = conn.cursor()

# 查询模型及其所属的 provider
cursor.execute('''
    SELECT p.name, m.name, m.display_name, m.is_active, m.remote_identifier
    FROM models m
    JOIN providers p ON m.provider_id = p.id
    ORDER BY p.name, m.name
''')

print("可用的模型:")
print("-" * 80)
for row in cursor.fetchall():
    provider, model_name, display_name, is_active, remote_id = row
    status = "✓" if is_active else "✗"
    remote_info = f" ({remote_id})" if remote_id else ""
    print(f"{status} {provider}/{model_name} - {display_name}{remote_info}")
conn.close()

