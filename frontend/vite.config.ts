import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import fs from 'fs'
import toml from '@iarna/toml'

// 从 router.toml 加载前端配置
function loadFrontendConfigFromToml() {
  const routerTomlPath = path.resolve(__dirname, '../router.toml')
  
  if (!fs.existsSync(routerTomlPath)) {
    return null
  }

  try {
    const tomlContent = fs.readFileSync(routerTomlPath, 'utf-8')
    const config = toml.parse(tomlContent)
    return config.frontend || null
  } catch (error) {
    console.warn(`无法读取 router.toml: ${error.message}`)
    return null
  }
}

export default defineConfig(({ mode }) => {
  // 优先从 router.toml 加载配置
  const frontendConfig = loadFrontendConfigFromToml()
  
  // 加载环境变量（.env 文件中的配置会覆盖 router.toml）
  const env = loadEnv(mode, process.cwd(), '')
  
  // 获取服务器配置（用于构建API URL）
  let serverConfig = null
  const routerTomlPath = path.resolve(__dirname, '../router.toml')
  if (fs.existsSync(routerTomlPath)) {
    try {
      const tomlContent = fs.readFileSync(routerTomlPath, 'utf-8')
      const config = toml.parse(tomlContent)
      serverConfig = config.server || null
    } catch (error) {
      // 忽略错误
    }
  }
  
  // 构建API URL
  let apiUrl = env.VITE_API_URL
  if (!apiUrl && frontendConfig?.api_url) {
    apiUrl = frontendConfig.api_url
  }
  if (!apiUrl && serverConfig) {
    const host = serverConfig.host === '0.0.0.0' ? 'localhost' : (serverConfig.host || 'localhost')
    const port = serverConfig.port || 8000
    apiUrl = `http://${host}:${port}`
  }
  if (!apiUrl) {
    apiUrl = 'http://localhost:8000'
  }
  
  // 获取端口配置（优先级：环境变量 > router.toml > 默认值）
  const frontendPort = parseInt(
    env.VITE_PORT || 
    (frontendConfig?.port?.toString()) || 
    '3000',
    10
  )
  
  // 获取API基础路径
  const apiBaseUrl = env.VITE_API_BASE_URL || frontendConfig?.api_base_url || '/api'
  
  // 输出配置信息（开发模式）
  if (mode === 'development') {
    console.log('前端配置:')
    console.log(`  端口: ${frontendPort}`)
    console.log(`  API URL: ${apiUrl}`)
    console.log(`  API Base URL: ${apiBaseUrl}`)
  }
  
  return {
    plugins: [react()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: frontendPort,
      proxy: {
        '/api': {
          target: apiUrl,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api/, ''),
        },
      },
    },
    // 将环境变量暴露给客户端
    define: {
      'import.meta.env.VITE_API_BASE_URL': JSON.stringify(apiBaseUrl),
    },
  }
})

