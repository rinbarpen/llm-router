import React, { useState, useEffect } from 'react'
import { Table, Tag, Select, Space, message, Alert } from 'antd'
import dayjs from 'dayjs'
import { loginRecordApi } from '../services/api'
import type { LoginRecord } from '../services/types'

const LoginRecordList: React.FC = () => {
  const [records, setRecords] = useState<LoginRecord[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [redisAvailable, setRedisAvailable] = useState<boolean | undefined>(undefined)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [authTypeFilter, setAuthTypeFilter] = useState<string | undefined>(undefined)
  const [successFilter, setSuccessFilter] = useState<boolean | undefined>(undefined)

  const loadRecords = async () => {
    setLoading(true)
    try {
      const offset = (page - 1) * pageSize
      const result = await loginRecordApi.getLoginRecords({
        limit: pageSize,
        offset,
        auth_type: authTypeFilter,
        is_success: successFilter,
      })
      setRecords(result.records)
      setTotal(result.total)
      setRedisAvailable(result.redis_available)
    } catch (error) {
      console.error('Failed to load login records:', error)
      message.error('加载登录记录失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadRecords()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, authTypeFilter, successFilter])

  const columns = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (text: string) => dayjs(text).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: 'IP 地址',
      dataIndex: 'ip_address',
      key: 'ip_address',
      width: 140,
    },
    {
      title: '认证方式',
      dataIndex: 'auth_type',
      key: 'auth_type',
      width: 120,
      render: (auth_type: string) => {
        const map: Record<string, string> = {
          api_key: 'API Key',
          session_token: 'Session',
          none: '免认证',
        }
        return map[auth_type] ?? auth_type
      },
    },
    {
      title: '状态',
      dataIndex: 'is_success',
      key: 'is_success',
      width: 90,
      render: (is_success: boolean) =>
        is_success ? (
          <Tag color="success">成功</Tag>
        ) : (
          <Tag color="error">失败</Tag>
        ),
    },
    {
      title: '本地',
      dataIndex: 'is_local',
      key: 'is_local',
      width: 70,
      render: (is_local: boolean) => (is_local ? '是' : '否'),
    },
    {
      title: 'API Key ID',
      dataIndex: 'api_key_id',
      key: 'api_key_id',
      width: 100,
      render: (v: number | null) => (v != null ? String(v) : '—'),
    },
  ]

  return (
    <div className="login-record-list">
      {redisAvailable === false && (
        <Alert
          className="login-record-alert"
          type="warning"
          showIcon
          message="登录记录暂不可用"
          description="请检查 Redis 是否已启动，并确认已配置 LLM_ROUTER_REDIS_URL（默认 redis://localhost:6379/0）。"
        />
      )}
      <div className="login-record-filters-wrap">
        <Space className="login-record-filters" wrap>
          <span>认证方式:</span>
          <Select
            placeholder="全部"
            allowClear
            className="login-record-filter-auth"
            value={authTypeFilter}
            onChange={setAuthTypeFilter}
            options={[
              { label: 'API Key', value: 'api_key' },
              { label: 'Session', value: 'session_token' },
              { label: '免认证', value: 'none' },
            ]}
          />
          <span>状态:</span>
          <Select
            placeholder="全部"
            allowClear
            className="login-record-filter-status"
            value={successFilter}
            onChange={setSuccessFilter}
            options={[
              { label: '成功', value: true },
              { label: '失败', value: false },
            ]}
          />
        </Space>
      </div>
      <div className="login-record-table-wrap">
        <Table
          rowKey="id"
          columns={columns}
          dataSource={records}
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, size) => {
              setPage(p)
              setPageSize(size ?? 20)
            },
          }}
          size="small"
        />
      </div>
    </div>
  )
}

export default LoginRecordList
