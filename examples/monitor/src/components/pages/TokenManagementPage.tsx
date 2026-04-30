import React, { useEffect, useMemo, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, message } from 'antd'
import { ClockCircleOutlined, KeyOutlined, SafetyOutlined, ThunderboltOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import APIKeyManagementPane from '../APIKeyManagementPane'
import { apiKeyApi } from '../../services/api'
import type { APIKeyRead } from '../../services/types'

const { Paragraph, Title } = Typography

const TokenManagementPage: React.FC = () => {
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
        message.warning('令牌摘要加载失败，主列表仍可使用')
      })
    return () => {
      mounted = false
    }
  }, [])

  const summary = useMemo(() => {
    const now = dayjs()
    return {
      active: keys.filter((item) => item.is_active).length,
      inactive: keys.filter((item) => !item.is_active).length,
      expiring: keys.filter((item) => item.expires_at && dayjs(item.expires_at).diff(now, 'day') <= 14).length,
      quota: keys.filter((item) => typeof item.quota_tokens_monthly === 'number' && item.quota_tokens_monthly > 0).length,
      owned: keys.filter((item) => item.owner_type && item.owner_type !== 'system').length,
    }
  }, [keys])

  return (
    <div className="monitor-page token-page">
      <section className="page-hero compact-hero">
        <div className="page-kicker">Access Control</div>
        <Title level={1}>令牌管理</Title>
        <Paragraph>
          管理系统 API Key、月度令牌配额、模型/Provider 访问限制和参数上限。密钥默认脱敏，复制和显示都需要显式操作。
        </Paragraph>
      </section>

      <Row gutter={[16, 16]} className="summary-grid">
        <Col xs={12} lg={6}>
          <Card className="metric-card">
            <Statistic title="Active Tokens" value={summary.active} prefix={<KeyOutlined />} />
          </Card>
        </Col>
        <Col xs={12} lg={6}>
          <Card className="metric-card">
            <Statistic title="Inactive" value={summary.inactive} prefix={<SafetyOutlined />} />
          </Card>
        </Col>
        <Col xs={12} lg={6}>
          <Card className="metric-card">
            <Statistic title="Expiring Soon" value={summary.expiring} prefix={<ClockCircleOutlined />} />
          </Card>
        </Col>
        <Col xs={12} lg={6}>
          <Card className="metric-card">
            <Statistic title="Quota Rules" value={summary.quota} prefix={<ThunderboltOutlined />} suffix={`/ ${summary.owned} scoped`} />
          </Card>
        </Col>
      </Row>

      <div className="ops-panel">
        <APIKeyManagementPane />
      </div>
    </div>
  )
}

export default TokenManagementPage
