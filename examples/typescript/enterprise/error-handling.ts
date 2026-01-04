/**
 * 错误处理示例
 * 
 * 演示如何处理各种错误情况。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY;

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

interface InvokeResult {
    success: boolean;
    data?: any;
    error?: string;
    message?: string;
}

function sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function invokeWithErrorHandling(
    prompt: string,
    maxRetries: number = 3,
    options: { temperature?: number; maxTokens?: number } = {}
): Promise<any> {
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
            temperature: options.temperature || 0.7,
            max_tokens: options.maxTokens || 200,
        },
    };
    
    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            const response = await fetch(url, {
                method: 'POST',
                headers: headers,
                body: JSON.stringify(payload),
            });
            
            if (response.ok) {
                return await response.json();
            }
            
            const status = response.status;
            if (status >= 500 && attempt < maxRetries - 1) {
                const waitTime = Math.pow(2, attempt) * 1000;
                console.log(`⚠ 服务器错误，${waitTime / 1000} 秒后重试 (${attempt + 1}/${maxRetries})...`);
                await sleep(waitTime);
                continue;
            }
            
            throw new Error(`HTTP ${status}: ${await response.text()}`);
        } catch (error) {
            if (attempt === maxRetries - 1) {
                throw error;
            }
        }
    }
    
    throw new Error('所有重试都失败');
}

async function safeInvoke(prompt: string, options = {}): Promise<InvokeResult> {
    try {
        const result = await invokeWithErrorHandling(prompt, 3, options);
        return { success: true, data: result };
    } catch (error) {
        return {
            success: false,
            error: 'API 错误',
            message: error instanceof Error ? error.message : String(error),
        };
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 错误处理示例');
    console.log('='.repeat(60));
    console.log();
    
    const result = await safeInvoke('What is Python?', { maxTokens: 100 });
    if (result.success) {
        console.log('✓ 调用成功');
    } else {
        console.log(`✗ 调用失败: ${result.error} - ${result.message}`);
    }
}

if (require.main === module) {
    main();
}

export { invokeWithErrorHandling, safeInvoke };

