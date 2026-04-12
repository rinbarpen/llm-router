import React, { Component, ErrorInfo, ReactNode } from 'react'
import { Layout, Alert, ConfigProvider, theme as antdTheme } from 'antd'
import type { ThemeConfig } from 'antd'
import MonitorDashboard from './components/MonitorDashboard'
import { useMonitorTheme } from './hooks/useMonitorTheme'
import type { ThemeMode } from './hooks/useMonitorTheme'
import './App.css'

const { Header, Content } = Layout

function getAppTheme(mode: ThemeMode): ThemeConfig {
  const isDark = mode === 'dark'
  return {
    algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: '#14b8a6',
      colorSuccess: '#2f8f6f',
      colorWarning: '#b98235',
      colorBgLayout: isDark ? '#0c1422' : '#eee6d7',
      colorBgContainer: isDark ? '#111d2e' : '#fffaf1',
      colorBorder: isDark ? '#2c3d56' : '#d9cbb4',
      colorTextBase: isDark ? '#e8edf1' : '#2d261d',
      borderRadius: 16,
      fontFamily: '"Avenir Next", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
    },
    components: {
      Layout: {
        headerBg: isDark ? '#0b1220' : '#fff7ea',
        headerHeight: 68,
        headerPadding: '0 20px',
        siderBg: isDark ? '#0b1220' : '#102033',
      },
      Menu: {
        itemBorderRadius: 10,
      },
      Card: {
        borderRadiusLG: 14,
      },
      Button: {
        borderRadius: 10,
      },
    },
  }
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  constructor(props: { children: ReactNode }) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Error caught by boundary:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <Layout className="app-root">
          <Header className="app-main-header">
            <h1 className="app-brand-title">LLM Router Monitor</h1>
          </Header>
          <Content className="app-main-content">
            <Alert
              message="页面加载错误"
              description={this.state.error?.message || '未知错误'}
              type="error"
              showIcon
            />
          </Content>
        </Layout>
      )
    }

    return this.props.children
  }
}

const App: React.FC = () => {
  const { themeMode, toggleTheme } = useMonitorTheme()

  return (
    <ConfigProvider theme={getAppTheme(themeMode)}>
      <ErrorBoundary>
        <Layout className="app-root">
          <Content className="app-main-content">
            <MonitorDashboard themeMode={themeMode} onToggleTheme={toggleTheme} />
          </Content>
        </Layout>
      </ErrorBoundary>
    </ConfigProvider>
  )
}

export default App
