import React, { useState, useEffect } from 'react'
import { Row, Col, Tabs, Select, Space, Button, message, Dropdown } from 'antd'
import { ReloadOutlined, DownloadOutlined, FileExcelOutlined, FileTextOutlined } from '@ant-design/icons'
import StatisticsPanel from './StatisticsPanel'
import InvocationList from './InvocationList'
import TimeSeriesChart from './TimeSeriesChart'
import ActivityDashboard from './ActivityDashboard'
import { monitorApi } from '../services/api'
import { dbService } from '../services/dbService'
import type { StatisticsResponse } from '../services/types'

const MonitorDashboard: React.FC = () => {
  const [statistics, setStatistics] = useState<StatisticsResponse | null>(null)
  const [timeRange, setTimeRange] = useState<number>(24)
  const [loading, setLoading] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)

  const loadStatistics = async () => {
    setLoading(true)
    try {
      const data = await dbService.getStatistics(timeRange, 10)
      setStatistics(data)
    } catch (error) {
      console.error('Failed to load statistics:', error)
      message.error('加载统计数据失败')
    } finally {
      setLoading(false)
    }
  }

  const handleExportJSON = async () => {
    setLoading(true)
    try {
      const blob = await monitorApi.exportJSON(timeRange)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      link.download = `llm_router_export_${timestamp}.json`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('JSON导出成功')
    } catch (error) {
      console.error('Failed to export JSON:', error)
      message.error('JSON导出失败')
    } finally {
      setLoading(false)
    }
  }

  const handleExportExcel = async () => {
    setLoading(true)
    try {
      const blob = await monitorApi.exportExcel(timeRange)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      link.download = `llm_router_export_${timestamp}.csv`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('CSV导出成功')
    } catch (error) {
      console.error('Failed to export CSV:', error)
      message.error('CSV导出失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDownloadDatabase = async () => {
    setLoading(true)
    try {
      const blob = await monitorApi.downloadDatabase()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = 'llm_router.db'
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      message.success('数据库文件下载成功')
    } catch (error) {
      console.error('Failed to download database:', error)
      message.error('数据库文件下载失败')
    } finally {
      setLoading(false)
    }
  }

  const exportMenu = (
    <Dropdown.Menu>
      <Dropdown.Item
        key="json"
        icon={<FileTextOutlined />}
        onClick={handleExportJSON}
      >
        导出为JSON
      </Dropdown.Item>
      <Dropdown.Item
        key="excel"
        icon={<FileExcelOutlined />}
        onClick={handleExportExcel}
      >
        导出为Excel
      </Dropdown.Item>
      <Dropdown.Item
        key="database"
        icon={<DownloadOutlined />}
        onClick={handleDownloadDatabase}
      >
        下载数据库
      </Dropdown.Item>
    </Dropdown.Menu>
  )

  useEffect(() => {
    loadStatistics()
  }, [timeRange])

  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      loadStatistics()
    }, 10000) // 每10秒刷新一次（从数据库读取，适当频率）

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
            <Dropdown.Button
              type="default"
              loading={loading}
              icon={<DownloadOutlined />}
            >
              导出数据
              {exportMenu}
            </Dropdown.Button>
          </Space>
        </Col>
      </Row>

      <Tabs
        defaultActiveKey="activity"
        items={[
          {
            key: 'activity',
            label: 'Your Activity',
            children: <ActivityDashboard />,
          },
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
