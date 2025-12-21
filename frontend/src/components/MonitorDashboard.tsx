import React, { useState, useEffect } from 'react'
import { Row, Col, Tabs, Select, Space, Button } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import StatisticsPanel from './StatisticsPanel'
import InvocationList from './InvocationList'
import TimeSeriesChart from './TimeSeriesChart'
import { monitorApi } from '../services/api'
import type { StatisticsResponse } from '../services/types'

const MonitorDashboard: React.FC = () => {
  const [statistics, setStatistics] = useState<StatisticsResponse | null>(null)
  const [timeRange, setTimeRange] = useState<number>(24)
  const [loading, setLoading] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)

  const loadStatistics = async () => {
    setLoading(true)
    try {
      const data = await monitorApi.getStatistics(timeRange, 10)
      setStatistics(data)
    } catch (error) {
      console.error('Failed to load statistics:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStatistics()
  }, [timeRange])

  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      loadStatistics()
    }, 5000) // 每5秒刷新一次

    return () => clearInterval(interval)
  }, [autoRefresh, timeRange])

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={24}>
          <Space>
            <span>时间范围:</span>
            <Select
              value={timeRange}
              onChange={setTimeRange}
              style={{ width: 120 }}
              options={[
                { label: '1小时', value: 1 },
                { label: '24小时', value: 24 },
                { label: '7天', value: 168 },
              ]}
            />
            <Button
              icon={<ReloadOutlined />}
              onClick={loadStatistics}
              loading={loading}
            >
              刷新
            </Button>
            <Button
              type={autoRefresh ? 'primary' : 'default'}
              onClick={() => setAutoRefresh(!autoRefresh)}
            >
              {autoRefresh ? '停止自动刷新' : '开启自动刷新'}
            </Button>
          </Space>
        </Col>
      </Row>

      <Tabs
        defaultActiveKey="statistics"
        items={[
          {
            key: 'statistics',
            label: '统计信息',
            children: <StatisticsPanel statistics={statistics} loading={loading} />,
          },
          {
            key: 'time-series',
            label: '时间序列',
            children: <TimeSeriesChart />,
          },
          {
            key: 'invocations',
            label: '调用历史',
            children: <InvocationList />,
          },
        ]}
      />
    </div>
  )
}

export default MonitorDashboard

