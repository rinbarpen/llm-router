import requests
import json

# OpenRouter 免费模型列表（更兼容的）
FREE_MODELS = [
    "openrouter/llama-3.2-3b-instruct",
    "openrouter/mistral-7b-instruct",
    "openrouter/mistral-small-3.1-24b-instruct",
    "openrouter/qwen-2.5-vl-7b-instruct",
    "openrouter/nemotron-nano-9b-v2",
]

BASE_URL = "http://localhost:18000/v1/chat/completions"

def test_model(model_name):
    """测试一个模型"""
    print(f"\n测试模型: {model_name}")
    print("-" * 60)

    # 使用最简单的请求（避免参数兼容性问题）
    payload = {
        "model": model_name,
        "messages": [
            {"role": "user", "content": "Say hello in one word."}
        ],
        "max_tokens": 10
    }

    try:
        response = requests.post(BASE_URL, json=payload, timeout=30)

        if response.status_code == 200:
            data = response.json()
            content = data.get("choices", [{}])[0].get("message", {}).get("content", "")
            print(f"成功！回复: {content}")
            return True
        else:
            print(f"失败！状态码: {response.status_code}")
            try:
                error_data = response.json()
                print(f"错误: {json.dumps(error_data, indent=2, ensure_ascii=False)}")
            except:
                print(f"响应: {response.text[:200]}")
            return False

    except Exception as e:
        print(f"异常: {e}")
        return False

print("=" * 60)
print("测试 OpenRouter 免费模型")
print("=" * 60)

working_models = []
for model in FREE_MODELS:
    if test_model(model):
        working_models.append(model)

print("\n" + "=" * 60)
print("总结")
print("=" * 60)
print(f"成功测试的模型 ({len(working_models)}):")
for model in working_models:
    print(f"  {model}")

