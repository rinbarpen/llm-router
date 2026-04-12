import React, { Suspense, lazy, useEffect, useMemo, useState } from 'react'
import { Layout, Menu, Button, Space, Dropdown, Drawer, message, Grid, Spin } from 'antd'
import {
  ApiOutlined,
  AppstoreOutlined,
  BookOutlined,
  DashboardOutlined,
  DownloadOutlined,
  FileExcelOutlined,
  FileTextOutlined,
  GithubOutlined,
  HistoryOutlined,
  KeyOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MessageOutlined,
  MoonOutlined,
  SunOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { monitorApi } from '../services/api'
import type { ThemeMode } from '../hooks/useMonitorTheme'

const DashboardPage = lazy(() => import('./pages/DashboardPage'))
const TokenManagementPage = lazy(() => import('./pages/TokenManagementPage'))
const LogsPage = lazy(() => import('./pages/LogsPage'))
const ProfilePage = lazy(() => import('./pages/ProfilePage'))
const ModelSquarePage = lazy(() => import('./pages/ModelSquarePage'))
const ChatWorkbench = lazy(() => import('./ChatWorkbench'))
const HelpPage = lazy(() => import('./pages/HelpPage'))
const ApiDocPage = lazy(() => import('./pages/ApiDocPage'))

const { Sider, Content, Header } = Layout

export type PageKey =
  | 'dashboard'
  | 'token-management'
  | 'logs'
  | 'profile'
  | 'model-square'
  | 'chat'
  | 'help'
  | 'api-doc'

interface PageMeta {
  label: string
  eyebrow: string
  description: string
  icon: React.ReactNode
}

const PAGE_META: Record<PageKey, PageMeta> = {
  dashboard: {
    label: '仪表盘',
    eyebrow: 'Ops Core',
    description: '系统态势、调用趋势和近期活动',
    icon: <DashboardOutlined />,
  },
  'token-management': {
    label: '令牌管理',
    eyebrow: 'Access',
    description: 'API Key、配额、限制和有效期',
    icon: <KeyOutlined />,
  },
  logs: {
    label: '日志信息',
    eyebrow: 'Audit',
    description: '调用历史与登录访问审计',
    icon: <HistoryOutlined />,
  },
  profile: {
    label: '个人中心',
    eyebrow: 'Local Account',
    description: '本地偏好、最近会话和快捷入口',
    icon: <UserOutlined />,
  },
  'model-square': {
    label: '模型广场',
    eyebrow: 'Product Access',
    description: '浏览可用模型、能力和 Provider 状态',
    icon: <AppstoreOutlined />,
  },
  chat: {
    label: 'Chat',
    eyebrow: 'Workbench',
    description: '对话、流式输出、工具与多模态调试',
    icon: <MessageOutlined />,
  },
  help: {
    label: 'Help',
    eyebrow: 'Guides',
    description: '常用工作流、FAQ 和排障指南',
    icon: <BookOutlined />,
  },
  'api-doc': {
    label: 'API Doc',
    eyebrow: 'Reference',
    description: '认证、接口示例和开发者参考',
    icon: <ApiOutlined />,
  },
}

const OPS_NAV_ITEMS: PageKey[] = ['dashboard', 'token-management', 'logs', 'profile']
const PRODUCT_NAV_ITEMS: PageKey[] = ['model-square', 'chat', 'help', 'api-doc']
const ALL_PAGE_KEYS = new Set<PageKey>([...OPS_NAV_ITEMS, ...PRODUCT_NAV_ITEMS])

function isPageKey(value: string | null | undefined): value is PageKey {
  return Boolean(value && ALL_PAGE_KEYS.has(value as PageKey))
}

function getInitialPageKey(): PageKey {
  const hash = window.location.hash.replace(/^#\/?/, '')
  return isPageKey(hash) ? hash : 'dashboard'
}

interface MonitorDashboardProps {
  themeMode: ThemeMode
  onToggleTheme: () => void
}

const MonitorDashboard: React.FC<MonitorDashboardProps> = ({ themeMode, onToggleTheme }) => {
  const screens = Grid.useBreakpoint()
  const isMobile = !screens.lg

  const [collapsed, setCollapsed] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [activeKey, setActiveKey] = useState<PageKey>(getInitialPageKey)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    if (params.get('oauth') === 'success') {
      const provider = params.get('provider')
      message.success(provider ? `已通过 OAuth 绑定 Provider: ${provider}` : 'OAuth 绑定成功')
      window.history.replaceState({}, '', `${window.location.pathname}${window.location.hash}`)
    }
  }, [])

  useEffect(() => {
    const handleHashChange = () => {
      const hash = window.location.hash.replace(/^#\/?/, '')
      if (isPageKey(hash)) {
        setActiveKey(hash)
      }
    }
    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [])

  const navigateToPage = (page: PageKey) => {
    setActiveKey(page)
    setMobileMenuOpen(false)
    if (window.location.hash !== `#${page}`) {
      window.history.pushState({}, '', `${window.location.pathname}${window.location.search}#${page}`)
    }
  }

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

  const opsMenuItems = useMemo(
    () =>
      OPS_NAV_ITEMS.map((key) => ({
        key,
        icon: PAGE_META[key].icon,
        label: PAGE_META[key].label,
      })),
    [],
  )

  const activeMeta = PAGE_META[activeKey]

  const renderContent = () => {
    switch (activeKey) {
      case 'dashboard':
        return <DashboardPage onNavigate={navigateToPage} />
      case 'token-management':
        return <TokenManagementPage />
      case 'logs':
        return <LogsPage />
      case 'profile':
        return (
          <ProfilePage
            themeMode={themeMode}
            onToggleTheme={onToggleTheme}
            onNavigate={navigateToPage}
          />
        )
      case 'model-square':
        return <ModelSquarePage onNavigate={navigateToPage} />
      case 'chat':
        return (
          <div className="monitor-page chat-page-shell">
            <section className="page-hero compact-hero">
              <div className="page-kicker">Workbench</div>
              <h1>Chat</h1>
              <p>测试对话、流式输出、工具调用和多模态能力。</p>
            </section>
            <ChatWorkbench />
          </div>
        )
      case 'help':
        return <HelpPage onNavigate={navigateToPage} />
      case 'api-doc':
        return <ApiDocPage onNavigate={navigateToPage} />
      default:
        return <DashboardPage onNavigate={navigateToPage} />
    }
  }

  const opsMenuNode = (
    <Menu
      mode="inline"
      selectedKeys={OPS_NAV_ITEMS.includes(activeKey) ? [activeKey] : []}
      items={opsMenuItems}
      onClick={({ key }) => navigateToPage(key as PageKey)}
      className="monitor-nav-menu"
    />
  )

  const productNavNode = (
    <nav className="product-nav" aria-label="Product navigation">
      {PRODUCT_NAV_ITEMS.map((key) => (
        <button
          key={key}
          type="button"
          className={`product-nav-item${activeKey === key ? ' product-nav-item-active' : ''}`}
          onClick={() => navigateToPage(key)}
        >
          <span className="product-nav-icon">{PAGE_META[key].icon}</span>
          <span>{PAGE_META[key].label}</span>
        </button>
      ))}
    </nav>
  )

  return (
    <Layout className="monitor-shell">
      <Header className="monitor-deck-header">
        <div className="monitor-brand-block">
          <span className="monitor-brand-mark">LR</span>
          <div>
            <div className="monitor-brand-name">LLM Router</div>
            <div className="monitor-brand-subtitle">Editorial Control Deck</div>
          </div>
        </div>
        <div className="monitor-header-nav">{productNavNode}</div>
        <Space className="monitor-header-actions">
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

      <Layout className="monitor-deck-body">
        {!isMobile && (
          <Sider
            trigger={null}
            collapsible
            collapsed={collapsed}
            width={248}
            className="monitor-sidebar"
          >
            <div className="monitor-sidebar-inner">
              <div>
                <div className="monitor-sidebar-label">Ops Core</div>
                <div className="monitor-sidebar-menu">{opsMenuNode}</div>
              </div>
              {!collapsed && (
                <div className="monitor-sidebar-footer">
                  <div className="monitor-sidebar-version">LLM Router v1.2.0</div>
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
          <div className="mobile-product-drawer">{productNavNode}</div>
          {opsMenuNode}
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
                <div className="monitor-toolbar-kicker">{activeMeta.eyebrow}</div>
                <div className="monitor-toolbar-title">{activeMeta.label}</div>
              </div>
            </div>
            {!isMobile && <div className="monitor-toolbar-desc">{activeMeta.description}</div>}
          </Header>
          <Content className="monitor-content">
            <Suspense fallback={<div className="monitor-content-loading"><Spin /></div>}>
              {renderContent()}
            </Suspense>
          </Content>
        </Layout>
      </Layout>
    </Layout>
  )
}

export default MonitorDashboard
