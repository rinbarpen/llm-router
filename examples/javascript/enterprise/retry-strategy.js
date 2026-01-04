/**
 * 重试策略示例
 * 
 * 演示不同的重试策略，包括指数退避、固定间隔等。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

function randomBetween(min, max) {
    return Math.random() * (max - min) + min;
}

async function invoke(prompt, options = {}) {
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
    
    const response = await fetch(url, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(payload),
    });
    
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${await response.text()}`);
    }
    
    return await response.json();
}

async function retryWithExponentialBackoff(
    func,
    maxRetries = 3,
    initialDelay = 1000,
    maxDelay = 60000,
    exponentialBase = 2.0,
    jitter = true
) {
    let delay = initialDelay;
    
    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            return await func();
        } catch (error) {
            if (attempt === maxRetries - 1) {
                throw error;
            }
            
            // 计算等待时间
            let waitTime = Math.min(delay, maxDelay);
            
            // 添加抖动（jitter）避免雷群效应
            if (jitter) {
                waitTime = waitTime * (0.5 + Math.random());
            }
            
            console.log(`⚠ 重试 ${attempt + 1}/${maxRetries}，等待 ${(waitTime / 1000).toFixed(2)} 秒...`);
            await sleep(waitTime);
            
            // 指数增长延迟
            delay *= exponentialBase;
        }
    }
    
    throw new Error('所有重试都失败');
}

async function retryWithFixedInterval(func, maxRetries = 3, interval = 2000) {
    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            return await func();
        } catch (error) {
            if (attempt === maxRetries - 1) {
                throw error;
            }
            
            console.log(`⚠ 重试 ${attempt + 1}/${maxRetries}，等待 ${interval / 1000} 秒...`);
            await sleep(interval);
        }
    }
    
    throw new Error('所有重试都失败');
}

function isRetryableError(error) {
    // 网络错误、超时错误可以重试
    if (error.message.includes('fetch') || error.message.includes('network')) {
        return true;
    }
    
    // 5xx 服务器错误可以重试
    if (error.message.includes('500') || error.message.includes('502') || error.message.includes('503')) {
        return true;
    }
    
    // 429 限流错误可以重试
    if (error.message.includes('429')) {
        return true;
    }
    
    // 4xx 客户端错误通常不可重试
    return false;
}

async function retryWithSmartStrategy(func, maxRetries = 3, initialDelay = 1000) {
    let delay = initialDelay;
    
    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            return await func();
        } catch (error) {
            if (!isRetryableError(error)) {
                throw error;
            }
            
            if (attempt === maxRetries - 1) {
                throw error;
            }
            
            // 根据错误类型调整延迟
            if (error.message.includes('429')) {
                // 限流错误，等待更长时间
                delay = 60000;
            } else if (error.message.includes('500') || error.message.includes('502') || error.message.includes('503')) {
                // 服务器错误，指数退避
                delay = Math.min(delay * 2, 60000);
            } else {
                // 其他错误，固定延迟
                delay = 2000;
            }
            
            console.log(`⚠ 重试 ${attempt + 1}/${maxRetries}，等待 ${(delay / 1000).toFixed(2)} 秒...`);
            await sleep(delay);
        }
    }
    
    throw new Error('所有重试都失败');
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 重试策略示例');
    console.log('='.repeat(60));
    console.log();
    
    const prompt = 'What is Python?';
    
    // 示例 1: 指数退避
    console.log('示例 1: 指数退避重试策略');
    console.log('-'.repeat(60));
    try {
        const result = await retryWithExponentialBackoff(
            () => invoke(prompt, { maxTokens: 100 }),
            3,
            1000,
            10000
        );
        console.log(`✓ 调用成功: ${(result.output_text || '').substring(0, 50)}...`);
    } catch (error) {
        console.log(`✗ 调用失败: ${error.message}`);
    }
    console.log();
    
    // 示例 2: 固定间隔
    console.log('示例 2: 固定间隔重试策略');
    console.log('-'.repeat(60));
    try {
        const result = await retryWithFixedInterval(
            () => invoke(prompt, { maxTokens: 100 }),
            3,
            2000
        );
        console.log(`✓ 调用成功: ${(result.output_text || '').substring(0, 50)}...`);
    } catch (error) {
        console.log(`✗ 调用失败: ${error.message}`);
    }
    console.log();
    
    // 示例 3: 智能重试策略
    console.log('示例 3: 智能重试策略（推荐）');
    console.log('-'.repeat(60));
    try {
        const result = await retryWithSmartStrategy(
            () => invoke(prompt, { maxTokens: 100 }),
            3,
            1000
        );
        console.log(`✓ 调用成功: ${(result.output_text || '').substring(0, 50)}...`);
    } catch (error) {
        console.log(`✗ 调用失败: ${error.message}`);
    }
    console.log();
    
    console.log('重试策略选择建议:');
    console.log('1. 指数退避: 适合网络不稳定场景，避免对服务器造成压力');
    console.log('2. 固定间隔: 适合简单的重试场景，实现简单');
    console.log('3. 智能策略: 推荐使用，根据错误类型自动调整策略');
    console.log();
    console.log('最佳实践:');
    console.log('- 区分可重试错误（网络错误、5xx）和不可重试错误（4xx）');
    console.log('- 使用指数退避避免雷群效应');
    console.log('- 添加抖动（jitter）避免同时重试');
    console.log('- 设置最大重试次数和最大延迟时间');
    console.log('- 记录重试日志，便于问题排查');
}

if (require.main === module) {
    main();
}

module.exports = {
    retryWithExponentialBackoff,
    retryWithFixedInterval,
    retryWithSmartStrategy,
    isRetryableError
};

