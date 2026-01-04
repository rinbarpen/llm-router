/**
 * 批量处理示例
 * 
 * 演示如何高效地批量处理多个请求，包括并发处理和结果收集。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

const PROVIDER_NAME = 'openrouter';
const MODEL_NAME = 'openrouter-llama-3.3-70b-instruct';

async function singleInvoke(prompt, options = {}) {
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
    
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(payload),
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        return await response.json();
    } catch (error) {
        console.log(`✗ 请求失败: ${prompt.substring(0, 50)}... - ${error.message}`);
        return null;
    }
}

async function batchSequential(prompts, options = {}) {
    console.log(`顺序处理 ${prompts.length} 个请求...`);
    const results = [];
    
    for (let i = 0; i < prompts.length; i++) {
        const prompt = prompts[i];
        console.log(`处理 ${i + 1}/${prompts.length}: ${prompt.substring(0, 50)}...`);
        const result = await singleInvoke(prompt, options);
        results.push(result);
    }
    
    return results;
}

async function batchConcurrent(prompts, maxConcurrent = 5, options = {}) {
    console.log(`并发处理 ${prompts.length} 个请求 (maxConcurrent=${maxConcurrent})...`);
    const results = [];
    const executing = [];
    
    for (const prompt of prompts) {
        const promise = singleInvoke(prompt, options).then(result => {
            results.push(result);
            console.log(`完成 ${results.length}/${prompts.length}`);
            return result;
        });
        
        executing.push(promise);
        
        if (executing.length >= maxConcurrent) {
            await Promise.race(executing);
            executing.splice(executing.findIndex(p => p === promise), 1);
        }
    }
    
    await Promise.all(executing);
    return results;
}

function processResults(results) {
    const total = results.length;
    const success = results.filter(r => r !== null).length;
    const failed = total - success;
    
    let totalTokens = 0;
    results.forEach(result => {
        if (result && result.raw && result.raw.usage) {
            totalTokens += result.raw.usage.total_tokens || 0;
        }
    });
    
    return {
        total,
        success,
        failed,
        successRate: total > 0 ? success / total : 0,
        totalTokens,
    };
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 批量处理示例');
    console.log('='.repeat(60));
    console.log();
    
    const prompts = [
        'What is Python?',
        'What is JavaScript?',
        'What is Rust?',
        'What is Go?',
        'What is TypeScript?',
    ];
    
    // 示例 1: 顺序处理
    console.log('示例 1: 顺序批量处理');
    console.log('-'.repeat(60));
    const start1 = Date.now();
    const sequentialResults = await batchSequential(prompts, { maxTokens: 100 });
    const sequentialTime = (Date.now() - start1) / 1000;
    const stats1 = processResults(sequentialResults);
    console.log('\n统计:');
    console.log(`  总请求数: ${stats1.total}`);
    console.log(`  成功: ${stats1.success}`);
    console.log(`  失败: ${stats1.failed}`);
    console.log(`  成功率: ${(stats1.successRate * 100).toFixed(2)}%`);
    console.log(`  总 Token: ${stats1.totalTokens}`);
    console.log(`  耗时: ${sequentialTime.toFixed(2)} 秒`);
    console.log();
    
    // 示例 2: 并发处理
    console.log('示例 2: 并发批量处理');
    console.log('-'.repeat(60));
    const start2 = Date.now();
    const concurrentResults = await batchConcurrent(prompts, 3, { maxTokens: 100 });
    const concurrentTime = (Date.now() - start2) / 1000;
    const stats2 = processResults(concurrentResults);
    console.log('\n统计:');
    console.log(`  总请求数: ${stats2.total}`);
    console.log(`  成功: ${stats2.success}`);
    console.log(`  失败: ${stats2.failed}`);
    console.log(`  成功率: ${(stats2.successRate * 100).toFixed(2)}%`);
    console.log(`  总 Token: ${stats2.totalTokens}`);
    console.log(`  耗时: ${concurrentTime.toFixed(2)} 秒`);
    console.log(`  速度提升: ${(sequentialTime / concurrentTime).toFixed(2)}x`);
    console.log();
    
    console.log('提示:');
    console.log('1. 顺序处理简单但慢，适合小批量');
    console.log('2. 并发处理使用 Promise，适合中等批量');
    console.log('3. 注意控制并发数，避免超过 API 限流');
}

if (require.main === module) {
    main();
}

module.exports = {
    singleInvoke,
    batchSequential,
    batchConcurrent,
    processResults
};

