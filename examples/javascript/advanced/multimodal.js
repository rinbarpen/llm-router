/**
 * 多模态输入示例
 * 
 * 演示如何使用图像、音频、视频等多模态输入。
 * 注意：多模态支持取决于具体的 Provider 和模型。
 */

const BASE_URL = process.env.LLM_ROUTER_BASE_URL || 'http://localhost:18000';
const API_KEY = process.env.LLM_ROUTER_API_KEY; // 可选，远程请求时需要

async function invokeWithImageUrl(providerName, modelName, imageUrl, textPrompt) {
    const url = `${BASE_URL}/models/${providerName}/${modelName}/invoke`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    // OpenAI 兼容格式：content 为数组
    const messages = [
        {
            role: 'user',
            content: [
                {
                    type: 'text',
                    text: textPrompt,
                },
                {
                    type: 'image_url',
                    image_url: {
                        url: imageUrl,
                    },
                },
            ],
        },
    ];
    
    const payload = {
        messages: messages,
        parameters: {
            max_tokens: 300,
        },
    };
    
    console.log(`调用模型: ${providerName}/${modelName}`);
    console.log(`图像 URL: ${imageUrl}`);
    console.log(`文本提示: ${textPrompt}`);
    
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
        
        return data;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return null;
    }
}

async function invokeWithMultipleImages(providerName, modelName, imageUrls, textPrompt) {
    const url = `${BASE_URL}/models/${providerName}/${modelName}/invoke`;
    
    const headers = {
        'Content-Type': 'application/json',
    };
    if (API_KEY) {
        headers['Authorization'] = `Bearer ${API_KEY}`;
    }
    
    // 构建包含多张图像的内容
    const content = [{ type: 'text', text: textPrompt }];
    imageUrls.forEach(imageUrl => {
        content.push({
            type: 'image_url',
            image_url: { url: imageUrl },
        });
    });
    
    const messages = [
        {
            role: 'user',
            content: content,
        },
    ];
    
    const payload = {
        messages: messages,
        parameters: {
            max_tokens: 500,
        },
    };
    
    console.log(`调用模型: ${providerName}/${modelName}`);
    console.log(`图像数量: ${imageUrls.length}`);
    console.log(`文本提示: ${textPrompt}`);
    
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
        
        return data;
    } catch (error) {
        console.log(`✗ 请求失败: ${error.message}`);
        return null;
    }
}

// 运行示例
async function main() {
    console.log('='.repeat(60));
    console.log('LLM Router 多模态输入示例');
    console.log('='.repeat(60));
    console.log();
    
    console.log('提示:');
    console.log('1. 确保使用的模型支持视觉功能（检查 config.supports_vision）');
    console.log('2. OpenAI 兼容格式适用于 GPT-4 Vision、Claude 等模型');
    console.log('3. Gemini 格式适用于 Gemini 系列模型');
    console.log('4. 图像格式支持: JPEG, PNG, GIF, WebP');
    console.log('5. 图像大小建议: 小于 20MB');
    console.log();
    console.log('注意: 以下示例需要实际可访问的图像 URL 才能运行');
    console.log('（示例代码已注释，需要实际图像 URL 才能运行）');
}

if (require.main === module) {
    main();
}

module.exports = {
    invokeWithImageUrl,
    invokeWithMultipleImages
};

