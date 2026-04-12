import React from 'react'
import { Button, Card, Col, Collapse, Row, Space, Steps, Typography } from 'antd'
import { ApiOutlined, KeyOutlined, MessageOutlined, RocketOutlined } from '@ant-design/icons'

const { Paragraph, Text, Title } = Typography

interface HelpPageProps {
  onNavigate: (page: 'api-doc' | 'model-square' | 'token-management' | 'chat') => void
}

const HelpPage: React.FC<HelpPageProps> = ({ onNavigate }) => (
  <div className="monitor-page help-page doc-page">
    <section className="page-hero">
      <div className="page-kicker">Help Center</div>
      <Title level={1}>Help</Title>
      <Paragraph>
        从配置 Provider、开放模型、签发令牌到调试 Chat，这里整理了使用 LLM Router Monitor 的核心路径。
      </Paragraph>
      <Space wrap>
        <Button type="primary" icon={<RocketOutlined />} onClick={() => onNavigate('model-square')}>
          浏览模型广场
        </Button>
        <Button icon={<ApiOutlined />} onClick={() => onNavigate('api-doc')}>
          查看 API Doc
        </Button>
      </Space>
    </section>

    <Row gutter={[16, 16]}>
      <Col xs={24} xl={10}>
        <Card className="editorial-card doc-card">
          <Title level={3}>快速开始</Title>
          <Steps
            direction="vertical"
            current={-1}
            items={[
              { title: '启动后端与 Monitor', description: '使用 scripts/start.sh 或分别启动 backend / monitor。' },
              { title: '配置 Provider', description: '确认 Provider、base URL、API key 与启用状态。' },
              { title: '签发令牌', description: '在令牌管理中创建 API Key，并设置限制与配额。' },
              { title: '测试 Chat', description: '进入 Chat 页面选择模型并发送测试请求。' },
            ]}
          />
        </Card>
      </Col>
      <Col xs={24} xl={14}>
        <Row gutter={[16, 16]}>
          <Col xs={24} md={12}>
            <Card className="editorial-card workflow-card">
              <KeyOutlined />
              <Text strong>管理访问令牌</Text>
              <Paragraph type="secondary">创建、复制、禁用 API Key，并配置模型或 Provider 访问范围。</Paragraph>
              <Button onClick={() => onNavigate('token-management')}>打开令牌管理</Button>
            </Card>
          </Col>
          <Col xs={24} md={12}>
            <Card className="editorial-card workflow-card">
              <MessageOutlined />
              <Text strong>调试模型响应</Text>
              <Paragraph type="secondary">使用 Chat 工作台测试流式输出、工具调用和多模态能力。</Paragraph>
              <Button onClick={() => onNavigate('chat')}>打开 Chat</Button>
            </Card>
          </Col>
          <Col xs={24}>
            <Card className="editorial-card doc-card">
              <Title level={3}>常见问题</Title>
              <Collapse
                ghost
                items={[
                  {
                    key: 'no-models',
                    label: '模型广场没有模型怎么办？',
                    children: '检查 Provider 是否已启用，并确认配置文件或数据库中存在可用模型。',
                  },
                  {
                    key: 'auth',
                    label: '什么时候需要 API Key 或 Session Token？',
                    children: '本地请求通常可免认证；远程访问或生产环境应使用 API Key 登录后携带 Session Token。',
                  },
                  {
                    key: 'logs',
                    label: '如何排查失败请求？',
                    children: '进入日志信息，按状态筛选失败调用，打开详情查看请求参数、响应和错误信息。',
                  },
                ]}
              />
            </Card>
          </Col>
        </Row>
      </Col>
    </Row>
  </div>
)

export default HelpPage
