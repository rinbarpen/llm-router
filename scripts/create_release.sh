#!/bin/bash

# 创建 GitHub Release 脚本
# 使用方法: GITHUB_TOKEN=your_token ./scripts/create_release.sh

set -e

REPO="rinbarpen/llm-router"
TAG="v1.1.0"
TITLE="v1.1.0"
RELEASE_NOTES=$(cat <<EOF
## v1.1.0 Release

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
EOF
)

if [ -z "$GITHUB_TOKEN" ]; then
    echo "错误: 请设置 GITHUB_TOKEN 环境变量"
    echo "使用方法: GITHUB_TOKEN=your_token $0"
    exit 1
fi

echo "正在创建 Release $TAG..."

response=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Authorization: token $GITHUB_TOKEN" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/$REPO/releases" \
    -d "{
        \"tag_name\": \"$TAG\",
        \"name\": \"$TITLE\",
        \"body\": $(echo "$RELEASE_NOTES" | jq -Rs .),
        \"draft\": false,
        \"prerelease\": false
    }")

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" -eq 201 ]; then
    echo "✓ Release $TAG 创建成功!"
    echo "$body" | jq -r '.html_url'
elif [ "$http_code" -eq 422 ]; then
    echo "⚠ Release $TAG 可能已经存在"
    echo "$body" | jq -r '.message'
else
    echo "✗ 创建 Release 失败 (HTTP $http_code)"
    echo "$body" | jq -r '.message // .'
    exit 1
fi
