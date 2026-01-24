import json
import re
from pathlib import Path

ROUTER_TOML = Path("router.toml")
TEST_RESULTS = Path("test_results.json")
BACKUP_FILE = Path("router.toml.backup")

# 加载无效模型列表
with TEST_RESULTS.open("r", encoding="utf-8") as f:
    results = json.load(f)
invalid_models = set(results.get("invalid_models", []))

print(f"找到 {len(invalid_models)} 个无效模型需要移除")

# 读取原始文件
with ROUTER_TOML.open("r", encoding="utf-8") as f:
    original_content = f.read()

# 创建备份
with BACKUP_FILE.open("w", encoding="utf-8") as f:
    f.write(original_content)
print(f"备份已创建: {BACKUP_FILE}")

# 移除无效模型块
lines = original_content.split('\n')
result_lines = []
i = 0

while i < len(lines):
    line = lines[i]
    if line.strip() == "[[models]]":
        model_block_lines = [line]
        j = i + 1
        model_name = None
        should_remove = False
        
        while j < len(lines):
            next_line = lines[j]
            if next_line.strip() == "[[models]]":
                break
            name_match = re.match(r'^\s*name\s*=\s*"([^"]+)"', next_line)
            if name_match:
                model_name = name_match.group(1)
                if model_name in invalid_models:
                    should_remove = True
            model_block_lines.append(next_line)
            j += 1
        
        if should_remove:
            print(f"  移除模型: {model_name}")
            i = j
        else:
            result_lines.extend(model_block_lines)
            i = j
    else:
        result_lines.append(line)
        i += 1

# 写入更新后的文件
new_content = '\n'.join(result_lines)
with ROUTER_TOML.open("w", encoding="utf-8") as f:
    f.write(new_content)

print(f"\n完成！已移除 {len(invalid_models)} 个无效模型")
