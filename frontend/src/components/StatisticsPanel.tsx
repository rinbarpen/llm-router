import React from 'react'
import { Row, Col, Card, Statistic, Table, Tag, Alert } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ClockCircleOutlined } from '@ant-design/icons'
import type { StatisticsResponse } from '../services/types'
import dayjs from 'dayjs'

interface StatisticsPanelProps {
  statistics: StatisticsResponse | null
  loading: boolean
}

const StatisticsPanel: React.FC<StatisticsPanelProps> = ({ statistics, loading }) => {
  if (!statistics) {
    return <div>加载中...</div>
  }

  const { overall, by_model, recent_errors } = statistics

  const modelColumns = [
    {
      title: '模型',
      dataIndex: 'model_name',
      key: 'model_name',
      render: (text: string, record: any) => (
        <span>
          <Tag color="blue">{record.provider_name}</Tag>
          {text}
        </span>
      ),
    },
    {
      title: '调用次数',
      dataIndex: 'total_calls',
      key: 'total_calls',
      sorter: (a: any, b: any) => a.total_calls - b.total_calls,
    },
    {
      title: '成功率',
      dataIndex: 'success_rate',
      key: 'success_rate',
      render: (rate: number) => `${rate.toFixed(2)}%`,
      sorter: (a: any, b: any) => a.success_rate - b.success_rate,
    },
    {
      title: '总Token数',
      dataIndex: 'total_tokens',
      key: 'total_tokens',
      render: (tokens: number) => tokens.toLocaleString(),
      sorter: (a: any, b: any) => a.total_tokens - b.total_tokens,
    },
    {
      title: '平均延迟',
      dataIndex: 'avg_duration_ms',
      key: 'avg_duration_ms',
      render: (ms: number | null) => (ms ? `${ms.toFixed(0)}ms` : '-'),
      sorter: (a: any, b: any) => (a.avg_duration_ms || 0) - (b.avg_duration_ms || 0),
    },
  ]

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="总调用次数"
              value={overall.total_calls}
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功次数"
              value={overall.success_calls}
              prefix={<CheckCircleOutlined style={{ color: '#3f8600' }} />}
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="失败次数"
              value={overall.error_calls}
              prefix={<CloseCircleOutlined style={{ color: '#cf1322' }} />}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={overall.success_rate}
              suffix="%"
              precision={2}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card title="Token使用统计">
            <Statistic
              title="总Token数"
              value={overall.total_tokens}
              valueStyle={{ fontSize: '24px' }}
            />
            {overall.avg_duration_ms && (
              <Statistic
                title="平均延迟"
                value={overall.avg_duration_ms}
                suffix="ms"
                style={{ marginTop: 16 }}
              />
            )}
          </Card>
        </Col>
        <Col span={8}>
          <Card title="时间范围">
            <p>统计范围: {overall.time_range}</p>
            <p>开始时间: {dayjs().subtract(parseInt(overall.time_range), 'hour').format('YYYY-MM-DD HH:mm:ss')}</p>
            <p>结束时间: {dayjs().format('YYYY-MM-DD HH:mm:ss')}</p>
          </Card>
        </Col>
        {overall.total_cost && (
          <Col span={8}>
            <Card title="成本统计">
              <Statistic
                title="总成本"
                value={overall.total_cost}
                prefix="$"
                precision={6}
                valueStyle={{ fontSize: '24px', color: '#3f8600' }}
              />
            </Card>
          </Col>
        )}
      </Row>

      <Card title="按模型统计" style={{ marginBottom: 16 }}>
        <Table
          dataSource={by_model}
          columns={modelColumns}
          rowKey="model_id"
          pagination={false}
          loading={loading}
        />
      </Card>

      {recent_errors.length > 0 && (
        <Card title="最近错误">
          {recent_errors.map((error) => (
            <Alert
              key={error.id}
              message={
                <span>
                  <Tag color="red">{error.provider_name}/{error.model_name}</Tag>
                  {dayjs(error.started_at).format('YYYY-MM-DD HH:mm:ss')}
                </span>
              }
              description={error.error_message || '未知错误'}
              type="error"
              style={{ marginBottom: 8 }}
              showIcon
            />
          ))}
        </Card>
      )}
    </div>
  )
}

export default StatisticsPanel

