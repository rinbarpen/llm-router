/**
 * 获取模型列表示例
 * 
 * 演示如何获取可用的模型列表，支持按标签、Provider 类型等过滤。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

async function listAllModels() {
    const url = `${BASE_URL}/models`;
    
    const headers = {};
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    console.log(`获取所有模型: ${url}`);
    
    try {
        const response = await fetch(url, {
            method: 'GET',
            headers: headers,
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const models = await response.json();
        console.log(`✓ 找到 ${models.length} 个模型`);
        return models;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return [];
    }
}

async function listModelsByTags(tags) {
    const url = `${BASE_URL}/models`;
    
    // 支持多个标签，用逗号分隔
    const tagsStr = Array.isArray(tags) ? tags.join(',') : tags;
    const params = new URLSearchParams({ tags: tagsStr });
    
    const headers = {};
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    console.log(`按标签过滤模型 (tags=${tagsStr}): ${url}`);
    
    try {
        const response = await fetch(`${url}?${params}`, {
            method: 'GET',
            headers: headers,
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const models = await response.json();
        console.log(`✓ 找到 ${models.length} 个匹配的模型`);
        return models;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return [];
    }
}

async function listModelsByProvider(providerTypes) {
    const url = `${BASE_URL}/models`;
    
    const providerTypesStr = Array.isArray(providerTypes) 
        ? providerTypes.join(',') 
        : providerTypes;
    const params = new URLSearchParams({ provider_types: providerTypesStr });
    
    const headers = {};
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    console.log(`按 Provider 类型过滤模型 (provider_types=${providerTypesStr}): ${url}`);
    
    try {
        const response = await fetch(`${url}?${params}`, {
            method: 'GET',
            headers: headers,
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }
        
        const models = await response.json();
        console.log(`✓ 找到 ${models.length} 个匹配的模型`);
        return models;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return [];
    }
}

function printModelInfo(models) {
    if (!models || models.length === 0) {
        console.log('没有找到模型');
        return;
    }
    
    console.log('\n模型列表:');
    console.log('-'.repeat(80));
    models.slice(0, 3).forEach(model => {
        console.log(`名称: ${model.provider_name}/${model.name}`);
        console.log(`显示名: ${model.display_name || 'N/A'}`);
        console.log(`标签: ${(model.tags || []).join(', ')}`);
        console.log(`状态: ${model.is_active ? '激活' : '未激活'}`);
        console.log('-'.repeat(80));
    });
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 获取模型列表示例');
    console.log('='.repeat(60));
    console.log();
    
    // 1. 获取所有模型
    const allModels = await listAllModels();
    console.log();
    
    // 2. 按标签过滤（免费模型）
    const freeModels = await listModelsByTags('free');
    printModelInfo(freeModels);
    console.log();
    
    // 3. 按标签过滤（中文模型）
    const chineseModels = await listModelsByTags('chinese');
    printModelInfo(chineseModels);
    console.log();
    
    // 4. 按 Provider 类型过滤
    const openrouterModels = await listModelsByProvider('openrouter');
    printModelInfo(openrouterModels);
    console.log();
    
    // 5. 组合过滤
    console.log('组合过滤: tags=free, provider_types=openrouter');
    const url = `${BASE_URL}/models`;
    const params = new URLSearchParams({
        tags: 'free',
        provider_types: 'openrouter'
    });
    const headers = {};
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    try {
        const response = await fetch(`${url}?${params}`, {
            method: 'GET',
            headers: headers,
        });
        if (response.ok) {
            const models = await response.json();
            console.log(`✓ 找到 ${models.length} 个匹配的模型`);
            printModelInfo(models);
        }
    } catch (error) {
        console.log(`✗ 发生错误: ${error.message}`);
    }
}

if (require.main === module) {
    main();
}

module.exports = {
    listAllModels,
    listModelsByTags,
    listModelsByProvider,
    printModelInfo
};

