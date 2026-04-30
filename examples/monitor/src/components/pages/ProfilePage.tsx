import React, { useEffect, useMemo, useState } from 'react'
import { Button, Card, Col, Form, Input, InputNumber, Modal, Row, Select, Space, Switch, Tag, Typography, message } from 'antd'
import { ApiOutlined, DollarOutlined, KeyOutlined, MessageOutlined, TeamOutlined, UserOutlined } from '@ant-design/icons'
import { SESSION_TOKEN_KEY, apiKeyApi, consoleAuthApi, consoleTeamApi, orderApi, walletApi } from '../../services/api'
import type { APIKeyRead, ConsoleSession, RechargeCheckout, RechargeOrderRead, WalletRead } from '../../services/types'
import type { ThemeMode } from '../../hooks/useMonitorTheme'

const { Paragraph, Text, Title } = Typography
const CHAT_STORAGE_KEY = 'llm-router-chat-sessions-v1'

interface ProfilePageProps {
  themeMode: ThemeMode
  onToggleTheme: () => void
  onNavigate: (page: 'chat' | 'orders' | 'token-management' | 'help' | 'api-doc') => void
  onOpenOrder: (orderNo: string, checkout?: RechargeCheckout | null) => void
}

const ProfilePage: React.FC<ProfilePageProps> = ({ themeMode, onToggleTheme, onNavigate, onOpenOrder }) => {
  const [keys, setKeys] = useState<APIKeyRead[]>([])
  const [consoleSession, setConsoleSession] = useState<ConsoleSession | null>(null)
  const [wallet, setWallet] = useState<WalletRead | null>(null)
  const [recentOrders, setRecentOrders] = useState<RechargeOrderRead[]>([])
  const [teamCount, setTeamCount] = useState(0)
  const [acceptingInvite, setAcceptingInvite] = useState(false)
  const [rechargeOpen, setRechargeOpen] = useState(false)
  const [creatingRecharge, setCreatingRecharge] = useState(false)
  const [inviteForm] = Form.useForm<{ invite_token: string }>()
  const [rechargeForm] = Form.useForm<{ amount: number; currency: string; payment_provider: string }>()

  useEffect(() => {
    let mounted = true
    const params = new URLSearchParams(window.location.search)
    const inviteToken = params.get('invite_token')?.trim()
    if (inviteToken) {
      inviteForm.setFieldsValue({ invite_token: inviteToken })
    }
    Promise.allSettled([
      apiKeyApi.list(true),
      consoleAuthApi.me(),
      walletApi.me(),
      orderApi.list(),
      consoleTeamApi.list(),
    ]).then((results) => {
      if (!mounted) return
      const [keysResult, sessionResult, walletResult, orderResult, teamResult] = results
      if (keysResult.status === 'fulfilled') {
        setKeys(keysResult.value)
      } else {
        console.error(keysResult.reason)
        message.warning('账户令牌摘要加载失败')
      }
      if (sessionResult.status === 'fulfilled') {
        setConsoleSession(sessionResult.value)
      }
      if (walletResult.status === 'fulfilled') {
        setWallet(walletResult.value)
      }
      if (orderResult.status === 'fulfilled') {
        setRecentOrders(orderResult.value.slice(0, 3))
      }
      if (teamResult.status === 'fulfilled') {
        setTeamCount(teamResult.value.length)
      }
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
  const scopedTokens = keys.filter((item) => item.owner_type && item.owner_type !== 'system').length

  const refreshOrders = async () => {
    try {
      const items = await orderApi.list()
      setRecentOrders(items.slice(0, 3))
      message.success('订单状态已刷新')
    } catch (error) {
      console.error(error)
      message.error('刷新订单失败')
    }
  }

  const handleAcceptInvite = async () => {
    try {
      const values = await inviteForm.validateFields()
      setAcceptingInvite(true)
      await consoleTeamApi.acceptInvite(values.invite_token)
      inviteForm.resetFields()
      const teams = await consoleTeamApi.list()
      setTeamCount(teams.length)
      message.success('团队邀请已接受')
    } catch (error) {
      console.error(error)
      message.error('接受邀请失败')
    } finally {
      setAcceptingInvite(false)
    }
  }

  const handleCreateRecharge = async () => {
    try {
      const values = await rechargeForm.validateFields()
      setCreatingRecharge(true)
      const result = await orderApi.createRecharge(values)
      setRechargeOpen(false)
      rechargeForm.resetFields()
      await refreshOrders()
      onOpenOrder(result.order.order_no, result.checkout)
      message.success(`充值订单已创建：${result.order.order_no}`)
    } catch (error) {
      console.error(error)
      message.error('创建充值订单失败')
    } finally {
      setCreatingRecharge(false)
    }
  }

  return (
    <div className="monitor-page profile-page">
      <section className="page-hero compact-hero">
        <div className="page-kicker">Console Identity</div>
        <Title level={1}>个人中心</Title>
        <Paragraph>
          管理控制台用户、个人钱包、团队归属和最近充值订单，同时保留本地调试用的 Session Token 视图。
        </Paragraph>
      </section>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={9}>
          <Card className="editorial-card profile-identity-card">
            <div className="profile-avatar">
              <UserOutlined />
            </div>
            <Title level={3}>{consoleSession?.user.display_name || 'Local Console User'}</Title>
            <Text type="secondary">{consoleSession?.user.email || 'LLM Router Monitor'}</Text>
            <div className="profile-tags">
              <Tag color={hasSessionToken ? 'success' : 'default'}>
                {hasSessionToken ? 'Session Token 已启用' : '本地访问'}
              </Tag>
              {consoleSession?.user.roles?.map((role) => (
                <Tag color="cyan" key={role}>{role}</Tag>
              ))}
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
                <Paragraph type="secondary">当前可用 API Key 数量，其中 {scopedTokens} 个已绑定到用户或团队钱包。</Paragraph>
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
                <Text strong>钱包余额</Text>
                <Title level={2}>{wallet ? `${wallet.balance.toFixed(2)} ${wallet.currency}` : '--'}</Title>
                <Paragraph type="secondary">
                  最近充值订单 {recentOrders.length} 笔，最新状态 {recentOrders[0]?.status || '暂无'}。
                </Paragraph>
                <Space wrap>
                  <Button icon={<DollarOutlined />} onClick={() => onNavigate('token-management')}>
                    查看支付资源
                  </Button>
                  <Button type="primary" onClick={() => setRechargeOpen(true)}>
                    发起充值
                  </Button>
                  <Button onClick={() => void refreshOrders()}>刷新订单</Button>
                  <Button onClick={() => onNavigate('orders')}>订单中心</Button>
                  <Tag color={wallet?.status === 'active' ? 'success' : 'default'}>
                    {wallet?.status || 'unavailable'}
                  </Tag>
                </Space>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card className="editorial-card preference-card">
                <Text strong>团队归属</Text>
                <Title level={2}>{teamCount}</Title>
                <Paragraph type="secondary">平台控制台中当前可见团队数量。</Paragraph>
                <Space wrap>
                  <Tag icon={<TeamOutlined />} color="geekblue">
                    {consoleSession?.user.roles?.join(', ') || 'member'}
                  </Tag>
                </Space>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card className="editorial-card preference-card">
                <Text strong>接受邀请</Text>
                <Paragraph type="secondary">输入团队邀请 token，接受后会把当前控制台账户加入对应团队。</Paragraph>
                <Form form={inviteForm} layout="vertical">
                  <Form.Item name="invite_token" rules={[{ required: true, message: '请输入邀请 token' }]}>
                    <Input placeholder="ti_xxx" />
                  </Form.Item>
                </Form>
                <Button type="primary" loading={acceptingInvite} onClick={() => void handleAcceptInvite()}>
                  接受团队邀请
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

      <Modal
        title="个人钱包充值"
        open={rechargeOpen}
        onCancel={() => setRechargeOpen(false)}
        onOk={() => void handleCreateRecharge()}
        confirmLoading={creatingRecharge}
        destroyOnClose
      >
        <Form
          form={rechargeForm}
          layout="vertical"
          initialValues={{ currency: wallet?.currency || 'CNY', payment_provider: 'stripe', amount: 100 }}
        >
          <Form.Item name="amount" label="充值金额" rules={[{ required: true, message: '请输入金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="currency" label="币种" rules={[{ required: true, message: '请选择币种' }]}>
            <Select options={[{ label: 'CNY', value: 'CNY' }, { label: 'USD', value: 'USD' }]} />
          </Form.Item>
          <Form.Item name="payment_provider" label="支付渠道" rules={[{ required: true, message: '请选择支付渠道' }]}>
            <Select
              options={[
                { label: 'Stripe', value: 'stripe' },
                { label: '支付宝', value: 'alipay' },
                { label: '微信支付', value: 'wechatpay' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ProfilePage
