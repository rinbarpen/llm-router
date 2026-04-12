import React from 'react'
import { Button, Card, Col, Row, Space, Tag, Typography } from 'antd'
import { CopyOutlined, KeyOutlined, MessageOutlined } from '@ant-design/icons'

const { Paragraph, Text, Title } = Typography

interface ApiDocPageProps {
  onNavigate: (page: 'help' | 'token-management' | 'chat') => void
}

const copyText = async (value: string) => {
  await navigator.clipboard.writeText(value)
}

const EndpointCard: React.FC<{
  method: string
  path: string
  title: string
  description: string
  example: string
}> = ({ method, path, title, description, example }) => (
  <Card className="editorial-card endpoint-card">
    <Space wrap className="endpoint-title">
      <Tag color={method === 'GET' ? 'blue' : 'green'}>{method}</Tag>
      <Text code>{path}</Text>
    </Space>
    <Title level={4}>{title}</Title>
    <Paragraph type="secondary">{description}</Paragraph>
    <pre>
      <code>{example}</code>
    </pre>
    <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(example)}>
      复制示例
    </Button>
  </Card>
)

const ApiDocPage: React.FC<ApiDocPageProps> = ({ onNavigate }) => (
  <div className="monitor-page api-doc-page doc-page">
    <section className="page-hero">
      <div className="page-kicker">Developer Reference</div>
      <Title level={1}>API Doc</Title>
      <Paragraph>
        LLM Router 提供 OpenAI 兼容接口、模型路由、Provider/Model 管理、API Key 管理和监控导出能力。
      </Paragraph>
      <Space wrap>
        <Button type="primary" icon={<KeyOutlined />} onClick={() => onNavigate('token-management')}>
          管理 API Key
        </Button>
        <Button icon={<MessageOutlined />} onClick={() => onNavigate('chat')}>
          在 Chat 测试
        </Button>
      </Space>
    </section>

    <Row gutter={[16, 16]}>
      <Col xs={24} xl={7}>
        <Card className="editorial-card doc-nav-card">
          <Title level={4}>基础信息</Title>
          <Paragraph>
            默认开发代理为 <Text code>/api</Text>，生产环境使用 <Text code>VITE_API_BASE_URL</Text> 或部署代理。
          </Paragraph>
          <Title level={5}>认证</Title>
          <Paragraph>
            本地请求可免认证；远程请求建议先 <Text code>POST /auth/login</Text> 获取 Session Token。
          </Paragraph>
          <Button onClick={() => onNavigate('help')}>查看 Help</Button>
        </Card>
      </Col>
      <Col xs={24} xl={17}>
        <Space direction="vertical" size="middle" className="doc-stack">
          <EndpointCard
            method="POST"
            path="/auth/login"
            title="登录获取 Session Token"
            description="使用 API Key 获取短期 Session Token，后续请求通过 Authorization Bearer 携带。"
            example={'curl -X POST http://localhost:18000/auth/login \\\n  -H "Authorization: Bearer <api-key>"'}
          />
          <EndpointCard
            method="POST"
            path="/v1/chat/completions"
            title="Chat Completions"
            description="OpenAI 兼容 Chat 接口，支持 stream=true 的 SSE 流式响应。"
            example={'curl -X POST http://localhost:18000/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer <token>" \\\n  -d \'{"model":"openai/gpt-4.1","messages":[{"role":"user","content":"hello"}]}\''}
          />
          <EndpointCard
            method="GET"
            path="/models"
            title="模型列表"
            description="返回当前已配置模型，可用于模型广场、Chat 模型选择和管理界面。"
            example={'curl http://localhost:18000/models \\\n  -H "Authorization: Bearer <token>"'}
          />
          <EndpointCard
            method="GET"
            path="/monitor/export/json"
            title="监控导出"
            description="导出指定时间范围内的监控数据，用于审计和排障。"
            example={'curl "http://localhost:18000/monitor/export/json?time_range_hours=24" \\\n  -H "Authorization: Bearer <token>"'}
          />
        </Space>
      </Col>
    </Row>
  </div>
)

export default ApiDocPage
