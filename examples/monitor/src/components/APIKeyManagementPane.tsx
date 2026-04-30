import React, { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { CopyOutlined, EyeInvisibleOutlined, EyeOutlined, PlusOutlined } from '@ant-design/icons'
import { consoleAPIKeyApi, consoleTeamApi, consoleUserApi } from '../services/api'
import type {
  APIKeyCreate,
  APIKeyRead,
  APIKeyUpdate,
  ConsoleUser,
  ParameterLimits,
  TeamRead,
} from '../services/types'
import { getApiErrorMessage } from '../utils/errorUtils'

const { Text } = Typography
const COMMON_LIMIT_KEYS = ['max_tokens', 'temperature', 'top_p', 'frequency_penalty', 'presence_penalty'] as const

type APIKeyFormValues = {
  name?: string
  key?: string
  is_active?: boolean
  owner_type?: string
  owner_id?: number
  expires_at?: dayjs.Dayjs
  quota_tokens_monthly?: number
  ip_allowlist?: string[]
  allowed_models?: string[]
  allowed_providers?: string[]
  parameter_limits_common?: Record<string, number | null>
  parameter_limits_extra_json?: string
}

const maskApiKey = (key?: string | null): string => {
  if (!key) return '—'
  if (key.length <= 8) return '********'
  return `${key.slice(0, 4)}••••${key.slice(-4)}`
}

const APIKeyManagementPane: React.FC = () => {
  const [items, setItems] = useState<APIKeyRead[]>([])
  const [users, setUsers] = useState<ConsoleUser[]>([])
  const [teams, setTeams] = useState<TeamRead[]>([])
  const [loading, setLoading] = useState(false)
  const [includeInactive, setIncludeInactive] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [visibleKeys, setVisibleKeys] = useState<Set<number>>(new Set())
  const [formVisible, setFormVisible] = useState(false)
  const [formMode, setFormMode] = useState<'create' | 'edit'>('create')
  const [editingItem, setEditingItem] = useState<APIKeyRead | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm<APIKeyFormValues>()

  const loadAPIKeys = async () => {
    setLoading(true)
    try {
      const data = await consoleAPIKeyApi.list(includeInactive)
      setItems(data)
    } catch (error) {
      message.error(getApiErrorMessage(error, '加载 API Key 失败'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAPIKeys()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [includeInactive])

  useEffect(() => {
    Promise.allSettled([consoleUserApi.list(), consoleTeamApi.list()]).then(([usersResult, teamsResult]) => {
      if (usersResult.status === 'fulfilled') {
        setUsers(usersResult.value)
      }
      if (teamsResult.status === 'fulfilled') {
        setTeams(teamsResult.value)
      }
    })
  }, [])

  const filteredItems = useMemo(() => {
    if (!keyword.trim()) return items
    const lower = keyword.toLowerCase()
    return items.filter((item) => {
      const name = item.name?.toLowerCase() ?? ''
      const key = item.key?.toLowerCase() ?? ''
      const ownerType = item.owner_type?.toLowerCase() ?? ''
      return name.includes(lower) || key.includes(lower) || ownerType.includes(lower) || String(item.id).includes(lower)
    })
  }, [items, keyword])

  const parseExtraJSON = (value?: string): Record<string, any> => {
    if (!value || !value.trim()) return {}
    const parsed = JSON.parse(value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('高级参数必须是 JSON 对象')
    }
    return parsed as Record<string, any>
  }

  const buildParameterLimits = (values: APIKeyFormValues): ParameterLimits | undefined => {
    const common = values.parameter_limits_common ?? {}
    const out: ParameterLimits = {}
    COMMON_LIMIT_KEYS.forEach((key) => {
      const val = common[key]
      if (val != null) out[key] = val
    })
    const extra = parseExtraJSON(values.parameter_limits_extra_json)
    Object.assign(out, extra)
    return Object.keys(out).length > 0 ? out : undefined
  }

  const openCreate = () => {
    setFormMode('create')
    setEditingItem(null)
    form.resetFields()
    form.setFieldsValue({
      owner_type: 'system',
      ip_allowlist: [],
      allowed_models: [],
      allowed_providers: [],
      parameter_limits_common: {},
      parameter_limits_extra_json: '',
    })
    setFormVisible(true)
  }

  const openEdit = (item: APIKeyRead) => {
    setFormMode('edit')
    setEditingItem(item)
    const limits = item.parameter_limits ?? {}
    const common: Record<string, number | null> = {}
    COMMON_LIMIT_KEYS.forEach((key) => {
      const val = limits[key]
      common[key] = typeof val === 'number' ? val : null
    })
    const extra = { ...limits }
    COMMON_LIMIT_KEYS.forEach((key) => delete extra[key])
    form.setFieldsValue({
      name: item.name ?? '',
      is_active: item.is_active,
      owner_type: item.owner_type ?? 'system',
      owner_id: item.owner_id ?? undefined,
      expires_at: item.expires_at ? dayjs(item.expires_at) : undefined,
      quota_tokens_monthly: item.quota_tokens_monthly ?? undefined,
      ip_allowlist: item.ip_allowlist ?? [],
      allowed_models: item.allowed_models ?? [],
      allowed_providers: item.allowed_providers ?? [],
      parameter_limits_common: common,
      parameter_limits_extra_json: Object.keys(extra).length > 0 ? JSON.stringify(extra, null, 2) : '',
    })
    setFormVisible(true)
  }

  const handleToggleVisible = (id: number) => {
    setVisibleKeys((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const handleCopy = async (value?: string | null) => {
    if (!value) return
    try {
      await navigator.clipboard.writeText(value)
      message.success('已复制到剪贴板')
    } catch (error) {
      message.error(getApiErrorMessage(error, '复制失败'))
    }
  }

  const handleDisable = async (id: number) => {
    try {
      await consoleAPIKeyApi.disable(id)
      message.success('API Key 已禁用')
      await loadAPIKeys()
    } catch (error) {
      message.error(getApiErrorMessage(error, '禁用 API Key 失败'))
    }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      const ownerType = values.owner_type || 'system'
      const payloadBase = {
        name: values.name?.trim() || undefined,
        owner_type: ownerType,
        owner_id: ownerType === 'system' ? undefined : values.owner_id,
        expires_at: values.expires_at ? values.expires_at.toISOString() : undefined,
        quota_tokens_monthly: values.quota_tokens_monthly ?? undefined,
        ip_allowlist: values.ip_allowlist ?? [],
        allowed_models: values.allowed_models ?? [],
        allowed_providers: values.allowed_providers ?? [],
        parameter_limits: buildParameterLimits(values),
      }
      setSubmitting(true)
      if (formMode === 'create') {
        const payload: APIKeyCreate = {
          ...payloadBase,
          key: values.key?.trim() || undefined,
        }
        const created = await consoleAPIKeyApi.create(payload)
        message.success('API Key 创建成功')
        if (created.key) {
          Modal.info({
            title: '请保存新创建的 API Key',
            content: (
              <Space direction="vertical" size="small">
                <Text copyable>{created.key}</Text>
                <Text type="secondary">默认已脱敏展示，可在列表中临时查看与复制。</Text>
              </Space>
            ),
          })
        }
      } else if (editingItem) {
        const payload: APIKeyUpdate = {
          ...payloadBase,
          is_active: values.is_active,
        }
        await consoleAPIKeyApi.update(editingItem.id, payload)
        message.success('API Key 更新成功')
      }
      setFormVisible(false)
      form.resetFields()
      await loadAPIKeys()
    } catch (error) {
      if (error instanceof Error && error.message.includes('JSON')) {
        message.error(error.message)
      } else {
        message.error(getApiErrorMessage(error, '保存 API Key 失败'))
      }
    } finally {
      setSubmitting(false)
    }
  }

  const ownerLabel = (item: APIKeyRead) => {
    if (!item.owner_type || item.owner_type === 'system') return 'System'
    if (item.owner_type === 'user') {
      const user = users.find((entry) => entry.id === item.owner_id)
      return user ? `User · ${user.display_name || user.email}` : `User #${item.owner_id}`
    }
    if (item.owner_type === 'team') {
      const team = teams.find((entry) => entry.id === item.owner_id)
      return team ? `Team · ${team.name}` : `Team #${item.owner_id}`
    }
    return `${item.owner_type}#${item.owner_id ?? '—'}`
  }

  const columns: ColumnsType<APIKeyRead> = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 72,
    },
    {
      title: '名称',
      dataIndex: 'name',
      render: (value: string | null | undefined) => value || '—',
    },
    {
      title: 'API Key',
      dataIndex: 'key',
      width: 280,
      render: (value: string | null | undefined, record) => (
        <Space size="small">
          <Text code>{visibleKeys.has(record.id) ? value || '—' : maskApiKey(value)}</Text>
          {value && (
            <>
              <Button
                type="text"
                size="small"
                icon={visibleKeys.has(record.id) ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                onClick={() => handleToggleVisible(record.id)}
              />
              <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => handleCopy(value)} />
            </>
          )}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'is_active',
      width: 100,
      render: (active: boolean) => (active ? <Tag color="success">启用</Tag> : <Tag>禁用</Tag>),
    },
    {
      title: '归属',
      width: 210,
      render: (_, record) => (
        <Tag color={record.owner_type === 'team' ? 'geekblue' : record.owner_type === 'user' ? 'gold' : 'default'}>
          {ownerLabel(record)}
        </Tag>
      ),
    },
    {
      title: '过期时间',
      dataIndex: 'expires_at',
      width: 190,
      render: (value: string | null | undefined) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '—'),
    },
    {
      title: '月额度(Tokens)',
      dataIndex: 'quota_tokens_monthly',
      width: 170,
      render: (value: number | null | undefined) => (value != null ? value.toLocaleString() : '—'),
    },
    {
      title: '操作',
      width: 160,
      render: (_, record) => (
        <Space>
          <Button size="small" onClick={() => openEdit(record)}>
            编辑
          </Button>
          {record.is_active && (
            <Popconfirm title="确认禁用此 API Key？" onConfirm={() => handleDisable(record.id)}>
              <Button size="small" danger>
                禁用
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="控制台 API Key 现支持 system、user、team 三类归属；个人与团队归属会参与钱包校验与调用扣费。"
      />
      <div className="api-key-toolbar">
        <Input
          value={keyword}
          allowClear
          placeholder="按名称/Key/ID/归属搜索..."
          onChange={(e) => setKeyword(e.target.value)}
          className="api-key-search"
        />
        <Space>
          <span>显示禁用项</span>
          <Switch checked={includeInactive} onChange={setIncludeInactive} />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新建 API Key
          </Button>
        </Space>
      </div>
      <div className="api-key-table-wrap">
        <Table rowKey="id" columns={columns} dataSource={filteredItems} loading={loading} pagination={{ pageSize: 12 }} />
      </div>

      <Modal
        title={formMode === 'create' ? '新建 API Key' : '编辑 API Key'}
        open={formVisible}
        onCancel={() => setFormVisible(false)}
        onOk={handleSubmit}
        confirmLoading={submitting}
        width={860}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称">
            <Input placeholder="例如：Prod Web App Key" />
          </Form.Item>
          {formMode === 'create' && (
            <Form.Item name="key" label="Key（可选）" extra="留空将由后端自动生成。">
              <Input.Password placeholder="sk-..." />
            </Form.Item>
          )}
          {formMode === 'edit' && (
            <Form.Item name="is_active" label="状态" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="禁用" />
            </Form.Item>
          )}
          <Form.Item name="owner_type" label="归属类型" initialValue="system">
            <Select
              options={[
                { label: '系统', value: 'system' },
                { label: '个人', value: 'user' },
                { label: '团队', value: 'team' },
              ]}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.owner_type !== curr.owner_type}>
            {({ getFieldValue }) => {
              const ownerType = getFieldValue('owner_type')
              if (ownerType === 'user') {
                return (
                  <Form.Item name="owner_id" label="归属用户" rules={[{ required: true, message: '请选择用户' }]}>
                    <Select
                      showSearch
                      placeholder="选择用户"
                      options={users.map((item) => ({
                        label: `${item.display_name || item.email} (${item.email})`,
                        value: item.id,
                      }))}
                    />
                  </Form.Item>
                )
              }
              if (ownerType === 'team') {
                return (
                  <Form.Item name="owner_id" label="归属团队" rules={[{ required: true, message: '请选择团队' }]}>
                    <Select
                      showSearch
                      placeholder="选择团队"
                      options={teams.map((item) => ({
                        label: `${item.name} (${item.slug})`,
                        value: item.id,
                      }))}
                    />
                  </Form.Item>
                )
              }
              return null
            }}
          </Form.Item>
          <Form.Item name="expires_at" label="过期时间">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="quota_tokens_monthly" label="月额度（Tokens）">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="例如：1000000" />
          </Form.Item>
          <Form.Item name="ip_allowlist" label="IP 白名单">
            <Select mode="tags" tokenSeparators={[',']} placeholder="输入 IP/CIDR 后回车" />
          </Form.Item>
          <Form.Item name="allowed_providers" label="允许 Provider">
            <Select mode="tags" tokenSeparators={[',']} placeholder="输入 provider 名称后回车" />
          </Form.Item>
          <Form.Item name="allowed_models" label="允许模型">
            <Select mode="tags" tokenSeparators={[',']} placeholder="输入 provider/model 或 model 后回车" />
          </Form.Item>

          <Typography.Title level={5}>参数上限（常用字段）</Typography.Title>
          <Space size="middle" wrap style={{ width: '100%' }}>
            <Form.Item name={['parameter_limits_common', 'max_tokens']} label="max_tokens">
              <InputNumber min={0} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name={['parameter_limits_common', 'temperature']} label="temperature">
              <InputNumber min={0} max={2} step={0.1} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name={['parameter_limits_common', 'top_p']} label="top_p">
              <InputNumber min={0} max={1} step={0.05} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name={['parameter_limits_common', 'frequency_penalty']} label="frequency_penalty">
              <InputNumber min={-2} max={2} step={0.1} style={{ width: 180 }} />
            </Form.Item>
            <Form.Item name={['parameter_limits_common', 'presence_penalty']} label="presence_penalty">
              <InputNumber min={-2} max={2} step={0.1} style={{ width: 180 }} />
            </Form.Item>
          </Space>

          <Form.Item
            name="parameter_limits_extra_json"
            label="高级 JSON 扩展（可选）"
            extra="用于补充自定义策略字段，必须是 JSON 对象。"
          >
            <Input.TextArea rows={5} placeholder='例如：{"custom_limits":{"max_prompt_tokens":4096}}' />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}

export default APIKeyManagementPane
