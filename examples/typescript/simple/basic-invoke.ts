/**
 * 基础调用示例
 * 
 * 演示如何使用简单的 prompt 调用模型。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY;

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

interface InvokeResponse {
    output_text?: string;
    raw?: {
        usage?: {
            prompt_tokens?: number;
            completion_tokens?: number;
            total_tokens?: number;
        };
    };
}

async function basicInvoke(
    prompt: string,
    temperature: number = 0.7,
    maxTokens: number = 200
): Promise<InvokeResponse | null> {
    const url = `${BASE_URL}/models/${PROVIDER_NAME}/${MODEL_NAME}/invoke`;
    
    const headers: HeadersInit = {
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
        
        const data: InvokeResponse = await response.json();
        
        console.log('✓ 调用成功');
        console.log(`输出: ${data.output_text || 'N/A'}`);
        
        if (data.raw?.usage) {
            const usage = data.raw.usage;
            console.log(
                `Token 使用: ${usage.total_tokens || 0} ` +
                `(prompt: ${usage.prompt_tokens || 0}, ` +
                `completion: ${usage.completion_tokens || 0})`
            );
        }
        
        return data;
    } catch (error) {
        console.log(`✗ 请求失败: ${error instanceof Error ? error.message : String(error)}`);
        return null;
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 基础调用示例');
    console.log('='.repeat(60));
    console.log();
    
    await basicInvoke('What is the capital of France?');
}

if (require.main === module) {
    main();
}

export { basicInvoke };

