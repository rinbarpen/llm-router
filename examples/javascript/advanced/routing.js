/**
 * 智能路由示例
 * 
 * 演示如何使用智能路由功能，根据标签和 Provider 类型自动选择最佳模型。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

async function routeByTags(tags, providerTypes = null, prompt = null, messages = null) {
    const url = `${BASE_URL}/route/invoke`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    const query = {
        tags: Array.isArray(tags) ? tags : [tags],
    };
    
    if (providerTypes) {
        query.provider_types = Array.isArray(providerTypes) 
            ? providerTypes 
            : [providerTypes];
    }
    
    const requestPayload = {};
    if (prompt) {
        requestPayload.prompt = prompt;
    } else if (messages) {
        requestPayload.messages = messages;
    } else {
        console.log('✗ 错误: 需要提供 prompt 或 messages');
        return null;
    }
    
    requestPayload.parameters = {
        temperature: 0.7,
        max_tokens: 200,
    };
    
    const payload = {
        query: query,
        request: requestPayload,
    };
    
    console.log('智能路由请求');
    console.log(`  标签: ${tags}`);
    if (providerTypes) {
        console.log(`  Provider 类型: ${providerTypes}`);
    }
    const queryText = prompt || (messages && messages[0]?.content) || '';
    console.log(`  提示词: ${queryText.substring(0, 50)}...`);
    
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
        
        console.log('✓ 路由成功');
        console.log(`输出: ${data.output_text || 'N/A'}`);
        
        if (data.raw && data.raw.model) {
            console.log(`使用的模型: ${data.raw.model}`);
        }
        
        return data;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return null;
    }
}

async function routeFreeFast(prompt = null, messages = null) {
    return routeByTags(['free', 'fast'], null, prompt, messages);
}

async function routeChinese(prompt = null, messages = null) {
    return routeByTags(['chinese'], null, prompt, messages);
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 智能路由示例');
    console.log('='.repeat(60));
    console.log();
    
    // 示例 1: 根据标签路由
    console.log('示例 1: 路由到免费快速模型');
    console.log('-'.repeat(60));
    await routeFreeFast('What is 2+2?');
    console.log();
    
    // 示例 2: 路由到中文模型
    console.log('示例 2: 路由到中文模型');
    console.log('-'.repeat(60));
    await routeChinese('请用一句话解释什么是人工智能');
    console.log();
    
    // 示例 3: 根据 Provider 类型路由
    console.log('示例 3: 路由到 OpenRouter 模型');
    console.log('-'.repeat(60));
    await routeByTags([], ['openrouter'], 'Write a haiku about nature');
    console.log();
    
    // 示例 4: 组合条件路由
    console.log('示例 4: 组合条件路由（免费 + OpenRouter）');
    console.log('-'.repeat(60));
    await routeByTags(['free'], ['openrouter'], 'Explain quantum computing in simple terms');
    console.log();
    
    // 示例 5: 使用 messages 格式
    console.log('示例 5: 使用 messages 格式路由');
    console.log('-'.repeat(60));
    const messages = [
        { role: 'user', content: 'Hello, how are you?' }
    ];
    await routeByTags(['chat', 'general'], null, null, messages);
}

if (require.main === module) {
    main();
}

module.exports = {
    routeByTags,
    routeFreeFast,
    routeChinese
};

