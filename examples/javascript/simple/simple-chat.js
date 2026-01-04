/**
 * 简单对话示例
 * 
 * 演示如何使用 messages 格式进行多轮对话。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

// 使用免费模型作为示例
const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

async function simpleChat(messages, temperature = 0.7, maxTokens = 300) {
    const url = `${BASE_URL}/models/${PROVIDER_NAME}/${MODEL_NAME}/invoke`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    const payload = {
        messages: messages,
        parameters: {
            temperature: temperature,
            max_tokens: maxTokens,
        },
    };
    
    console.log(`调用模型: ${PROVIDER_NAME}/${MODEL_NAME}`);
    console.log('对话历史:');
    messages.forEach(msg => {
        const content = typeof msg.content === 'string' 
            ? msg.content.substring(0, 50) 
            : JSON.stringify(msg.content).substring(0, 50);
        console.log(`  ${msg.role}: ${content}...`);
    });
    console.log();
    
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
        console.log(`回复: ${data.output_text || 'N/A'}`);
        
        // 显示使用统计
        if (data.raw && data.raw.usage) {
            const usage = data.raw.usage;
            console.log(
                `Token 使用: ${usage.total_tokens || 0} ` +
                `(prompt: ${usage.prompt_tokens || 0}, ` +
                `completion: ${usage.completion_tokens || 0})`
            );
        }
        
        return data;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return null;
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 简单对话示例');
    console.log('='.repeat(60));
    console.log();
    
    // 示例 1: 单轮对话
    console.log('示例 1: 单轮对话');
    console.log('-'.repeat(60));
    const messages1 = [
        { role: 'user', content: 'Explain quantum computing in simple terms' }
    ];
    await simpleChat(messages1);
    console.log();
    
    // 示例 2: 多轮对话
    console.log('示例 2: 多轮对话');
    console.log('-'.repeat(60));
    const messages2 = [
        { role: 'user', content: 'Hello, how are you?' },
        { role: 'assistant', content: "I'm doing well, thank you! How can I help you today?" },
        { role: 'user', content: 'Can you explain machine learning?' }
    ];
    await simpleChat(messages2);
    console.log();
    
    // 示例 3: 带系统提示的对话
    console.log('示例 3: 带系统提示的对话');
    console.log('-'.repeat(60));
    const messages3 = [
        { role: 'system', content: '你是一个专业的AI助手，擅长用中文回答问题，回答要简洁明了。' },
        { role: 'user', content: '请用Python写一个快速排序算法' }
    ];
    await simpleChat(messages3, 0.3, 500);
    console.log();
    
    // 示例 4: 持续对话
    console.log('示例 4: 持续对话');
    console.log('-'.repeat(60));
    let conversation = [
        { role: 'user', content: 'What is Python?' }
    ];
    
    // 第一轮
    const response1 = await simpleChat(conversation, undefined, 200);
    if (response1 && response1.output_text) {
        conversation.push({ role: 'assistant', content: response1.output_text });
        conversation.push({ role: 'user', content: 'Can you give me an example?' });
        
        // 第二轮
        console.log('\n继续对话...');
        await simpleChat(conversation, undefined, 200);
    }
}

if (require.main === module) {
    main();
}

module.exports = { simpleChat };

