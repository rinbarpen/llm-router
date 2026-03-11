import React, { useState, useEffect } from 'react'
import { Card, Select, Space, Spin, Tabs } from 'antd'
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
import { dbService } from '../services/dbService'
import type { GroupedTimeSeriesResponse } from '../services/types'
import dayjs from 'dayjs'

type Granularity = 'hour' | 'day' | 'week' | 'month'
type GroupBy = 'model' | 'provider'

const GroupedTimeSeriesChart: React.FC = () => {
  const [granularity, setGranularity] = useState<Granularity>('day')
  const [groupBy, setGroupBy] = useState<GroupBy>('model')
  const [data, setData] = useState<GroupedTimeSeriesResponse | null>(null)
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
        const response = await dbService.getGroupedTimeSeries(groupBy, granularity, timeRangeHours)
        console.log('Grouped time series data loaded:', response)
        setData(response)
      } catch (error) {
        console.error('Failed to load grouped time series data:', error)
        setData(null)
      } finally {
        setLoading(false)
      }
    }
    loadData()
  }, [granularity, groupBy])

  // 按组名分组数据
  const groupedData = data?.data.reduce((acc, point) => {
    const groupName = point.group_name
    if (!acc[groupName]) {
      acc[groupName] = []
    }
    acc[groupName].push(point)
    return acc
  }, {} as Record<string, GroupedTimeSeriesResponse['data']>) || {}

  // 格式化图表数据
  const formatChartData = (points: GroupedTimeSeriesResponse['data']) => {
    return points.map((point) => {
      // 处理时间戳，支持字符串和Date对象
      let timestamp: dayjs.Dayjs
      if (typeof point.timestamp === 'string') {
        timestamp = dayjs(point.timestamp)
      } else {
        timestamp = dayjs(point.timestamp)
      }
      
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
        timestamp: timestamp.toISOString(),
        'Total Tokens': point.total_tokens || 0,
        'Prompt Tokens': point.prompt_tokens || 0,
        'Completion Tokens': point.completion_tokens || 0,
      }
    })
  }

  // 获取所有组名
  const groupNames = Object.keys(groupedData).sort()

  // 为每个组生成不同的颜色
  const getColor = (index: number) => {
    const colors = [
      '#1890ff', '#52c41a', '#ff4d4f', '#722ed1', '#13c2c2',
      '#fa8c16', '#eb2f96', '#2f54eb', '#faad14', '#a0d911'
    ]
    return colors[index % colors.length]
  }

  return (
    <Card
      title={`${groupBy === 'model' ? '按模型' : '按Provider'}分组的时间序列统计`}
      extra={
        <Space>
          <span>分组方式:</span>
          <Select
            value={groupBy}
            onChange={setGroupBy}
            style={{ width: 120 }}
            options={[
              { label: '按模型', value: 'model' },
              { label: '按Provider', value: 'provider' },
            ]}
          />
          <span>粒度:</span>
          <Select
            value={granularity}
            onChange={setGranularity}
            style={{ width: 120 }}
            options={[
              { label: '小时', value: 'hour' },
              { label: '天', value: 'day' },
              { label: '周', value: 'week' },
              { label: '月', value: 'month' },
            ]}
          />
        </Space>
      }
    >
      <Spin spinning={loading}>
        {groupNames.length > 0 ? (
          <Tabs
            type="card"
            items={groupNames.map((groupName, index) => {
              const chartData = formatChartData(groupedData[groupName])
              return {
                key: groupName,
                label: groupName,
                children: (
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
                        dataKey="Total Tokens"
                        stroke={getColor(index * 3)}
                        strokeWidth={2}
                        dot={{ r: 4 }}
                        activeDot={{ r: 6 }}
                      />
                      <Line
                        type="monotone"
                        dataKey="Prompt Tokens"
                        stroke={getColor(index * 3 + 1)}
                        strokeWidth={2}
                        dot={{ r: 4 }}
                        activeDot={{ r: 6 }}
                      />
                      <Line
                        type="monotone"
                        dataKey="Completion Tokens"
                        stroke={getColor(index * 3 + 2)}
                        strokeWidth={2}
                        dot={{ r: 4 }}
                        activeDot={{ r: 6 }}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                ),
              }
            })}
          />
        ) : (
          <div style={{ textAlign: 'center', padding: '40px' }}>
            {loading ? 'Loading...' : 'No data available'}
          </div>
        )}
      </Spin>
    </Card>
  )
}

export default GroupedTimeSeriesChart

