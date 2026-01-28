#!/usr/bin/env python3
"""
创建 GitHub Release 脚本
使用方法: GITHUB_TOKEN=your_token python scripts/create_release.py
"""

import os
import sys
import json
import requests

REPO = "rinbarpen/llm-router"
TAG = "v1.1.0"
TITLE = "v1.1.0"

RELEASE_NOTES = """## v1.1.0 Release

### 主要更新
- 更新项目版本号至 1.1.0
- 明确项目采用 MIT 许可证
- 优化文档和配置说明

### 变更内容
- 更新 README.md，明确 MIT 许可证说明
- 更新 pyproject.toml 版本号为 1.1.0
- 更新 frontend/package.json 版本号为 1.1.0

### 许可证
本项目采用 MIT License 许可证。
Copyright (c) 2025 rinbarpen
"""


def main():
    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        print("错误: 请设置 GITHUB_TOKEN 环境变量")
        print(f"使用方法: GITHUB_TOKEN=your_token {sys.argv[0]}")
        sys.exit(1)

    url = f"https://api.github.com/repos/{REPO}/releases"
    headers = {
        "Authorization": f"token {token}",
        "Accept": "application/vnd.github.v3+json",
    }
    data = {
        "tag_name": TAG,
        "name": TITLE,
        "body": RELEASE_NOTES,
        "draft": False,
        "prerelease": False,
    }

    print(f"正在创建 Release {TAG}...")

    try:
        response = requests.post(url, headers=headers, json=data)
        
        if response.status_code == 201:
            release_data = response.json()
            print(f"✓ Release {TAG} 创建成功!")
            print(f"Release URL: {release_data['html_url']}")
        elif response.status_code == 422:
            error_data = response.json()
            print(f"⚠ Release {TAG} 可能已经存在")
            print(f"错误信息: {error_data.get('message', 'Unknown error')}")
        else:
            print(f"✗ 创建 Release 失败 (HTTP {response.status_code})")
            try:
                error_data = response.json()
                print(f"错误信息: {error_data.get('message', 'Unknown error')}")
            except:
                print(f"响应: {response.text}")
            sys.exit(1)
    except requests.exceptions.RequestException as e:
        print(f"✗ 请求失败: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
