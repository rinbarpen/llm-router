import React, { useState, useEffect } from 'react'
import { Table, Tag, Button, Space, Input, Select, message } from 'antd'
import { EyeOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { dbService } from '../services/dbService'
import InvocationDetail from './InvocationDetail'
import type { InvocationRead, InvocationQuery, InvocationStatus } from '../services/types'

interface InvocationListProps {
  startTime?: Date
  endTime?: Date
}

const InvocationList: React.FC<InvocationListProps> = ({ startTime, endTime }) => {
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
    start_time: startTime,
    end_time: endTime,
  })

  const loadInvocations = async () => {
    setLoading(true)
    try {
      const result = await dbService.getInvocations(query)
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
    setQuery(prev => ({
      ...prev,
      start_time: startTime,
      end_time: endTime,
    }))
  }, [startTime, endTime])

  useEffect(() => {
    loadInvocations()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query])

  const handleViewDetail = async (id: number) => {
    try {
      const invocation = await dbService.getInvocationById(id)
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
      title: '时间戳',
      dataIndex: 'started_at',
      key: 'started_at',
      width: 180,
      render: (text: string) => dayjs(text).format('MMM D, h:mm A'),
      sorter: true,
    },
    {
      title: 'Provider / Model',
      key: 'model',
      render: (_: any, record: InvocationRead) => (
        <span>
          <Tag color="blue">{record.provider_name}</Tag>
          {record.model_name}
        </span>
      ),
    },
    {
      title: 'App',
      key: 'app',
      width: 100,
      render: () => <span style={{ color: '#999' }}>Unknown</span>,
    },
    {
      title: 'Token',
      key: 'tokens',
      width: 150,
      render: (_: any, record: InvocationRead) => {
        if (record.prompt_tokens !== null && record.completion_tokens !== null) {
          return (
            <span>
              {record.prompt_tokens.toLocaleString()} → {record.completion_tokens.toLocaleString()}
            </span>
          )
        }
        if (record.total_tokens) {
          return <span>{record.total_tokens.toLocaleString()}</span>
        }
        return '-'
      },
    },
    {
      title: '成本',
      dataIndex: 'cost',
      key: 'cost',
      width: 100,
      render: (cost: number | null) => {
        if (cost !== null && cost !== undefined) {
          return <span>${cost.toFixed(6)}</span>
        }
        return '-'
      },
      sorter: true,
    },
    {
      title: '速度',
      key: 'speed',
      width: 100,
      render: (_: any, record: InvocationRead) => {
        // 计算tps: tokens per second
        if (record.completion_tokens && record.duration_ms) {
          const tps = (record.completion_tokens / (record.duration_ms / 1000)).toFixed(1)
          return <span>{tps} tps</span>
        }
        return '-'
      },
    },
    {
      title: '完成',
      dataIndex: 'status',
      key: 'finish',
      width: 80,
      render: (status: InvocationStatus) => {
        if (status === 'success') {
          return <Tag color="green">stop</Tag>
        }
        return <Tag color="red">error</Tag>
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

