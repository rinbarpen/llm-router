import React from 'react'
import { Layout } from 'antd'
import MonitorDashboard from './components/MonitorDashboard'
import './App.css'

const { Header, Content } = Layout

const App: React.FC = () => {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ background: '#001529', color: '#fff', padding: '0 24px' }}>
        <h1 style={{ color: '#fff', margin: 0, lineHeight: '64px' }}>LLM Router Monitor</h1>
      </Header>
      <Content style={{ padding: '24px', background: '#f0f2f5' }}>
        <MonitorDashboard />
      </Content>
    </Layout>
  )
}

export default App

