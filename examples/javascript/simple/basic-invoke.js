/**
 * 基础调用示例
 * 
 * 演示如何使用简单的 prompt 调用模型。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

// 使用免费模型作为示例
const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

async function basicInvoke(prompt, temperature = 0.7, maxTokens = 200) {
    const url = `${BASE_URL}/models/${PROVIDER_NAME}/${MODEL_NAME}/invoke`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    const payload = {
        prompt: prompt,
        parameters: {
            temperature: temperature,
            max_tokens: maxTokens,
        },
    };
    
    console.log(`调用模型: ${PROVIDER_NAME}/${MODEL_NAME}`);
    console.log(`提示词: ${prompt}`);
    console.log(`参数: temperature=${temperature}, max_tokens=${maxTokens}`);
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
        console.log(`输出: ${data.output_text || 'N/A'}`);
        
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
    console.log('LLM Router 基础调用示例');
    console.log('='.repeat(60));
    console.log();
    
    // 示例 1: 简单问题
    console.log('示例 1: 简单问题');
    console.log('-'.repeat(60));
    await basicInvoke('What is the capital of France?');
    console.log();
    
    // 示例 2: 中文问题
    console.log('示例 2: 中文问题');
    console.log('-'.repeat(60));
    await basicInvoke('请用一句话解释什么是人工智能', 0.5, 100);
    console.log();
    
    // 示例 3: 创意任务（高温度）
    console.log('示例 3: 创意任务（高温度）');
    console.log('-'.repeat(60));
    await basicInvoke('Write a short haiku about technology', 0.9, 50);
    console.log();
    
    // 示例 4: 编程任务（低温度）
    console.log('示例 4: 编程任务（低温度）');
    console.log('-'.repeat(60));
    await basicInvoke('Write a Python function to calculate factorial', 0.2, 200);
}

if (require.main === module) {
    main();
}

module.exports = { basicInvoke };

