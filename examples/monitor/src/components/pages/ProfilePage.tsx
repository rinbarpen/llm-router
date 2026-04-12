import React, { useEffect, useMemo, useState } from 'react'
import { Button, Card, Col, Row, Space, Switch, Tag, Typography, message } from 'antd'
import { ApiOutlined, KeyOutlined, MessageOutlined, UserOutlined } from '@ant-design/icons'
import { SESSION_TOKEN_KEY, apiKeyApi } from '../../services/api'
import type { APIKeyRead } from '../../services/types'
import type { ThemeMode } from '../../hooks/useMonitorTheme'

const { Paragraph, Text, Title } = Typography
const CHAT_STORAGE_KEY = 'llm-router-chat-sessions-v1'

interface ProfilePageProps {
  themeMode: ThemeMode
  onToggleTheme: () => void
  onNavigate: (page: 'chat' | 'token-management' | 'help' | 'api-doc') => void
}

const ProfilePage: React.FC<ProfilePageProps> = ({ themeMode, onToggleTheme, onNavigate }) => {
  const [keys, setKeys] = useState<APIKeyRead[]>([])

  useEffect(() => {
    let mounted = true
    apiKeyApi
      .list(true)
      .then((items) => {
        if (mounted) setKeys(items)
      })
      .catch((error) => {
        console.error(error)
        message.warning('账户令牌摘要加载失败')
      })
    return () => {
      mounted = false
    }
  }, [])

  const chatSummary = useMemo(() => {
    try {
      const raw = localStorage.getItem(CHAT_STORAGE_KEY)
      if (!raw) return { count: 0, latest: '暂无会话' }
      const sessions = JSON.parse(raw) as Array<{ title?: string; updatedAt?: string }>
      const latest = sessions
        .slice()
        .sort((a, b) => String(b.updatedAt ?? '').localeCompare(String(a.updatedAt ?? '')))[0]
      return { count: sessions.length, latest: latest?.title || '新会话' }
    } catch {
      return { count: 0, latest: '无法读取会话' }
    }
  }, [])

  const hasSessionToken = Boolean(localStorage.getItem(SESSION_TOKEN_KEY))
  const activeTokens = keys.filter((item) => item.is_active).length

  return (
    <div className="monitor-page profile-page">
      <section className="page-hero compact-hero">
        <div className="page-kicker">Local Account</div>
        <Title level={1}>个人中心</Title>
        <Paragraph>
          管理本地控制台偏好、最近会话和常用入口。此页面不引入团队、角色或 IAM 配置。
        </Paragraph>
      </section>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={9}>
          <Card className="editorial-card profile-identity-card">
            <div className="profile-avatar">
              <UserOutlined />
            </div>
            <Title level={3}>Local Console User</Title>
            <Text type="secondary">LLM Router Monitor</Text>
            <div className="profile-tags">
              <Tag color={hasSessionToken ? 'success' : 'default'}>
                {hasSessionToken ? 'Session Token 已启用' : '本地访问'}
              </Tag>
              <Tag color="blue">{themeMode === 'dark' ? 'Dark' : 'Light'} Theme</Tag>
            </div>
          </Card>
        </Col>
        <Col xs={24} xl={15}>
          <Row gutter={[16, 16]}>
            <Col xs={24} md={12}>
              <Card className="editorial-card preference-card">
                <Text strong>界面偏好</Text>
                <div className="preference-row">
                  <span>深色模式</span>
                  <Switch checked={themeMode === 'dark'} onChange={onToggleTheme} />
                </div>
                <Paragraph type="secondary">主题设置会保存在当前浏览器。</Paragraph>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card className="editorial-card preference-card">
                <Text strong>令牌摘要</Text>
                <Title level={2}>{activeTokens}</Title>
                <Paragraph type="secondary">当前可用 API Key 数量。</Paragraph>
                <Button icon={<KeyOutlined />} onClick={() => onNavigate('token-management')}>
                  打开令牌管理
                </Button>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card className="editorial-card preference-card">
                <Text strong>最近 Chat</Text>
                <Title level={2}>{chatSummary.count}</Title>
                <Paragraph type="secondary">{chatSummary.latest}</Paragraph>
                <Button icon={<MessageOutlined />} onClick={() => onNavigate('chat')}>
                  继续 Chat
                </Button>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card className="editorial-card preference-card">
                <Text strong>开发者入口</Text>
                <Paragraph type="secondary">查看使用指南与 API 示例。</Paragraph>
                <Space wrap>
                  <Button onClick={() => onNavigate('help')}>Help</Button>
                  <Button icon={<ApiOutlined />} onClick={() => onNavigate('api-doc')}>
                    API Doc
                  </Button>
                </Space>
              </Card>
            </Col>
          </Row>
        </Col>
      </Row>
    </div>
  )
}

export default ProfilePage
