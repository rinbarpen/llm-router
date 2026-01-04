/**
 * 流式响应示例
 * 
 * 演示如何处理流式响应（Server-Sent Events）。
 * 注意：当前实现可能不支持所有 Provider 的流式响应。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

async function streamInvoke(prompt, temperature = 0.7, maxTokens = 200) {
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
        stream: true,
    };
    
    console.log(`流式调用模型: ${PROVIDER_NAME}/${MODEL_NAME}`);
    console.log(`提示词: ${prompt}`);
    console.log('\n流式输出:');
    console.log('-'.repeat(60));
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let fullText = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            const chunk = decoder.decode(value, { stream: true });
            const lines = chunk.split('\n');
            
            for (const line of lines) {
                if (!line.trim()) continue;
                
                try {
                    const data = JSON.parse(line);
                    
                    // 提取文本片段
                    let textPiece = '';
                    if (data.delta) {
                        textPiece = data.delta.content || '';
                    } else if (data.text) {
                        textPiece = data.text;
                    } else {
                        textPiece = data.output_text || '';
                    }
                    
                    if (textPiece) {
                        process.stdout.write(textPiece);
                        fullText += textPiece;
                    }
                    
                    // 检查是否完成
                    if (data.is_final || data.finish_reason) {
                        break;
                    }
                } catch (e) {
                    // 忽略 JSON 解析错误
                }
            }
        }
        
        console.log('\n' + '-'.repeat(60));
        console.log('\n✓ 流式调用完成');
        console.log(`完整输出: ${fullText}`);
        
        return fullText;
    } catch (error) {
        console.log(`\n✗ 请求失败: ${error.message}`);
        return null;
    }
}

async function streamOpenAICompatible(messages, temperature = 0.7, maxTokens = 200) {
    const url = `${BASE_URL}/models/${PROVIDER_NAME}/${MODEL_NAME}/v1/chat/completions`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    const payload = {
        messages: messages,
        temperature: temperature,
        max_tokens: maxTokens,
        stream: true,
    };
    
    console.log(`流式调用 OpenAI 兼容 API: ${PROVIDER_NAME}/${MODEL_NAME}`);
    console.log('\n流式输出:');
    console.log('-'.repeat(60));
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let fullText = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            const chunk = decoder.decode(value, { stream: true });
            const lines = chunk.split('\n');
            
            for (const line of lines) {
                if (!line.trim() || !line.startsWith('data: ')) continue;
                
                const dataStr = line.substring(6).trim();
                if (dataStr === '[DONE]') {
                    break;
                }
                
                try {
                    const data = JSON.parse(dataStr);
                    
                    // 提取 delta content
                    if (data.choices && data.choices.length > 0) {
                        const delta = data.choices[0].delta || {};
                        const content = delta.content || '';
                        
                        if (content) {
                            process.stdout.write(content);
                            fullText += content;
                        }
                        
                        // 检查完成原因
                        if (data.choices[0].finish_reason) {
                            break;
                        }
                    }
                } catch (e) {
                    // 忽略 JSON 解析错误
                }
            }
        }
        
        console.log('\n' + '-'.repeat(60));
        console.log('\n✓ 流式调用完成');
        console.log(`完整输出: ${fullText}`);
        
        return fullText;
    } catch (error) {
        console.log(`\n✗ 请求失败: ${error.message}`);
        return null;
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 流式响应示例');
    console.log('='.repeat(60));
    console.log();
    
    // 示例 1: 标准接口流式调用
    console.log('示例 1: 标准接口流式调用');
    console.log('-'.repeat(60));
    await streamInvoke('Write a short story about a robot learning to paint', undefined, 300);
    console.log();
    
    // 示例 2: OpenAI 兼容 API 流式调用
    console.log('示例 2: OpenAI 兼容 API 流式调用');
    console.log('-'.repeat(60));
    const messages = [
        { role: 'user', content: 'Explain quantum computing in simple terms' }
    ];
    await streamOpenAICompatible(messages, undefined, 300);
}

if (require.main === module) {
    main();
}

module.exports = {
    streamInvoke,
    streamOpenAICompatible
};

