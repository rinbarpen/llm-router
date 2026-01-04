/**
 * 健康检查示例
 * 
 * 演示如何检查 LLM Router 服务的健康状态。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

interface HealthResponse {
    status: string;
}

async function healthCheck(): Promise<boolean> {
    const url = `${BASE_URL}/health`;
    
    console.log(`检查服务健康状态: ${url}`);
    
    try {
        const headers: HeadersInit = {};
        if (API_KEY) {
            headers['Authorization'] = `Bearer ${API_KEY}`;
        }
        
        const response = await fetch(url, {
            method: 'GET',
            headers: headers,
        });
        
        if (response.ok) {
            const data: HealthResponse = await response.json();
            console.log(`✓ 服务运行正常:`, data);
            return true;
        } else {
            console.log(`✗ 服务异常: ${response.status} - ${await response.text()}`);
            return false;
        }
    } catch (error) {
        console.log(`✗ 请求失败: ${error instanceof Error ? error.message : String(error)}`);
        return false;
    }
}

// 运行示例
if (require.main === module) {
    console.log('='.repeat(60));
    console.log('LLM Router 健康检查示例');
    console.log('='.repeat(60));
    console.log();
    
    healthCheck();
}

export { healthCheck };

