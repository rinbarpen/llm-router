#!/usr/bin/env python3
"""
生成 LLM Router API Key 的工具

用法:
    python scripts/generate_api_key.py                    # 生成一个默认长度的 key
    python scripts/generate_api_key.py --length 64       # 生成指定长度的 key
    python scripts/generate_api_key.py --count 5          # 生成多个 key
    python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY  # 生成并输出为环境变量格式
"""

import argparse
import secrets
import string
import sys
from typing import List


def generate_api_key(length: int = 32) -> str:
    """
    生成安全的 API Key
    
    Args:
        length: Key 的长度（默认 32）
    
    Returns:
        生成的 API Key 字符串
    """
    # 使用字母、数字和部分特殊字符
    # 排除容易混淆的字符：0, O, I, l
    alphabet = string.ascii_letters + string.digits
    # 移除容易混淆的字符
    alphabet = alphabet.replace('0', '').replace('O', '').replace('I', '').replace('l', '')
    # 添加一些安全的特殊字符
    alphabet += '-_'
    
    return ''.join(secrets.choice(alphabet) for _ in range(length))


def generate_api_keys(count: int = 1, length: int = 32) -> List[str]:
    """
    生成多个 API Key
    
    Args:
        count: 要生成的 key 数量
        length: 每个 key 的长度
    
    Returns:
        API Key 列表
    """
    return [generate_api_key(length) for _ in range(count)]


def main():
    parser = argparse.ArgumentParser(
        description='生成 LLM Router API Key',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s                                    # 生成一个默认长度的 key
  %(prog)s --length 64                        # 生成 64 字符长度的 key
  %(prog)s --count 5                         # 生成 5 个 key
  %(prog)s --env LLM_ROUTER_ADMIN_KEY        # 生成并输出为环境变量格式
  %(prog)s --count 3 --env LLM_ROUTER_KEY    # 生成 3 个 key，用逗号分隔
        """
    )
    parser.add_argument(
        '--length',
        type=int,
        default=32,
        help='API Key 的长度（默认: 32）'
    )
    parser.add_argument(
        '--count',
        type=int,
        default=1,
        help='要生成的 key 数量（默认: 1）'
    )
    parser.add_argument(
        '--env',
        type=str,
        help='环境变量名（如果指定，将输出为环境变量格式，如: KEY_NAME=value）'
    )
    parser.add_argument(
        '--prefix',
        type=str,
        default='',
        help='API Key 的前缀（可选，如: sk-）'
    )
    
    args = parser.parse_args()
    
    if args.length < 16:
        print("警告: API Key 长度建议至少 16 个字符，当前长度可能不够安全", file=sys.stderr)
    
    if args.length > 256:
        print("错误: API Key 长度不能超过 256 个字符", file=sys.stderr)
        sys.exit(1)
    
    # 生成 API Key
    keys = generate_api_keys(args.count, args.length)
    
    # 添加前缀
    if args.prefix:
        keys = [f"{args.prefix}{key}" for key in keys]
    
    # 输出结果
    if args.env:
        # 输出为环境变量格式
        if args.count == 1:
            print(f"{args.env}={keys[0]}")
        else:
            # 多个 key 用逗号分隔
            print(f"{args.env}={','.join(keys)}")
    else:
        # 直接输出 key
        for i, key in enumerate(keys, 1):
            if args.count > 1:
                print(f"Key {i}: {key}")
            else:
                print(key)
    
    # 提示信息
    if not args.env:
        print("\n提示: 可以将生成的 key 添加到 .env 文件中，或在 router.toml 中配置", file=sys.stderr)
        print("例如:", file=sys.stderr)
        if args.count == 1:
            print(f"  LLM_ROUTER_ADMIN_KEY={keys[0]}", file=sys.stderr)
        else:
            print(f"  LLM_ROUTER_ADMIN_KEY={','.join(keys)}", file=sys.stderr)


if __name__ == '__main__':
    main()

