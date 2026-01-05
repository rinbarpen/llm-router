import React, { useState, useEffect } from 'react'
import { Row, Col, Card, Statistic, DatePicker, Space, Button, Select } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import dayjs, { Dayjs } from 'dayjs'
import { BarChart, Bar, ResponsiveContainer } from 'recharts'
import { dbService } from '../services/dbService'
import InvocationList from './InvocationList'
import TimeSeriesChart from './TimeSeriesChart'
import type { StatisticsResponse, TimeSeriesResponse } from '../services/types'

const { RangePicker } = DatePicker

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
  const days = timeRange / 24
  const avgDaySpend = statistics?.overall.total_cost
    ? (statistics.overall.total_cost / days).toFixed(4)
    : '0.0000'
  const avgDayTokens = statistics?.overall.total_tokens
    ? Math.round(statistics.overall.total_tokens / days).toLocaleString()
    : '0'
  const avgDayRequests = statistics?.overall.total_calls
    ? (statistics.overall.total_calls / days).toFixed(2)
    : '0.00'

  // 准备柱状图数据（最近30天的每日数据）
  const prepareChartData = (type: 'spend' | 'tokens' | 'requests') => {
    if (!timeSeriesData || !timeSeriesData.data) return []
    
    // 取最近30个数据点
    const recentData = timeSeriesData.data.slice(-30)
    
    return recentData.map((point, index) => ({
      index,
      value: (() => {
        if (type === 'spend') {
          // 对于spend，我们需要从统计数据中估算（这里简化处理）
          return 0 // 暂时返回0，因为时间序列数据中没有cost
        } else if (type === 'tokens') {
          return point.total_tokens || 0
        } else {
          return point.total_calls || 0
        }
      })(),
    }))
  }

  const MiniBarChart: React.FC<{ data: Array<{ index: number; value: number }>; color: string }> = ({ data, color }) => {
    if (!data || data.length === 0) {
      return <div style={{ height: 40 }} />
    }
    
    return (
      <ResponsiveContainer width="100%" height={40}>
        <BarChart data={data} margin={{ top: 0, right: 0, bottom: 0, left: 0 }}>
          <Bar dataKey="value" fill={color} radius={[2, 2, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    )
  }

  return (
    <div>
      {/* 顶部摘要卡片 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <div style={{ marginBottom: 8 }}>
              <MiniBarChart data={prepareChartData('spend')} color="#1890ff" />
            </div>
            <Statistic
              title="花费"
              value={statistics?.overall.total_cost || 0}
              prefix="$"
              precision={4}
              valueStyle={{ fontSize: '20px', fontWeight: 'bold' }}
            />
            <div style={{ marginTop: 8, fontSize: '12px', color: '#999' }}>
              日均: <strong>${avgDaySpend}</strong> | 过去一月: <strong>${statistics?.overall.total_cost?.toFixed(4) || '0.0000'}</strong>
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <div style={{ marginBottom: 8 }}>
              <MiniBarChart data={prepareChartData('tokens')} color="#52c41a" />
            </div>
            <Statistic
              title="令牌数"
              value={statistics?.overall.total_tokens || 0}
              valueStyle={{ fontSize: '20px', fontWeight: 'bold' }}
            />
            <div style={{ marginTop: 8, fontSize: '12px', color: '#999' }}>
              日均: <strong>{avgDayTokens}</strong> | 过去一月: <strong>{statistics?.overall.total_tokens?.toLocaleString() || '0'}</strong>
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <div style={{ marginBottom: 8 }}>
              <MiniBarChart data={prepareChartData('requests')} color="#faad14" />
            </div>
            <Statistic
              title="请求数"
              value={statistics?.overall.total_calls || 0}
              valueStyle={{ fontSize: '20px', fontWeight: 'bold' }}
            />
            <div style={{ marginTop: 8, fontSize: '12px', color: '#999' }}>
              日均: <strong>{avgDayRequests}</strong> | 过去一月: <strong>{statistics?.overall.total_calls || 0}</strong>
            </div>
          </Card>
        </Col>
      </Row>

      {/* 日期范围选择器和操作按钮 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={12}>
          <Space>
            <span>从:</span>
            <RangePicker
              showTime
              value={dateRange}
              onChange={handleDateRangeChange}
              format="YYYY/MM/DD HH:mm"
            />
          </Space>
        </Col>
        <Col span={12} style={{ textAlign: 'right' }}>
          <Space>
            <Select<number>
              value={timeRange === 24 * 30 ? 24 * 30 : timeRange === 24 * 7 ? 24 * 7 : 24}
              style={{ width: 120 }}
              options={[
                { label: '1个月', value: 24 * 30 },
                { label: '7天', value: 24 * 7 },
                { label: '24小时', value: 24 },
              ]}
              onChange={(value) => {
                if (typeof value === 'number') {
                  setTimeRange(value)
                  if (value === 24 * 30) {
                    setDateRange([dayjs().subtract(1, 'month'), dayjs()])
                  } else if (value === 24 * 7) {
                    setDateRange([dayjs().subtract(7, 'day'), dayjs()])
                  } else {
                    setDateRange([dayjs().subtract(1, 'day'), dayjs()])
                  }
                }
              }}
            />
            <Select
              defaultValue="按模型"
              style={{ width: 120 }}
              options={[
                { label: '按模型', value: 'model' },
                { label: '按提供商', value: 'provider' },
              ]}
            />
            <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>
              刷新
            </Button>
          </Space>
        </Col>
      </Row>

      {/* 时间序列图表 */}
      <div style={{ marginBottom: 24 }}>
        <TimeSeriesChart />
      </div>

      {/* 调用历史列表 */}
      <InvocationList 
        startTime={dateRange[0]?.toDate()}
        endTime={dateRange[1]?.toDate()}
      />
    </div>
  )
}

export default ActivityDashboard

