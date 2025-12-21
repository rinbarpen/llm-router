import httpx
import asyncio
import json

async def test_api():
    url = "http://localhost:18000/models/openrouter/grok-4.1-fast/v1/chat/completions"
    payload = {
        "messages": [{"role": "user", "content": "Hello"}],
        "model": "openrouter/grok-4.1-fast"
    }
    
    print(f"Testing URL: {url}")
    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.post(url, json=payload)
            print(f"Status Code: {response.status_code}")
            print(f"Response: {response.text}")
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    asyncio.run(test_api())
