#!/usr/bin/env python3
"""
监控示例

演示如何导出和下载监控数据。

注意：后端只提供导出端点，不提供直接的查询 API。前端监控界面通过下载数据库文件，
使用 sql.js 在浏览器中直接查询数据库。本示例演示如何通过导出端点获取监控数据。
"""

import os
from datetime import datetime
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要


def export_data_json(time_range_hours=24, **filters):
    """导出监控数据为 JSON（包含统计信息和调用历史）"""
    url = f"{BASE_URL}/monitor/export/json"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    params = {"time_range_hours": time_range_hours}
    
    # 添加过滤条件（如果导出端点支持）
    if "model_name" in filters:
        params["model_name"] = filters["model_name"]
    if "provider_name" in filters:
        params["provider_name"] = filters["provider_name"]
    if "status" in filters:
        params["status"] = filters["status"]
    if "limit" in filters:
        params["limit"] = min(filters["limit"], 1000)  # 最多1000条
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=30)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 导出数据失败: {e}")
        return None


def export_data_excel(time_range_hours=24, **filters):
    """导出监控数据为 Excel/CSV"""
    url = f"{BASE_URL}/monitor/export/excel"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    params = {"time_range_hours": time_range_hours}
    
    # 添加过滤条件
    if "model_name" in filters:
        params["model_name"] = filters["model_name"]
    if "provider_name" in filters:
        params["provider_name"] = filters["provider_name"]
    if "status" in filters:
        params["status"] = filters["status"]
    if "limit" in filters:
        params["limit"] = min(filters["limit"], 1000)
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=30, stream=True)
        response.raise_for_status()
        
        # 保存文件
        filename = f"llm_router_export_{datetime.now().strftime('%Y%m%d_%H%M%S')}.csv"
        with open(filename, 'wb') as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)
        
        print(f"✓ 数据已导出到: {filename}")
        return filename
    except Exception as e:
        print(f"✗ 导出数据失败: {e}")
        return None


def download_database():
    """下载监控数据库文件（SQLite 格式，可用于直接查询）"""
    url = f"{BASE_URL}/monitor/database"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    try:
        response = requests.get(url, headers=headers, timeout=30, stream=True)
        response.raise_for_status()
        
        # 保存数据库文件
        filename = "llm_router_monitor.db"
        with open(filename, 'wb') as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)
        
        print(f"✓ 数据库已下载到: {filename}")
        print(f"  文件大小: {os.path.getsize(filename) / 1024:.2f} KB")
        print(f"  提示: 可以使用 SQLite 工具或 Python sqlite3 模块查询此数据库")
        return filename
    except Exception as e:
        print(f"✗ 下载数据库失败: {e}")
        return None


def print_invocations(invocations):
    """打印调用历史"""
    if not invocations:
        print("没有调用记录")
        return
    
    print(f"\n调用历史 (共 {len(invocations)} 条):")
    print("-" * 100)
    for inv in invocations[:10]:  # 只显示前10条
        # 处理不同的数据格式（可能是字典或对象）
        if isinstance(inv, dict):
            inv_id = inv.get('id')
            provider = inv.get('provider_name')
            model = inv.get('model_name')
            status = inv.get('status')
            started_at = inv.get('started_at')
            duration = inv.get('duration_ms', 0)
            tokens = inv.get('total_tokens', 0)
            error = inv.get('error_message')
        else:
            inv_id = getattr(inv, 'id', None)
            provider = getattr(inv, 'provider_name', None)
            model = getattr(inv, 'model_name', None)
            status = getattr(inv, 'status', None)
            started_at = getattr(inv, 'started_at', None)
            duration = getattr(inv, 'duration_ms', 0)
            tokens = getattr(inv, 'total_tokens', 0)
            error = getattr(inv, 'error_message', None)
        
        print(f"ID: {inv_id}")
        print(f"  模型: {provider}/{model}")
        print(f"  状态: {status}")
        print(f"  时间: {started_at}")
        if duration:
            print(f"  耗时: {float(duration):.2f} ms")
        print(f"  Tokens: {tokens}")
        if error:
            print(f"  错误: {error}")
        print("-" * 100)


def print_statistics(stats):
    """打印统计信息"""
    if not stats:
        print("无法获取统计信息")
        return
    
    overall = stats.get("overall", {})
    print(f"\n总体统计 ({overall.get('time_range', 'N/A')}):")
    print("-" * 60)
    print(f"总调用数: {overall.get('total_calls', 0)}")
    print(f"成功调用: {overall.get('success_calls', 0)}")
    print(f"失败调用: {overall.get('error_calls', 0)}")
    print(f"成功率: {overall.get('success_rate', 0):.2%}")
    print(f"总 Token: {overall.get('total_tokens', 0)}")
    print(f"平均耗时: {overall.get('avg_duration_ms', 0):.2f} ms")
    
    by_model = stats.get("by_model", [])
    if by_model:
        print(f"\n按模型统计:")
        print("-" * 60)
        for model_stat in by_model[:5]:  # 只显示前5个模型
            print(f"模型: {model_stat.get('provider_name')}/{model_stat.get('model_name')}")
            print(f"  调用数: {model_stat.get('total_calls', 0)}")
            print(f"  成功率: {model_stat.get('success_rate', 0):.2%}")
            print(f"  总 Token: {model_stat.get('total_tokens', 0)}")
            print(f"  平均耗时: {model_stat.get('avg_duration_ms', 0):.2f} ms")
            print("-" * 60)


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 监控示例")
    print("=" * 60)
    print()
    print("注意: 后端只提供导出端点，不提供直接的查询 API。")
    print("前端监控界面通过下载数据库文件，使用 sql.js 在浏览器中直接查询。")
    print("本示例演示如何通过导出端点获取监控数据。")
    print()
    
    # 1. 导出 JSON 数据（包含统计和调用历史）
    print("1. 导出监控数据为 JSON（24小时）")
    print("-" * 60)
    export_data = export_data_json(time_range_hours=24, limit=10)
    if export_data:
        statistics = export_data.get("statistics", {})
        invocations = export_data.get("invocations", [])
        total_invocations = export_data.get("total_invocations", 0)
        
        print(f"✓ 成功导出数据")
        print(f"  总调用数: {total_invocations}")
        print(f"  导出调用数: {len(invocations)}")
        
        # 打印统计信息
        print_statistics(statistics)
        
        # 打印调用历史
        print_invocations(invocations)
    print()
    
    # 2. 按状态过滤导出
    print("2. 导出成功的调用（24小时）")
    print("-" * 60)
    success_export = export_data_json(time_range_hours=24, status="success", limit=10)
    if success_export:
        success_invocations = success_export.get("invocations", [])
        print_invocations(success_invocations)
    print()
    
    # 3. 按模型过滤导出
    print("3. 导出特定模型的调用（24小时）")
    print("-" * 60)
    model_export = export_data_json(
        time_range_hours=24,
        provider_name="openrouter",
        model_name="openrouter-llama-3.3-70b-instruct",
        limit=10
    )
    if model_export:
        model_invocations = model_export.get("invocations", [])
        print_invocations(model_invocations)
    print()
    
    # 4. 导出为 Excel/CSV
    print("4. 导出监控数据为 CSV（24小时）")
    print("-" * 60)
    csv_file = export_data_excel(time_range_hours=24, limit=100)
    if csv_file:
        print(f"✓ CSV 文件已保存: {csv_file}")
    print()
    
    # 5. 下载监控数据库
    print("5. 下载监控数据库")
    print("-" * 60)
    db_file = download_database()
    if db_file:
        print(f"✓ 数据库文件已保存: {db_file}")
        print("  提示: 可以使用以下方式查询数据库:")
        print("  - Python: import sqlite3; conn = sqlite3.connect('llm_router_monitor.db')")
        print("  - 命令行: sqlite3 llm_router_monitor.db")
        print("  - 前端: 使用 sql.js 在浏览器中查询")
    print()
    
    # 6. 显示调用详情（从导出的数据中）
    if export_data:
        invocations = export_data.get("invocations", [])
        if invocations and len(invocations) > 0:
            print("6. 调用详情示例（从导出数据中）")
            print("-" * 60)
            first_inv = invocations[0]
            print(f"调用 ID: {first_inv.get('id')}")
            print(f"模型: {first_inv.get('provider_name')}/{first_inv.get('model_name')}")
            print(f"状态: {first_inv.get('status')}")
            if first_inv.get('request_prompt'):
                print(f"请求: {first_inv.get('request_prompt', 'N/A')[:100]}...")
            if first_inv.get('response_text'):
                print(f"响应: {first_inv.get('response_text', 'N/A')[:100]}...")
            print(f"耗时: {first_inv.get('duration_ms', 0):.2f} ms")
            print(f"Tokens: {first_inv.get('total_tokens', 0)}")
    print()
    
    print("监控功能说明:")
    print("1. JSON 导出: 包含统计信息和调用历史，适合程序处理")
    print("2. CSV 导出: 包含调用历史，适合 Excel 分析")
    print("3. 数据库下载: 完整的 SQLite 数据库，支持复杂查询")
    print("4. 过滤支持: 可按模型、Provider、状态、时间范围过滤")
    print("5. 前端界面: 访问 http://localhost:4022 查看可视化监控界面")

