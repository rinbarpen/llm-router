import React, { useState, useEffect } from 'react'
import { Row, Col, Card, DatePicker, Space, Button, Select, Typography, Divider } from 'antd'
import { ReloadOutlined, DollarOutlined, ThunderboltOutlined, InteractionOutlined } from '@ant-design/icons'
import dayjs, { Dayjs } from 'dayjs'
import { BarChart, Bar, ResponsiveContainer, Cell } from 'recharts'
import { dbService } from '../services/dbService'
import InvocationList from './InvocationList'
import TimeSeriesChart from './TimeSeriesChart'
import type { StatisticsResponse, TimeSeriesResponse } from '../services/types'

const { RangePicker } = DatePicker
const { Title, Text } = Typography

const ActivityDashboard: React.FC = () => {
  const [statistics, setStatistics] = useState<StatisticsResponse | null>(null)
  const [timeSeriesData, setTimeSeriesData] = useState<TimeSeriesResponse | null>(null)
  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(1, 'month'),
    dayjs(),
  ])
  const [loading, setLoading] = useState(false)
  const [timeRange, setTimeRange] = useState<number>(24 * 30) // 默认30天

  const loadData = async () => {
    setLoading(true)
    try {
      const [stats, timeSeries] = await Promise.all([
        dbService.getStatistics(timeRange, 10),
        dbService.getTimeSeries('day', timeRange),
      ])
      setStatistics(stats)
      setTimeSeriesData(timeSeries)
    } catch (error) {
      console.error('Failed to load data:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [timeRange])

  const handleDateRangeChange = (dates: [Dayjs | null, Dayjs | null] | null) => {
    if (dates && dates[0] && dates[1]) {
      setDateRange([dates[0], dates[1]])
      const hours = dates[1].diff(dates[0], 'hour')
      setTimeRange(hours)
    }
  }

  // 计算平均每天的值
  const days = Math.max(1, timeRange / 24)
  const avgDaySpend = statistics?.overall.total_cost
    ? (statistics.overall.total_cost / days).toFixed(4)
    : '0.0000'
  const avgDayTokens = statistics?.overall.total_tokens
    ? Math.round(statistics.overall.total_tokens / days).toLocaleString()
    : '0'
  const avgDayRequests = statistics?.overall.total_calls
    ? (statistics.overall.total_calls / days).toFixed(2)
    : '0.00'

  // 准备柱状图数据
  const prepareChartData = (type: 'spend' | 'tokens' | 'requests') => {
    if (!timeSeriesData || !timeSeriesData.data) return []
    const recentData = timeSeriesData.data.slice(-30)
    return recentData.map((point, index) => ({
      index,
      value: type === 'tokens' ? (point.total_tokens || 0) : (point.total_calls || 0),
    }))
  }

  const MiniBarChart: React.FC<{ data: Array<{ index: number; value: number }>; color: string }> = ({ data, color }) => {
    if (!data || data.length === 0) return <div className="activity-mini-chart-empty" />
    return (
      <ResponsiveContainer width="100%" height={40}>
        <BarChart data={data} margin={{ top: 0, right: 0, bottom: 0, left: 0 }}>
          <Bar dataKey="value" fill={color} radius={[2, 2, 0, 0]}>
            {data.map((_, index) => (
              <Cell key={`cell-${index}`} fillOpacity={0.6 + (index / data.length) * 0.4} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    )
  }

  return (
    <div className="activity-dashboard">
      <div className="activity-dashboard-header">
        <Title level={4} className="activity-dashboard-title">活动概览</Title>
        <Space className="activity-dashboard-controls" wrap>
          <RangePicker
            showTime
            value={dateRange}
            onChange={handleDateRangeChange}
            format="YYYY/MM/DD HH:mm"
            size="small"
            className="activity-range-picker"
          />
          <Select<number>
            value={timeRange === 24 * 30 ? 24 * 30 : timeRange === 24 * 7 ? 24 * 7 : 24}
            size="small"
            className="activity-range-select"
            options={[
              { label: '30天', value: 24 * 30 },
              { label: '7天', value: 24 * 7 },
              { label: '24小时', value: 24 },
            ]}
            onChange={(value) => {
              if (typeof value === 'number') {
                setTimeRange(value)
                if (value === 24 * 30) setDateRange([dayjs().subtract(1, 'month'), dayjs()])
                else if (value === 24 * 7) setDateRange([dayjs().subtract(7, 'day'), dayjs()])
                else setDateRange([dayjs().subtract(1, 'day'), dayjs()])
              }
            }}
          />
          <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading} size="small" shape="circle" />
        </Space>
      </div>

      {/* 顶部摘要卡片 */}
      <Row gutter={[16, 16]} className="activity-summary-row">
        <Col xs={24} sm={8}>
          <Card bordered={false} className="stat-card activity-stat-card">
            <div className="activity-stat-head">
              <Text type="secondary" strong>总花费 (USD)</Text>
              <DollarOutlined className="activity-stat-icon activity-stat-icon-spend" />
            </div>
            <Title level={2} className="activity-stat-value">
              ${statistics?.overall.total_cost?.toFixed(4) || '0.0000'}
            </Title>
            <div className="activity-stat-chart">
              <MiniBarChart data={prepareChartData('requests')} color="#14b8a6" />
            </div>
            <Divider className="activity-stat-divider" />
            <div className="activity-stat-foot">
              <Text type="secondary">日均花费</Text>
              <Text strong>${avgDaySpend}</Text>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="stat-card activity-stat-card">
            <div className="activity-stat-head">
              <Text type="secondary" strong>总令牌数 (Tokens)</Text>
              <ThunderboltOutlined className="activity-stat-icon activity-stat-icon-token" />
            </div>
            <Title level={2} className="activity-stat-value">
              {statistics?.overall.total_tokens?.toLocaleString() || '0'}
            </Title>
            <div className="activity-stat-chart">
              <MiniBarChart data={prepareChartData('tokens')} color="#10b981" />
            </div>
            <Divider className="activity-stat-divider" />
            <div className="activity-stat-foot">
              <Text type="secondary">日均消耗</Text>
              <Text strong>{avgDayTokens}</Text>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="stat-card activity-stat-card">
            <div className="activity-stat-head">
              <Text type="secondary" strong>总请求数 (Calls)</Text>
              <InteractionOutlined className="activity-stat-icon activity-stat-icon-request" />
            </div>
            <Title level={2} className="activity-stat-value">
              {statistics?.overall.total_calls?.toLocaleString() || '0'}
            </Title>
            <div className="activity-stat-chart">
              <MiniBarChart data={prepareChartData('requests')} color="#f59e0b" />
            </div>
            <Divider className="activity-stat-divider" />
            <div className="activity-stat-foot">
              <Text type="secondary">日均请求</Text>
              <Text strong>{avgDayRequests}</Text>
            </div>
          </Card>
        </Col>
      </Row>

      <div className="activity-section">
        <Title level={5} className="activity-section-title">时间序列分析</Title>
        <TimeSeriesChart />
      </div>

      <div className="activity-section">
        <Title level={5} className="activity-section-title">最近调用历史</Title>
        <InvocationList 
          startTime={dateRange[0]?.toDate()}
          endTime={dateRange[1]?.toDate()}
        />
      </div>
    </div>
  )
}

export default ActivityDashboard
