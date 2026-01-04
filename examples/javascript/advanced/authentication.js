/**
 * 认证流程示例
 * 
 * 演示完整的认证流程：登录、绑定模型、使用 Token、登出
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 必需，用于登录

// 模型配置
const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

async function login(apiKey = null) {
    const key = apiKey || API_KEY;
    
    if (!key) {
        console.log('✗ 错误: 需要提供 API Key');
        return null;
    }
    
    const url = `${BASE_URL}/auth/login`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    
    const payload = { api_key: key };
    
    console.log(`登录: ${url}`);
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const data = await response.json();
        const token = data.token;
        const expiresIn = data.expires_in || 0;
        
        console.log('✓ 登录成功');
        console.log(`  Token: ${token.substring(0, 20)}...`);
        console.log(`  过期时间: ${expiresIn} 秒 (${Math.floor(expiresIn / 3600)} 小时)`);
        
        return token;
    } catch (error) {
        console.log(`✗ 登录失败: ${error.message}`);
        return null;
    }
}

async function bindModel(token, providerName, modelName) {
    const url = `${BASE_URL}/auth/bind-model`;
    
    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
    };
    
    const payload = {
        provider_name: providerName,
        model_name: modelName,
    };
    
    console.log(`绑定模型: ${providerName}/${modelName}`);
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const data = await response.json();
        console.log(`✓ 模型绑定成功: ${data.message || 'N/A'}`);
        
        return true;
    } catch (error) {
        console.log(`✗ 绑定失败: ${error.message}`);
        return false;
    }
}

async function invokeWithToken(token, prompt, providerName = null, modelName = null) {
    const provider = providerName || PROVIDER_NAME;
    const model = modelName || MODEL_NAME;
    
    const url = `${BASE_URL}/models/${provider}/${model}/invoke`;
    
    const headers = {
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
    
    console.log(`使用 Token 调用模型: ${provider}/${model}`);
    console.log(`提示词: ${prompt}`);
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const data = await response.json();
        console.log('✓ 调用成功');
        console.log(`输出: ${data.output_text || 'N/A'}`);
        
        return data;
    } catch (error) {
        console.log(`✗ 调用失败: ${error.message}`);
        return null;
    }
}

async function invokeOpenAICompatible(token, messages, providerName = null, modelName = null) {
    const provider = providerName || PROVIDER_NAME;
    const model = modelName || MODEL_NAME;
    
    const url = `${BASE_URL}/models/${provider}/${model}/v1/chat/completions`;
    
    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
    };
    
    const payload = {
        messages: messages,
        temperature: 0.7,
        max_tokens: 200,
    };
    
    console.log(`使用 OpenAI 兼容 API 调用: ${provider}/${model}`);
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const data = await response.json();
        console.log('✓ 调用成功');
        
        if (data.choices && data.choices.length > 0) {
            const content = data.choices[0].message.content;
            console.log(`回复: ${content}`);
        }
        
        if (data.usage) {
            const usage = data.usage;
            console.log(`Token 使用: ${usage.total_tokens || 0}`);
        }
        
        return data;
    } catch (error) {
        console.log(`✗ 调用失败: ${error.message}`);
        return null;
    }
}

async function logout(token) {
    const url = `${BASE_URL}/auth/logout`;
    
    const headers = {
        'Authorization': `Bearer ${token}`,
    };
    
    console.log('登出...');
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const data = await response.json();
        console.log(`✓ 登出成功: ${data.message || 'N/A'}`);
        
        return true;
    } catch (error) {
        console.log(`✗ 登出失败: ${error.message}`);
        return false;
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 认证流程示例');
    console.log('='.repeat(60));
    console.log();
    
    if (!API_KEY) {
        console.log('⚠ 警告: 未设置 LLM_ROUTER_API_KEY 环境变量');
        console.log('本机请求（localhost）可以免认证，但无法演示完整认证流程');
        console.log();
    }
    
    // 1. 登录
    const token = await login();
    if (!token) {
        console.log('\n无法继续，请检查 API Key 配置');
        process.exit(1);
    }
    
    console.log();
    
    // 2. 绑定模型
    await bindModel(token, PROVIDER_NAME, MODEL_NAME);
    console.log();
    
    // 3. 使用 Token 调用模型
    console.log('示例 1: 使用标准接口调用');
    console.log('-'.repeat(60));
    await invokeWithToken(token, 'What is Python?');
    console.log();
    
    // 4. 使用 OpenAI 兼容 API
    console.log('示例 2: 使用 OpenAI 兼容 API');
    console.log('-'.repeat(60));
    const messages = [
        { role: 'user', content: 'Hello! How are you?' }
    ];
    await invokeOpenAICompatible(token, messages);
    console.log();
    
    // 5. 登出
    await logout(token);
}

if (require.main === module) {
    main();
}

module.exports = {
    login,
    bindModel,
    invokeWithToken,
    invokeOpenAICompatible,
    logout
};

