#!/usr/bin/env python3
import sqlite3

conn = sqlite3.connect('llm_router.db')
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

