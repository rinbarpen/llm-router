import React, { Component, ErrorInfo, ReactNode } from 'react'
import { Layout, Alert, ConfigProvider } from 'antd'
import MonitorDashboard from './components/MonitorDashboard'
import './App.css'

const { Header, Content } = Layout

const AppTheme = {
  token: {
    colorPrimary: '#6366f1',
    colorSuccess: '#10b981',
    colorBgLayout: '#f5f3ff',
    colorTextBase: '#1e1b4b',
    borderRadius: 8,
    fontFamily: 'Plus Jakarta Sans, sans-serif',
  },
  components: {
    Layout: {
      headerBg: '#ffffff',
      headerHeight: 64,
      headerPadding: '0 24px',
    },
    Card: {
      borderRadiusLG: 12,
    },
    Button: {
      borderRadius: 8,
      fontWeight: 500,
    },
  },
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
        <Layout style={{ minHeight: '100vh' }}>
          <Header style={{ background: '#001529', color: '#fff', padding: '0 24px' }}>
            <h1 style={{ color: '#fff', margin: 0, lineHeight: '64px' }}>LLM Router Monitor</h1>
          </Header>
          <Content style={{ padding: '24px', background: '#f0f2f5' }}>
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

    return (
      <ConfigProvider theme={AppTheme}>
        {this.props.children}
      </ConfigProvider>
    )
  }
}

const App: React.FC = () => {
  return (
    <ConfigProvider theme={AppTheme}>
      <ErrorBoundary>
        <Layout style={{ minHeight: '100vh' }}>
          <Header style={{ 
            display: 'flex', 
            alignItems: 'center', 
            borderBottom: '1px solid #e5e7eb',
            position: 'sticky',
            top: 0,
            zIndex: 1000,
            width: '100%'
          }}>
            <h1 style={{ 
              color: '#1e1b4b', 
              margin: 0, 
              fontSize: '20px', 
              fontWeight: 700,
              display: 'flex',
              alignItems: 'center',
              gap: '8px'
            }}>
              <span style={{ 
                background: 'linear-gradient(135deg, #6366f1 0%, #818cf8 100%)',
                color: 'white',
                padding: '4px 8px',
                borderRadius: '6px',
                fontSize: '16px'
              }}>LR</span>
              LLM Router
            </h1>
          </Header>
          <Content style={{ padding: '24px', maxWidth: '1440px', margin: '0 auto', width: '100%' }}>
            <MonitorDashboard />
          </Content>
        </Layout>
      </ErrorBoundary>
    </ConfigProvider>
  )
}

export default App
