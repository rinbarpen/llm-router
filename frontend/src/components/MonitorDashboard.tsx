import React, { useState, useEffect, ReactNode } from 'react'
import { Layout, Menu, Button, Space, Dropdown, message, ConfigProvider, theme } from 'antd'
import { 
  DashboardOutlined, 
  HistoryOutlined, 
  SettingOutlined, 
  UserOutlined,
  DownloadOutlined,
  FileTextOutlined,
  FileExcelOutlined,
  MenuUnfoldOutlined,
  MenuFoldOutlined,
  GithubOutlined
} from '@ant-design/icons'
import ActivityDashboard from './ActivityDashboard'
import InvocationList from './InvocationList'
import ModelManagement from './ModelManagement'
import LoginRecordList from './LoginRecordList'
import { monitorApi } from '../services/api'

const { Sider, Content, Header } = Layout

const MonitorDashboard: React.FC = () => {
  const [collapsed, setCollapsed] = useState(false)
  const [activeKey, setActiveKey] = useState('activity')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    if (params.get('oauth') === 'success') {
      const provider = params.get('provider')
      message.success(provider ? `已通过 OAuth 绑定 Provider: ${provider}` : 'OAuth 绑定成功')
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [])

  const handleExportJSON = async () => {
    setLoading(true)
    try {
      const blob = await monitorApi.exportJSON(24)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      link.download = `llm_router_export_${timestamp}.json`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('JSON导出成功')
    } catch (error) {
      console.error('Failed to export JSON:', error)
      message.error('JSON导出失败')
    } finally {
      setLoading(false)
    }
  }

  const handleExportExcel = async () => {
    setLoading(true)
    try {
      const blob = await monitorApi.exportExcel(24)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      link.download = `llm_router_export_${timestamp}.csv`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('CSV导出成功')
    } catch (error) {
      console.error('Failed to export CSV:', error)
      message.error('CSV导出失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDownloadDatabase = async () => {
    setLoading(true)
    try {
      const blob = await monitorApi.downloadDatabase()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = 'llm_datas.db'
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('数据库文件下载成功')
    } catch (error) {
      console.error('Failed to download database:', error)
      message.error('数据库文件下载失败')
    } finally {
      setLoading(false)
    }
  }

  const exportMenuItems = [
    {
      key: 'json',
      icon: <FileTextOutlined />,
      label: '导出为 JSON',
      onClick: handleExportJSON,
    },
    {
      key: 'excel',
      icon: <FileExcelOutlined />,
      label: '导出为 CSV',
      onClick: handleExportExcel,
    },
    {
      key: 'database',
      icon: <DownloadOutlined />,
      label: '下载 SQLite 数据库',
      onClick: handleDownloadDatabase,
    },
  ]

  const menuItems = [
    {
      key: 'activity',
      icon: <DashboardOutlined />,
      label: '活动概览',
    },
    {
      key: 'invocations',
      icon: <HistoryOutlined />,
      label: '调用历史',
    },
    {
      key: 'models',
      icon: <SettingOutlined />,
      label: '模型管理',
    },
    {
      key: 'login-records',
      icon: <UserOutlined />,
      label: '登录记录',
    },
  ]

  const renderContent = () => {
    switch (activeKey) {
      case 'activity':
        return <ActivityDashboard />
      case 'invocations':
        return <InvocationList />
      case 'models':
        return <ModelManagement />
      case 'login-records':
        return <LoginRecordList />
      default:
        return <ActivityDashboard />
    }
  }

  return (
    <Layout style={{ minHeight: 'calc(100vh - 64px)', background: 'transparent' }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        theme="light"
        width={240}
        style={{
          borderRight: '1px solid #e5e7eb',
          background: 'transparent',
          position: 'sticky',
          top: 64,
          height: 'calc(100vh - 64px)',
          overflow: 'auto',
        }}
      >
        <div style={{ padding: '16px 0' }}>
          <Menu
            mode="inline"
            selectedKeys={[activeKey]}
            items={menuItems}
            onClick={({ key }) => setActiveKey(key)}
            style={{ borderRight: 'none', background: 'transparent' }}
          />
        </div>
        {!collapsed && (
          <div style={{ 
            position: 'absolute', 
            bottom: 24, 
            left: 24, 
            right: 24,
            padding: '16px',
            background: 'white',
            borderRadius: '12px',
            border: '1px solid #e5e7eb'
          }}>
            <div style={{ fontSize: '12px', color: '#6b7280', marginBottom: '8px' }}>LLM Router v1.1.0</div>
            <Button 
              type="text" 
              icon={<GithubOutlined />} 
              block 
              style={{ textAlign: 'left', padding: 0, height: 'auto' }}
              href="https://github.com/rinbarpen/llm-router"
              target="_blank"
            >
              GitHub Repo
            </Button>
          </div>
        )}
      </Sider>
      <Layout style={{ background: 'transparent' }}>
        <Header style={{ 
          background: 'transparent', 
          padding: '0 24px', 
          height: '48px', 
          lineHeight: '48px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
            style={{ fontSize: '16px', width: 32, height: 32 }}
          />
          <Space>
            <Dropdown menu={{ items: exportMenuItems }} trigger={['click']}>
              <Button 
                type="default" 
                icon={<DownloadOutlined />}
                loading={loading}
                size="small"
              >
                导出数据
              </Button>
            </Dropdown>
          </Space>
        </Header>
        <Content style={{ padding: '0 24px 24px' }}>
          {renderContent()}
        </Content>
      </Layout>
    </Layout>
  )
}

export default MonitorDashboard
