import React, { useState } from 'react'
import { Row, Col, Tabs, Select, Space, message, Dropdown } from 'antd'
import { DownloadOutlined, FileExcelOutlined, FileTextOutlined } from '@ant-design/icons'
import InvocationList from './InvocationList'
import ActivityDashboard from './ActivityDashboard'
import { monitorApi } from '../services/api'

const MonitorDashboard: React.FC = () => {
  const [timeRange, setTimeRange] = useState<number>(24)
  const [loading, setLoading] = useState(false)

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

  const menuItems = [
    {
      key: 'json',
      icon: <FileTextOutlined />,
      label: '导出为JSON',
      onClick: handleExportJSON,
    },
    {
      key: 'excel',
      icon: <FileExcelOutlined />,
      label: '导出为Excel',
      onClick: handleExportExcel,
    },
    {
      key: 'database',
      icon: <DownloadOutlined />,
      label: '下载数据库',
      onClick: handleDownloadDatabase,
    },
  ]


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
            <Dropdown.Button
              type="default"
              loading={loading}
              icon={<DownloadOutlined />}
              menu={{ items: menuItems }}
            >
              导出数据
            </Dropdown.Button>
          </Space>
        </Col>
      </Row>

      <Tabs
        defaultActiveKey="activity"
        items={[
          {
            key: 'activity',
            label: '活动概览',
            children: <ActivityDashboard />,
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
