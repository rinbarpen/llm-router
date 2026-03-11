#!/usr/bin/env node

/**
 * 从 router.toml 读取前端配置并生成 .env 文件
 */

const fs = require('fs')
const path = require('path')
const toml = require('@iarna/toml')

const routerTomlPath = path.resolve(__dirname, '../../router.toml')
const envPath = path.resolve(__dirname, '../.env')

// 默认配置
const defaultConfig = {
  port: 3000,
  api_url: 'http://localhost:8000',
  api_base_url: '/api',
}

function loadFrontendConfig() {
  if (!fs.existsSync(routerTomlPath)) {
    console.warn(`配置文件 ${routerTomlPath} 不存在，使用默认配置`)
    return defaultConfig
  }

  try {
    const tomlContent = fs.readFileSync(routerTomlPath, 'utf-8')
    const config = toml.parse(tomlContent)

    const frontendConfig = config.frontend || {}
    const serverConfig = config.server || {}

    // 构建API URL（如果前端配置中没有指定，则根据服务器配置构建）
    let apiUrl = frontendConfig.api_url
    if (!apiUrl && serverConfig.host && serverConfig.port) {
      const host = serverConfig.host === '0.0.0.0' ? 'localhost' : serverConfig.host
      apiUrl = `http://${host}:${serverConfig.port}`
    }
    if (!apiUrl) {
      apiUrl = defaultConfig.api_url
    }

    return {
      port: frontendConfig.port || defaultConfig.port,
      api_url: apiUrl,
      api_base_url: frontendConfig.api_base_url || defaultConfig.api_base_url,
    }
  } catch (error) {
    console.error(`读取配置文件失败: ${error.message}`)
    console.warn('使用默认配置')
    return defaultConfig
  }
}

function generateEnvFile(config) {
  const envContent = `# Frontend Development Server Port
VITE_PORT=${config.port}

# Backend API Server URL (used for proxy in development)
VITE_API_URL=${config.api_url}

# Production API URL (used when building for production)
VITE_API_BASE_URL=${config.api_base_url}
`

  fs.writeFileSync(envPath, envContent, 'utf-8')
  console.log(`已生成 .env 文件: ${envPath}`)
  console.log(`配置内容:`)
  console.log(`  VITE_PORT=${config.port}`)
  console.log(`  VITE_API_URL=${config.api_url}`)
  console.log(`  VITE_API_BASE_URL=${config.api_base_url}`)
}

// 主函数
function main() {
  const config = loadFrontendConfig()
  generateEnvFile(config)
}

if (require.main === module) {
  main()
}

module.exports = { loadFrontendConfig, generateEnvFile }

