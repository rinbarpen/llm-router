import os
import json
import httpx
import asyncio
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY", "")

async def test_invoke(provider: str, model: str, prompt: str = "Hello"):
    print(f"\n[Testing Invoke] {provider}/{model}...")
    url = f"{BASE_URL}/models/{provider}/{model}/invoke"
    headers = {"Authorization": f"Bearer {API_KEY}"} if API_KEY else {}
    payload = {
        "prompt": prompt,
        "parameters": {"max_tokens": 50}
    }
    
    async with httpx.AsyncClient() as client:
        try:
            response = await client.post(url, json=payload, headers=headers, timeout=30.0)
            print(f"Status: {response.status_code}")
            if response.status_code == 200:
                data = response.json()
                print(f"Output: {data.get('output_text')}")
                print(f"Usage: {data.get('raw', {}).get('usage')}")
            else:
                print(f"Error: {response.text}")
        except Exception as e:
            print(f"Exception: {str(e)}")

async def test_openai_compatible(model_id: str, prompt: str = "Hello"):
    print(f"\n[Testing OpenAI Compatible] {model_id}...")
    url = f"{BASE_URL}/v1/chat/completions"
    headers = {"Authorization": f"Bearer {API_KEY}"} if API_KEY else {}
    payload = {
        "model": model_id,
        "messages": [{"role": "user", "content": prompt}],
        "stream": False
    }
    
    async with httpx.AsyncClient() as client:
        try:
            response = await client.post(url, json=payload, headers=headers, timeout=30.0)
            print(f"Status: {response.status_code}")
            if response.status_code == 200:
                data = response.json()
                print(f"Output: {data['choices'][0]['message']['content']}")
                print(f"Usage: {data.get('usage')}")
            else:
                print(f"Error: {response.text}")
        except Exception as e:
            print(f"Exception: {str(e)}")

async def main():
    # 示例：替换为实际存在的模型进行测试
    # await test_invoke("openai", "gpt-4o")
    # await test_openai_compatible("openai/gpt-4o")
    print("请在脚本中取消注释或修改模型名称以运行特定测试。")

if __name__ == "__main__":
    asyncio.run(main())
