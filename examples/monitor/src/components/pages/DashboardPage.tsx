import React from 'react'
import { Button, Card, Col, Row, Space, Typography } from 'antd'
import {
  ApiOutlined,
  BarChartOutlined,
  KeyOutlined,
  MessageOutlined,
  RocketOutlined,
} from '@ant-design/icons'
import ActivityDashboard from '../ActivityDashboard'

const { Paragraph, Text, Title } = Typography

interface DashboardPageProps {
  onNavigate: (page: 'token-management' | 'logs' | 'model-square' | 'chat') => void
}

const shortcuts = [
  {
    key: 'token-management' as const,
    title: '令牌管理',
    description: '检查 API Key 配额、有效期和访问限制。',
    icon: <KeyOutlined />,
  },
  {
    key: 'logs' as const,
    title: '日志信息',
    description: '排查调用失败、登录审计和请求明细。',
    icon: <BarChartOutlined />,
  },
  {
    key: 'model-square' as const,
    title: '模型广场',
    description: '浏览可用模型并进入配置或 Chat。',
    icon: <RocketOutlined />,
  },
  {
    key: 'chat' as const,
    title: 'Chat',
    description: '用当前模型路由快速测试对话与工具调用。',
    icon: <MessageOutlined />,
  },
]

const DashboardPage: React.FC<DashboardPageProps> = ({ onNavigate }) => (
  <div className="monitor-page dashboard-page">
    <section className="page-hero dashboard-hero">
      <div className="page-kicker">Control Overview</div>
      <Title level={1}>系统态势总览</Title>
      <Paragraph>
        从调用、成本、令牌消耗和近期异常开始观察 LLM Router。需要操作时，可从下方快捷入口进入令牌、日志、模型和 Chat 工作台。
      </Paragraph>
      <Space wrap>
        <Button type="primary" icon={<BarChartOutlined />} onClick={() => onNavigate('logs')}>
          查看日志
        </Button>
        <Button icon={<ApiOutlined />} onClick={() => onNavigate('model-square')}>
          浏览模型
        </Button>
      </Space>
    </section>

    <Row gutter={[16, 16]} className="dashboard-shortcuts">
      {shortcuts.map((item) => (
        <Col xs={24} sm={12} xl={6} key={item.key}>
          <Card className="editorial-card shortcut-card" onClick={() => onNavigate(item.key)}>
            <div className="shortcut-icon">{item.icon}</div>
            <Text strong>{item.title}</Text>
            <Paragraph type="secondary">{item.description}</Paragraph>
          </Card>
        </Col>
      ))}
    </Row>

    <div className="ops-panel dashboard-activity-panel">
      <ActivityDashboard />
    </div>
  </div>
)

export default DashboardPage
