import React, { useState } from 'react'
import { Alert, Button, Card, Form, Input, Space, Typography, message } from 'antd'
import { LockOutlined, LoginOutlined, MailOutlined, MonitorOutlined } from '@ant-design/icons'
import { consoleAuthApi } from '../../services/api'
import type { ConsoleSession } from '../../services/types'

const { Paragraph, Text, Title } = Typography

interface LoginPageProps {
  onLoggedIn: (session: ConsoleSession) => void
  onEnterLocalMode: () => void
}

const LoginPage: React.FC<LoginPageProps> = ({ onLoggedIn, onEnterLocalMode }) => {
  const [loading, setLoading] = useState(false)
  const [form] = Form.useForm<{ email: string; password: string }>()

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setLoading(true)
      const session = await consoleAuthApi.login(values.email, values.password)
      message.success(`已登录 ${session.user.display_name || session.user.email}`)
      onLoggedIn(session)
    } catch (error) {
      if (error instanceof Error && error.message.includes('validate')) {
        return
      }
      console.error(error)
      message.error('控制台登录失败，请检查邮箱或密码')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="monitor-login-shell">
      <div className="monitor-login-grid">
        <section className="monitor-login-hero">
          <div className="page-kicker">Platform Console</div>
          <Title>LLM Router 平台控制台</Title>
          <Paragraph>
            通过控制台账户进入平台化的用户、团队、钱包和支付工作流。若你当前仍在调试旧版本地监控，也可以切换到兼容模式继续使用。
          </Paragraph>
          <Space wrap>
            <Alert
              type="info"
              showIcon
              message="控制台登录走 HttpOnly Cookie，会话与 API Key 登录分离。"
            />
          </Space>
        </section>

        <Card className="editorial-card monitor-login-card">
          <Title level={3}>登录控制台</Title>
          <Paragraph type="secondary">
            使用平台账户登录后，可管理 API Key 归属、团队钱包和充值订单。
          </Paragraph>
          <Form form={form} layout="vertical" onFinish={handleSubmit}>
            <Form.Item
              name="email"
              label="邮箱"
              rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]}
            >
              <Input prefix={<MailOutlined />} placeholder="owner@example.com" />
            </Form.Item>
            <Form.Item
              name="password"
              label="密码"
              rules={[{ required: true, message: '请输入密码' }]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="输入控制台密码" />
            </Form.Item>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Button type="primary" icon={<LoginOutlined />} htmlType="submit" loading={loading} block>
                登录控制台
              </Button>
              <Button icon={<MonitorOutlined />} onClick={onEnterLocalMode} block>
                进入本地监控模式
              </Button>
            </Space>
          </Form>
          <div className="monitor-login-footnote">
            <Text type="secondary">
              首版兼容策略：控制台未配置账户时，仍可使用本地模式浏览旧版监控页面。
            </Text>
          </div>
        </Card>
      </div>
    </div>
  )
}

export default LoginPage
