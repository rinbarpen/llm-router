/**
 * OpenAI 兼容 API 示例
 * 
 * 演示如何使用 OpenAI 兼容的 API 接口，可以无缝替换 OpenAI SDK。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

async function openaiChatCompletions(messages, model = null, options = {}) {
    // 如果指定了 model，使用它；否则使用默认模型
    const modelPath = model || `${PROVIDER_NAME}/${MODEL_NAME}`;
    
    // 解析 provider 和 model
    let provider, modelName;
    if (modelPath.includes('/')) {
        [provider, modelName] = modelPath.split('/', 2);
    } else {
        provider = PROVIDER_NAME;
        modelName = modelPath;
    }
    
    const url = `${BASE_URL}/models/${provider}/${modelName}/v1/chat/completions`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    // 构建 OpenAI 兼容的请求体
    const payload = {
        messages: messages,
        model: modelPath, // 可选，用于覆盖远程模型标识符
        ...options,
    };
    
    console.log(`调用 OpenAI 兼容 API: ${url}`);
    console.log(`模型: ${modelPath}`);
    console.log(`消息数量: ${messages.length}`);
    
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
        
        // 解析 OpenAI 格式的响应
        if (data.choices && data.choices.length > 0) {
            const choice = data.choices[0];
            const message = choice.message || {};
            const content = message.content || '';
            const finishReason = choice.finish_reason || '';
            
            console.log(`回复: ${content}`);
            console.log(`完成原因: ${finishReason}`);
        }
        
        // 显示使用统计
        if (data.usage) {
            const usage = data.usage;
            console.log('Token 使用:');
            console.log(`  Prompt: ${usage.prompt_tokens || 0}`);
            console.log(`  Completion: ${usage.completion_tokens || 0}`);
            console.log(`  总计: ${usage.total_tokens || 0}`);
        }
        
        return data;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return null;
    }
}

async function openaiChatWithSystemPrompt(systemPrompt, userPrompt, options = {}) {
    const messages = [
        { role: 'system', content: systemPrompt },
        { role: 'user', content: userPrompt }
    ];
    return openaiChatCompletions(messages, null, options);
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router OpenAI 兼容 API 示例');
    console.log('='.repeat(60));
    console.log();
    
    // 示例 1: 简单对话
    console.log('示例 1: 简单对话');
    console.log('-'.repeat(60));
    const messages1 = [
        { role: 'user', content: 'Hello! How are you?' }
    ];
    await openaiChatCompletions(messages1, null, { temperature: 0.7, max_tokens: 100 });
    console.log();
    
    // 示例 2: 带系统提示
    console.log('示例 2: 带系统提示');
    console.log('-'.repeat(60));
    await openaiChatWithSystemPrompt(
        '你是一个专业的 Python 编程助手，擅长编写清晰、高效的代码。',
        '请写一个快速排序算法的 Python 实现',
        { temperature: 0.3, max_tokens: 500 }
    );
    console.log();
    
    // 示例 3: 多轮对话
    console.log('示例 3: 多轮对话');
    console.log('-'.repeat(60));
    const conversation = [
        { role: 'user', content: 'What is Python?' },
        { role: 'assistant', content: 'Python is a high-level programming language known for its simplicity and readability.' },
        { role: 'user', content: 'Can you give me an example?' }
    ];
    await openaiChatCompletions(conversation, null, { temperature: 0.7, max_tokens: 200 });
    console.log();
    
    console.log('提示:');
    console.log('1. OpenAI 兼容 API 可以无缝替换 OpenAI SDK');
    console.log('2. 使用 /models/{provider}/{model}/v1/chat/completions 端点');
    console.log('3. 如果已绑定模型到 Session，可以不指定 model 参数');
    console.log('4. 支持所有 OpenAI API 的标准参数');
}

if (require.main === module) {
    main();
}

module.exports = {
    openaiChatCompletions,
    openaiChatWithSystemPrompt
};

