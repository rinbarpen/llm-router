/**
 * 认证流程示例
 * 
 * 演示完整的认证流程：登录、绑定模型、使用 Token、登出
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY;

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

interface LoginResponse {
    token: string;
    expires_in: number;
    message: string;
}

async function login(apiKey?: string): Promise<string | null> {
    const key = apiKey || API_KEY;
    
    if (!key) {
        console.log('✗ 错误: 需要提供 API Key');
        return null;
    }
    
    const url = `${BASE_URL}/auth/login`;
    
    const headers: HeadersInit = {
        'Content-Type': 'application/json',
    };
    
    const payload = { api_key: key };
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const data: LoginResponse = await response.json();
        console.log('✓ 登录成功');
        console.log(`  Token: ${data.token.substring(0, 20)}...`);
        
        return data.token;
    } catch (error) {
        console.log(`✗ 登录失败: ${error instanceof Error ? error.message : String(error)}`);
        return null;
    }
}

async function invokeWithToken(
    token: string,
    prompt: string,
    providerName?: string,
    modelName?: string
): Promise<any> {
    const provider = providerName || PROVIDER_NAME;
    const model = modelName || MODEL_NAME;
    
    const url = `${BASE_URL}/models/${provider}/${model}/invoke`;
    
    const headers: HeadersInit = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
    };
    
    const payload = {
        prompt: prompt,
        parameters: {
            temperature: 0.7,
            max_tokens: 200,
        },
    };
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        return await response.json();
    } catch (error) {
        console.log(`✗ 调用失败: ${error instanceof Error ? error.message : String(error)}`);
        return null;
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 认证流程示例');
    console.log('='.repeat(60));
    console.log();
    
    const token = await login();
    if (token) {
        await invokeWithToken(token, 'What is Python?');
    }
}

if (require.main === module) {
    main();
}

export { login, invokeWithToken };

