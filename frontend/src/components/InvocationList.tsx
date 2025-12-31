import React, { useState, useEffect } from 'react'
import { Table, Tag, Button, Space, Input, Select, DatePicker, message } from 'antd'
import { EyeOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { monitorApi } from '../services/api'
import InvocationDetail from './InvocationDetail'
import type { InvocationRead, InvocationQuery, InvocationStatus } from '../services/types'

const InvocationList: React.FC = () => {
  const [invocations, setInvocations] = useState<InvocationRead[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [selectedInvocation, setSelectedInvocation] = useState<InvocationRead | null>(null)
  const [detailVisible, setDetailVisible] = useState(false)
  
  const [query, setQuery] = useState<Partial<InvocationQuery>>({
    limit: 50,
    offset: 0,
    order_by: 'started_at',
    order_desc: true,
  })

  const loadInvocations = async () => {
    setLoading(true)
    try {
      const result = await monitorApi.getInvocations(query)
      setInvocations(result.items)
      setTotal(result.total)
    } catch (error) {
      console.error('Failed to load invocations:', error)
      message.error('加载调用历史失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadInvocations()
  }, [query])

  const handleViewDetail = async (id: number) => {
    try {
      const invocation = await monitorApi.getInvocationById(id)
      setSelectedInvocation(invocation)
      setDetailVisible(true)
    } catch (error) {
      message.error('加载详情失败')
    }
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '时间',
      dataIndex: 'started_at',
      key: 'started_at',
      width: 180,
      render: (text: string) => dayjs(text).format('YYYY-MM-DD HH:mm:ss'),
      sorter: true,
    },
    {
      title: '模型',
      key: 'model',
      render: (_: any, record: InvocationRead) => (
        <span>
          <Tag color="blue">{record.provider_name}</Tag>
          {record.model_name}
        </span>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: InvocationStatus) => (
        <Tag color={status === 'success' ? 'green' : 'red'}>
          {status === 'success' ? '成功' : '失败'}
        </Tag>
      ),
      filters: [
        { text: '成功', value: 'success' },
        { text: '失败', value: 'error' },
      ],
    },
    {
      title: '延迟',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 100,
      render: (ms: number | null) => (ms ? `${ms.toFixed(0)}ms` : '-'),
      sorter: true,
    },
    {
      title: 'Token',
      key: 'tokens',
      width: 120,
      render: (_: any, record: InvocationRead) => {
        if (record.total_tokens) {
          return (
            <span>
              {record.total_tokens.toLocaleString()}
              {record.prompt_tokens && record.completion_tokens && (
                <span style={{ color: '#999', fontSize: '12px', marginLeft: 4 }}>
                  ({record.prompt_tokens}+{record.completion_tokens})
                </span>
              )}
            </span>
          )
        }
        return '-'
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: any, record: InvocationRead) => (
        <Button
          type="link"
          icon={<EyeOutlined />}
          onClick={() => handleViewDetail(record.id)}
        >
          详情
        </Button>
      ),
    },
  ]

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%' }} wrap>
        <Input
          placeholder="模型名称"
          value={query.model_name}
          onChange={(e) => setQuery({ ...query, model_name: e.target.value || undefined, offset: 0 })}
          style={{ width: 200 }}
          allowClear
        />
        <Input
          placeholder="Provider名称"
          value={query.provider_name}
          onChange={(e) => setQuery({ ...query, provider_name: e.target.value || undefined, offset: 0 })}
          style={{ width: 200 }}
          allowClear
        />
        <Select
          placeholder="状态"
          value={query.status}
          onChange={(value) => setQuery({ ...query, status: value, offset: 0 })}
          style={{ width: 120 }}
          allowClear
          options={[
            { label: '成功', value: 'success' },
            { label: '失败', value: 'error' },
          ]}
        />
        <Button onClick={loadInvocations} loading={loading}>
          刷新
        </Button>
      </Space>

      <Table
        dataSource={invocations}
        columns={columns}
        rowKey="id"
        loading={loading}
        pagination={{
          current: (query.offset || 0) / (query.limit || 50) + 1,
          pageSize: query.limit || 50,
          total,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (page, pageSize) => {
            setQuery({
              ...query,
              offset: (page - 1) * pageSize,
              limit: pageSize,
            })
          },
        }}
      />

      <InvocationDetail
        visible={detailVisible}
        invocation={selectedInvocation}
        onClose={() => {
          setDetailVisible(false)
          setSelectedInvocation(null)
        }}
      />
    </div>
  )
}

export default InvocationList

