import React, { useState, useEffect } from 'react'
import { Card, Select, Space, Spin, Row, Col } from 'antd'
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
import type { TimeSeriesResponse, GroupedTimeSeriesResponse } from '../services/types'
import dayjs from 'dayjs'

type Granularity = 'hour' | 'day' | 'week' | 'month'

const TimeSeriesChart: React.FC = () => {
  const [granularity, setGranularity] = useState<Granularity>('day')
  const [groupBy, setGroupBy] = useState<'model' | 'provider'>('model')
  const [data, setData] = useState<TimeSeriesResponse | null>(null)
  const [modelData, setModelData] = useState<GroupedTimeSeriesResponse | null>(null)
  const [providerData, setProviderData] = useState<GroupedTimeSeriesResponse | null>(null)
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
        const [overall, byModel, byProvider] = await Promise.all([
          dbService.getTimeSeries(granularity, timeRangeHours),
          dbService.getGroupedTimeSeries('model', granularity, timeRangeHours),
          dbService.getGroupedTimeSeries('provider', granularity, timeRangeHours),
        ])
        console.log('Time series data loaded:', { overall, byModel, byProvider })
        setData(overall)
        setModelData(byModel)
        setProviderData(byProvider)
      } catch (error) {
        console.error('Failed to load time series data:', error)
        setData(null)
        setModelData(null)
        setProviderData(null)
      } finally {
        setLoading(false)
      }
    }
    loadData()
  }, [granularity])

  // 格式化时间标签
  const formatTimeLabel = (timestamp: string | Date) => {
    const ts = typeof timestamp === 'string' ? dayjs(timestamp) : dayjs(timestamp)
    switch (granularity) {
      case 'hour':
        return ts.format('MM-DD HH:00')
      case 'day':
        return ts.format('MM-DD')
      case 'week':
        return ts.format('MM-DD')
      case 'month':
        return ts.format('YYYY-MM')
      default:
        return ts.format('MM-DD')
    }
  }

  // 格式化总体图表数据
  const chartData = data?.data.map((point) => ({
    time: formatTimeLabel(point.timestamp),
    timestamp: typeof point.timestamp === 'string' ? point.timestamp : dayjs(point.timestamp).toISOString(),
    'Total Calls': point.total_calls || 0,
    'Success Calls': point.success_calls || 0,
    'Error Calls': point.error_calls || 0,
    'Total Tokens': point.total_tokens || 0,
    'Prompt Tokens': point.prompt_tokens || 0,
    'Completion Tokens': point.completion_tokens || 0,
  })) || []

  // 格式化分组图表数据
  const formatGroupedChartData = (groupedData: GroupedTimeSeriesResponse | null) => {
    if (!groupedData) return {}
    
    const grouped = groupedData.data.reduce((acc, point) => {
      const groupName = point.group_name
      if (!acc[groupName]) {
        acc[groupName] = []
      }
      acc[groupName].push({
        time: formatTimeLabel(point.timestamp),
        timestamp: typeof point.timestamp === 'string' ? point.timestamp : dayjs(point.timestamp).toISOString(),
        'Total Tokens': point.total_tokens || 0,
        'Prompt Tokens': point.prompt_tokens || 0,
        'Completion Tokens': point.completion_tokens || 0,
      })
      return acc
    }, {} as Record<string, Array<{
      time: string
      timestamp: string
      'Total Tokens': number
      'Prompt Tokens': number
      'Completion Tokens': number
    }>>)
    
    return grouped
  }

  const modelGroupedData = formatGroupedChartData(modelData)
  const providerGroupedData = formatGroupedChartData(providerData)
  const modelNames = Object.keys(modelGroupedData).sort()
  const providerNames = Object.keys(providerGroupedData).sort()

  // 为每个组生成不同的颜色
  const getColor = (index: number) => {
    const colors = [
      '#1890ff', '#52c41a', '#ff4d4f', '#722ed1', '#13c2c2',
      '#fa8c16', '#eb2f96', '#2f54eb', '#faad14', '#a0d911'
    ]
    return colors[index % colors.length]
  }

  // 渲染图表组件
  type ChartDataPoint = {
    time: string
    timestamp: string
    'Total Calls': number
    'Success Calls': number
    'Error Calls': number
    'Total Tokens': number
    'Prompt Tokens': number
    'Completion Tokens': number
  }
  const renderChart = (chartData: ChartDataPoint[], showCalls = true) => (
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
        {showCalls && (
          <>
            <Line type="monotone" dataKey="Total Calls" stroke="#1890ff" strokeWidth={2} dot={{ r: 4 }} activeDot={{ r: 6 }} />
            <Line type="monotone" dataKey="Success Calls" stroke="#52c41a" strokeWidth={2} dot={{ r: 4 }} activeDot={{ r: 6 }} />
            <Line type="monotone" dataKey="Error Calls" stroke="#ff4d4f" strokeWidth={2} dot={{ r: 4 }} activeDot={{ r: 6 }} />
          </>
        )}
        <Line type="monotone" dataKey="Total Tokens" stroke="#722ed1" strokeWidth={2} dot={{ r: 4 }} activeDot={{ r: 6 }} />
        <Line type="monotone" dataKey="Prompt Tokens" stroke="#13c2c2" strokeWidth={2} dot={{ r: 4 }} activeDot={{ r: 6 }} />
        <Line type="monotone" dataKey="Completion Tokens" stroke="#fa8c16" strokeWidth={2} dot={{ r: 4 }} activeDot={{ r: 6 }} />
      </LineChart>
    </ResponsiveContainer>
  )

  // 渲染分组图表（所有组在一个图表中）
  const renderGroupedChart = (groupedData: typeof modelGroupedData) => {
    // 合并所有组的数据到同一个时间点
    const allTimes = new Set<string>()
    Object.values(groupedData).forEach(group => {
      group.forEach(point => allTimes.add(point.time))
    })
    const sortedTimes = Array.from(allTimes).sort()

    // 为每个时间点创建数据对象
    const mergedData = sortedTimes.map(time => {
      const dataPoint: Record<string, any> = { time }
      Object.entries(groupedData).forEach(([groupName, points]) => {
        const point = points.find(p => p.time === time)
        if (point) {
          dataPoint[`${groupName} - Total Tokens`] = point['Total Tokens']
          dataPoint[`${groupName} - Prompt Tokens`] = point['Prompt Tokens']
          dataPoint[`${groupName} - Completion Tokens`] = point['Completion Tokens']
        } else {
          dataPoint[`${groupName} - Total Tokens`] = 0
          dataPoint[`${groupName} - Prompt Tokens`] = 0
          dataPoint[`${groupName} - Completion Tokens`] = 0
        }
      })
      return dataPoint
    })

    const groupNames = Object.keys(groupedData).sort()
    
    return (
      <ResponsiveContainer width="100%" height={400}>
        <LineChart data={mergedData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
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
          {groupNames.flatMap((groupName, index) => [
            <Line
              key={`${groupName}-total`}
              type="monotone"
              dataKey={`${groupName} - Total Tokens`}
              stroke={getColor(index * 3)}
              strokeWidth={2}
              dot={{ r: 3 }}
              activeDot={{ r: 5 }}
            />,
            <Line
              key={`${groupName}-prompt`}
              type="monotone"
              dataKey={`${groupName} - Prompt Tokens`}
              stroke={getColor(index * 3 + 1)}
              strokeWidth={2}
              dot={{ r: 3 }}
              activeDot={{ r: 5 }}
              strokeDasharray="5 5"
            />,
            <Line
              key={`${groupName}-completion`}
              type="monotone"
              dataKey={`${groupName} - Completion Tokens`}
              stroke={getColor(index * 3 + 2)}
              strokeWidth={2}
              dot={{ r: 3 }}
              activeDot={{ r: 5 }}
              strokeDasharray="3 3"
            />,
          ])}
        </LineChart>
      </ResponsiveContainer>
    )
  }

  return (
    <div>
      <Card
        title="时间序列统计"
        extra={
          <Space>
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
          {chartData.length > 0 ? (
            renderChart(chartData, true)
          ) : !loading ? (
            <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>
              <p>暂无数据</p>
              {data && (
                <p style={{ fontSize: '12px' }}>
                  数据点数量: {data.data?.length || 0}
                  {data.data?.length === 0 && ' - 请先进行一些模型调用以生成数据'}
                </p>
              )}
              {!data && (
                <p style={{ fontSize: '12px' }}>请检查后端服务是否正常运行</p>
              )}
            </div>
          ) : null}
        </Spin>
      </Card>

      <Card
        title="Token消耗分组统计"
        extra={
          <Space>
            <span>分组方式:</span>
            <Select
              value={groupBy}
              onChange={setGroupBy}
              style={{ width: 120 }}
              options={[
                { label: '按模型', value: 'model' },
                { label: '按提供商', value: 'provider' },
              ]}
            />
          </Space>
        }
        style={{ marginTop: 16 }}
      >
            <Spin spinning={loading}>
          {groupBy === 'model' ? (
            modelNames.length > 0 ? (
                renderGroupedChart(modelGroupedData)
              ) : !loading ? (
                <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>
                  <p>暂无模型数据</p>
                </div>
            ) : null
          ) : (
            providerNames.length > 0 ? (
                renderGroupedChart(providerGroupedData)
              ) : !loading ? (
                <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>
                <p>暂无提供商数据</p>
                </div>
            ) : null
          )}
            </Spin>
          </Card>
    </div>
  )
}

export default TimeSeriesChart

