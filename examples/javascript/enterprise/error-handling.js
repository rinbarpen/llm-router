/**
 * 错误处理示例
 * 
 * 演示如何处理各种错误情况，包括网络错误、API 错误、限流等。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function invokeWithErrorHandling(prompt, maxRetries = 3, options = {}) {
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
            temperature: options.temperature || 0.7,
            max_tokens: options.maxTokens || 200,
        },
    };
    
    let lastError = null;
    
    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            const response = await fetch(url, {
                method: 'POST',
                headers: headers,
                body: JSON.stringify(payload),
            });
            
            // 处理不同的 HTTP 状态码
            if (response.ok) {
                return await response.json();
            }
            
            const status = response.status;
            const text = await response.text();
            
            if (status === 401) {
                throw new Error('认证失败: 无效的 API Key 或 Session Token');
            } else if (status === 403) {
                throw new Error('权限不足: API Key 没有访问该模型的权限');
            } else if (status === 404) {
                throw new Error(`模型未找到: ${PROVIDER_NAME}/${MODEL_NAME}`);
            } else if (status === 429) {
                // 限流错误，需要等待
                const retryAfter = parseInt(response.headers.get('Retry-After') || '60');
                if (attempt < maxRetries - 1) {
                    console.log(`⚠ 请求过于频繁，等待 ${retryAfter} 秒后重试...`);
                    await sleep(retryAfter * 1000);
                    continue;
                } else {
                    throw new Error(`请求过于频繁，请在 ${retryAfter} 秒后重试`);
                }
            } else if (status >= 500) {
                // 服务器错误，可以重试
                if (attempt < maxRetries - 1) {
                    const waitTime = Math.pow(2, attempt) * 1000; // 指数退避
                    console.log(`⚠ 服务器错误 (${status})，${waitTime / 1000} 秒后重试 (${attempt + 1}/${maxRetries})...`);
                    await sleep(waitTime);
                    continue;
                } else {
                    throw new Error(`服务器错误 (${status}): ${text}`);
                }
            } else {
                throw new Error(`请求失败 (${status}): ${text}`);
            }
        } catch (error) {
            lastError = error;
            
            // 网络错误，可以重试
            if (error.message.includes('fetch') || error.message.includes('network')) {
                if (attempt < maxRetries - 1) {
                    const waitTime = Math.pow(2, attempt) * 1000;
                    console.log(`⚠ 网络错误: ${error.message}，${waitTime / 1000} 秒后重试 (${attempt + 1}/${maxRetries})...`);
                    await sleep(waitTime);
                    continue;
                }
            }
            
            // 认证错误、模型未找到等不应该重试
            if (error.message.includes('认证') || error.message.includes('模型未找到')) {
                throw error;
            }
            
            // 其他错误，最后一次尝试时抛出
            if (attempt === maxRetries - 1) {
                throw error;
            }
        }
    }
    
    // 如果所有重试都失败
    if (lastError) {
        throw new Error(`请求失败，已重试 ${maxRetries} 次: ${lastError.message}`);
    }
}

async function safeInvoke(prompt, options = {}) {
    try {
        const result = await invokeWithErrorHandling(prompt, 3, options);
        return { success: true, data: result };
    } catch (error) {
        let errorType = '未知错误';
        if (error.message.includes('认证')) {
            errorType = '认证错误';
        } else if (error.message.includes('限流') || error.message.includes('频繁')) {
            errorType = '限流错误';
        } else if (error.message.includes('模型未找到')) {
            errorType = '模型未找到';
        } else if (error.message.includes('服务器错误')) {
            errorType = 'API 错误';
        }
        
        return {
            success: false,
            error: errorType,
            message: error.message,
        };
    }
}

async function checkServiceHealth() {
    const url = `${BASE_URL}/health`;
    
    try {
        const response = await fetch(url, { timeout: 5000 });
        if (response.ok) {
            return { healthy: true, message: '服务正常' };
        } else {
            return { healthy: false, message: `服务异常: ${response.status}` };
        }
    } catch (error) {
        return { healthy: false, message: `无法连接到服务: ${error.message}` };
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 错误处理示例');
    console.log('='.repeat(60));
    console.log();
    
    // 1. 检查服务健康
    console.log('1. 检查服务健康状态');
    console.log('-'.repeat(60));
    const health = await checkServiceHealth();
    console.log(`${health.healthy ? '✓' : '✗'} ${health.message}`);
    console.log();
    
    if (!health.healthy) {
        console.log('⚠ 服务不可用，无法继续演示');
        process.exit(1);
    }
    
    // 2. 正常调用
    console.log('2. 正常调用（带错误处理）');
    console.log('-'.repeat(60));
    const result = await safeInvoke('What is Python?', { maxTokens: 100 });
    if (result.success) {
        console.log('✓ 调用成功');
        console.log(`输出: ${result.data.output_text || 'N/A'}`);
    } else {
        console.log(`✗ 调用失败: ${result.error} - ${result.message}`);
    }
    console.log();
    
    // 3. 批量调用（带错误处理）
    console.log('3. 批量调用（带错误处理）');
    console.log('-'.repeat(60));
    const prompts = [
        'What is Python?',
        'What is JavaScript?',
        'Invalid prompt that might fail',
    ];
    
    const results = [];
    for (const prompt of prompts) {
        const result = await safeInvoke(prompt, { maxTokens: 50 });
        results.push(result);
        if (result.success) {
            console.log(`✓ ${prompt.substring(0, 30)}... - 成功`);
        } else {
            console.log(`✗ ${prompt.substring(0, 30)}... - 失败: ${result.error}`);
        }
    }
    
    const successCount = results.filter(r => r.success).length;
    console.log(`\n统计: ${successCount}/${results.length} 成功`);
    console.log();
    
    console.log('错误处理最佳实践:');
    console.log('1. 总是检查 HTTP 状态码');
    console.log('2. 区分可重试错误（网络错误、5xx）和不可重试错误（4xx）');
    console.log('3. 实现指数退避重试策略');
    console.log('4. 处理限流错误（429），等待 Retry-After 时间');
    console.log('5. 记录错误日志，便于调试和监控');
    console.log('6. 提供用户友好的错误消息');
}

if (require.main === module) {
    main();
}

module.exports = {
    invokeWithErrorHandling,
    safeInvoke,
    checkServiceHealth
};

