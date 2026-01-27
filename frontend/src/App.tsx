import React, { Component, ErrorInfo, ReactNode } from 'react'
import { Layout, Alert } from 'antd'
import MonitorDashboard from './components/MonitorDashboard'
import './App.css'

const { Header, Content } = Layout

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

    return this.props.children
  }
}

const App: React.FC = () => {
  return (
    <ErrorBoundary>
      <Layout style={{ minHeight: '100vh' }}>
        <Header style={{ background: '#001529', color: '#fff', padding: '0 24px' }}>
          <h1 style={{ color: '#fff', margin: 0, lineHeight: '64px' }}>LLM Router Monitor</h1>
        </Header>
        <Content style={{ padding: '24px', background: '#f0f2f5' }}>
          <MonitorDashboard />
        </Content>
      </Layout>
    </ErrorBoundary>
  )
}

export default App

