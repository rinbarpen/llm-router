/**
 * 监控示例
 * 
 * 演示如何查询调用历史、统计信息和监控数据。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

async function getInvocations(filters = {}) {
    const url = `${BASE_URL}/monitor/invocations`;
    
    const headers = {};
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    const params = new URLSearchParams({
        limit: filters.limit || 100,
        offset: filters.offset || 0,
    });
    
    if (filters.modelName) params.append('model_name', filters.modelName);
    if (filters.providerName) params.append('provider_name', filters.providerName);
    if (filters.status) params.append('status', filters.status);
    if (filters.startTime) params.append('start_time', filters.startTime);
    if (filters.endTime) params.append('end_time', filters.endTime);
    
    try {
        const response = await fetch(`${url}?${params}`, {
            method: 'GET',
            headers: headers,
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        return await response.json();
    } catch (error) {
        console.log(`✗ 获取调用历史失败: ${error.message}`);
        return [];
    }
}

async function getStatistics(timeRange = '24h') {
    const url = `${BASE_URL}/monitor/statistics`;
    
    const headers = {};
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    const params = new URLSearchParams({ time_range: timeRange });
    
    try {
        const response = await fetch(`${url}?${params}`, {
            method: 'GET',
            headers: headers,
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        return await response.json();
    } catch (error) {
        console.log(`✗ 获取统计信息失败: ${error.message}`);
        return null;
    }
}

function printStatistics(stats) {
    if (!stats) {
        console.log('无法获取统计信息');
        return;
    }
    
    const overall = stats.overall || {};
    console.log(`\n总体统计 (${overall.time_range || 'N/A'}):`);
    console.log('-'.repeat(60));
    console.log(`总调用数: ${overall.total_calls || 0}`);
    console.log(`成功调用: ${overall.success_calls || 0}`);
    console.log(`失败调用: ${overall.error_calls || 0}`);
    console.log(`成功率: ${((overall.success_rate || 0) * 100).toFixed(2)}%`);
    console.log(`总 Token: ${overall.total_tokens || 0}`);
    console.log(`平均耗时: ${(overall.avg_duration_ms || 0).toFixed(2)} ms`);
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 监控示例');
    console.log('='.repeat(60));
    console.log();
    
    // 1. 获取最近的调用历史
    console.log('1. 获取最近的调用历史');
    console.log('-'.repeat(60));
    const invocations = await getInvocations({ limit: 10 });
    if (invocations.length > 0) {
        console.log(`找到 ${invocations.length} 条调用记录`);
        invocations.slice(0, 3).forEach(inv => {
            console.log(`  ID: ${inv.id}, 模型: ${inv.provider_name}/${inv.model_name}, 状态: ${inv.status}`);
        });
    } else {
        console.log('没有找到调用记录');
    }
    console.log();
    
    // 2. 获取统计信息
    console.log('2. 获取使用统计（24小时）');
    console.log('-'.repeat(60));
    const stats = await getStatistics('24h');
    printStatistics(stats);
}

if (require.main === module) {
    main();
}

module.exports = {
    getInvocations,
    getStatistics,
    printStatistics
};

