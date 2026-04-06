import React, { Suspense, lazy, useEffect, useMemo, useState } from 'react'
import { Layout, Menu, Button, Space, Dropdown, Drawer, message, Grid, Spin } from 'antd'
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
  GithubOutlined,
  ThunderboltOutlined,
  MessageOutlined,
  MoonOutlined,
  SunOutlined,
} from '@ant-design/icons'
import { monitorApi } from '../services/api'
import type { ThemeMode } from '../hooks/useMonitorTheme'

const { Sider, Content, Header } = Layout

type DashboardKey = 'activity' | 'invocations' | 'models' | 'login-records' | 'multimodal' | 'chat-web'

interface DashboardMeta {
  label: string
  description: string
  icon: React.ReactNode
}

const DASHBOARD_META: Record<DashboardKey, DashboardMeta> = {
  activity: {
    label: '活动概览',
    description: '查看调用成功率、时序变化和近期错误分布',
    icon: <DashboardOutlined />,
  },
  invocations: {
    label: '调用历史',
    description: '按模型与 Provider 过滤调用记录，快速定位异常请求',
    icon: <HistoryOutlined />,
  },
  models: {
    label: '模型管理',
    description: '管理 Provider、模型能力、定价与启停状态',
    icon: <SettingOutlined />,
  },
  'login-records': {
    label: '登录记录',
    description: '审计用户登录行为与认证方式',
    icon: <UserOutlined />,
  },
  multimodal: {
    label: '多能力调试',
    description: '统一调试 Embedding、语音、图片与视频能力',
    icon: <ThunderboltOutlined />,
  },
  'chat-web': {
    label: 'Chat Web',
    description: '统一模型聊天工作台，支持流式、重放和调试视图',
    icon: <MessageOutlined />,
  },
}

const LazyActivityDashboard = lazy(() => import('./ActivityDashboard'))
const LazyInvocationList = lazy(() => import('./InvocationList'))
const LazyModelManagement = lazy(() => import('./ModelManagement'))
const LazyLoginRecordList = lazy(() => import('./LoginRecordList'))
const LazyMultimodalWorkbench = lazy(() => import('./MultimodalWorkbench'))
const LazyChatWorkbench = lazy(() => import('./ChatWorkbench'))

interface MonitorDashboardProps {
  themeMode: ThemeMode
  onToggleTheme: () => void
}

const MonitorDashboard: React.FC<MonitorDashboardProps> = ({ themeMode, onToggleTheme }) => {
  const screens = Grid.useBreakpoint()
  const isMobile = !screens.lg

  const [collapsed, setCollapsed] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [activeKey, setActiveKey] = useState<DashboardKey>('activity')
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
      message.success('JSON 导出成功')
    } catch (error) {
      console.error('Failed to export JSON:', error)
      message.error('JSON 导出失败')
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
      message.success('CSV 导出成功')
    } catch (error) {
      console.error('Failed to export CSV:', error)
      message.error('CSV 导出失败')
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
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      link.download = `llm_router_monitor_export_${timestamp}.zip`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('监控导出包下载成功')
    } catch (error) {
      console.error('Failed to download database:', error)
      message.error('监控导出包下载失败')
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
      label: '下载监控导出包',
      onClick: handleDownloadDatabase,
    },
  ]

  const menuItems = useMemo(
    () =>
      (Object.keys(DASHBOARD_META) as DashboardKey[]).map((key) => ({
        key,
        icon: DASHBOARD_META[key].icon,
        label: DASHBOARD_META[key].label,
      })),
    [],
  )

  const activeMeta = DASHBOARD_META[activeKey]

  const renderContent = () => {
    switch (activeKey) {
      case 'activity':
        return <LazyActivityDashboard />
      case 'invocations':
        return <LazyInvocationList />
      case 'models':
        return <LazyModelManagement />
      case 'login-records':
        return <LazyLoginRecordList />
      case 'multimodal':
        return <LazyMultimodalWorkbench />
      case 'chat-web':
        return <LazyChatWorkbench />
      default:
        return <LazyActivityDashboard />
    }
  }

  const menuNode = (
    <Menu
      mode="inline"
      selectedKeys={[activeKey]}
      items={menuItems}
      onClick={({ key }) => {
        setActiveKey(key as DashboardKey)
        setMobileMenuOpen(false)
      }}
      className="monitor-nav-menu"
    />
  )

  return (
    <Layout className="monitor-shell">
      {!isMobile && (
        <Sider
          trigger={null}
          collapsible
          collapsed={collapsed}
          width={252}
          className="monitor-sidebar"
        >
          <div className="monitor-sidebar-inner">
            <div className="monitor-sidebar-menu">{menuNode}</div>
            {!collapsed && (
              <div className="monitor-sidebar-footer">
                <div className="monitor-sidebar-version">LLM Router v1.1.0</div>
                <Button
                  type="text"
                  icon={<GithubOutlined />}
                  block
                  className="monitor-sidebar-github"
                  href="https://github.com/rinbarpen/llm-router"
                  target="_blank"
                >
                  GitHub Repo
                </Button>
              </div>
            )}
          </div>
        </Sider>
      )}

      <Drawer
        placement="left"
        open={isMobile && mobileMenuOpen}
        onClose={() => setMobileMenuOpen(false)}
        title="导航"
        bodyStyle={{ padding: 0 }}
      >
        {menuNode}
      </Drawer>

      <Layout className="monitor-main-layout">
        <Header className="monitor-toolbar">
          <div className="monitor-toolbar-left">
            <Button
              type="text"
              icon={
                isMobile ? (
                  <MenuUnfoldOutlined />
                ) : collapsed ? (
                  <MenuUnfoldOutlined />
                ) : (
                  <MenuFoldOutlined />
                )
              }
              onClick={() => {
                if (isMobile) {
                  setMobileMenuOpen(true)
                } else {
                  setCollapsed((prev) => !prev)
                }
              }}
              className="monitor-toolbar-toggle"
            />
            <div>
              <div className="monitor-toolbar-title">{activeMeta.label}</div>
              {!isMobile && <div className="monitor-toolbar-desc">{activeMeta.description}</div>}
            </div>
          </div>

          <Space>
            <Button
              icon={themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
              onClick={onToggleTheme}
            >
              {themeMode === 'dark' ? '浅色' : '深色'}
            </Button>
            <Dropdown menu={{ items: exportMenuItems }} trigger={['click']}>
              <Button icon={<DownloadOutlined />} loading={loading}>
                导出数据
              </Button>
            </Dropdown>
          </Space>
        </Header>
        <Content className="monitor-content">
          <Suspense fallback={<div className="monitor-content-loading"><Spin /></div>}>{renderContent()}</Suspense>
        </Content>
      </Layout>
    </Layout>
  )
}

export default MonitorDashboard
