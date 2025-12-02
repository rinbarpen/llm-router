import React, { useState, useEffect } from 'react'
import { Card, Select, Space, Spin } from 'antd'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { monitorApi } from '../services/api'
import type { TimeSeriesResponse } from '../services/types'
import dayjs from 'dayjs'

type Granularity = 'hour' | 'day' | 'week' | 'month'

const TimeSeriesChart: React.FC = () => {
  const [granularity, setGranularity] = useState<Granularity>('day')
  const [data, setData] = useState<TimeSeriesResponse | null>(null)
  const [loading, setLoading] = useState(false)

  // 根据粒度确定时间范围
  const getTimeRangeHours = (gran: Granularity): number => {
    switch (gran) {
      case 'hour':
        return 24 // 最近24小时
      case 'day':
        return 168 // 最近7天
      case 'week':
        return 720 // 最近30天
      case 'month':
        return 2160 // 最近90天
      default:
        return 168
    }
  }

  useEffect(() => {
    const loadData = async () => {
      setLoading(true)
      try {
        const timeRangeHours = getTimeRangeHours(granularity)
        const response = await monitorApi.getTimeSeries(granularity, timeRangeHours)
        setData(response)
      } catch (error) {
        console.error('Failed to load time series data:', error)
      } finally {
        setLoading(false)
      }
    }
    loadData()
  }, [granularity])

  // 格式化图表数据
  const chartData = data?.data.map((point) => {
    const timestamp = dayjs(point.timestamp)
    let label = ''
    
    switch (granularity) {
      case 'hour':
        label = timestamp.format('MM-DD HH:00')
        break
      case 'day':
        label = timestamp.format('MM-DD')
        break
      case 'week':
        label = timestamp.format('MM-DD')
        break
      case 'month':
        label = timestamp.format('YYYY-MM')
        break
    }

    return {
      time: label,
      timestamp: point.timestamp,
      'Total Calls': point.total_calls,
      'Success Calls': point.success_calls,
      'Error Calls': point.error_calls,
      'Total Tokens': point.total_tokens,
    }
  }) || []

  return (
    <Card
      title="Time Series Statistics"
      extra={
        <Space>
          <span>Granularity:</span>
          <Select
            value={granularity}
            onChange={setGranularity}
            style={{ width: 120 }}
            options={[
              { label: 'Hour', value: 'hour' },
              { label: 'Day', value: 'day' },
              { label: 'Week', value: 'week' },
              { label: 'Month', value: 'month' },
            ]}
          />
        </Space>
      }
    >
      <Spin spinning={loading}>
        {chartData.length > 0 ? (
          <ResponsiveContainer width="100%" height={400}>
            <LineChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey="time"
                angle={-45}
                textAnchor="end"
                height={80}
                interval="preserveStartEnd"
              />
              <YAxis />
              <Tooltip
                formatter={(value: number, name: string) => [
                  typeof value === 'number' ? value.toLocaleString() : value,
                  name,
                ]}
                labelFormatter={(label) => `Time: ${label}`}
              />
              <Legend />
              <Line
                type="monotone"
                dataKey="Total Calls"
                stroke="#1890ff"
                strokeWidth={2}
                dot={{ r: 4 }}
                activeDot={{ r: 6 }}
              />
              <Line
                type="monotone"
                dataKey="Success Calls"
                stroke="#52c41a"
                strokeWidth={2}
                dot={{ r: 4 }}
                activeDot={{ r: 6 }}
              />
              <Line
                type="monotone"
                dataKey="Error Calls"
                stroke="#ff4d4f"
                strokeWidth={2}
                dot={{ r: 4 }}
                activeDot={{ r: 6 }}
              />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <div style={{ textAlign: 'center', padding: '40px' }}>
            No data available
          </div>
        )}
      </Spin>
    </Card>
  )
}

export default TimeSeriesChart

