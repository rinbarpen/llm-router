import React, { useState, useEffect, useMemo } from 'react'
import {
  Space,
  Input,
  Button,
  Collapse,
  Tag,
  Tooltip,
  message,
  Card,
  Row,
  Col,
  Typography,
  Empty,
} from 'antd'
import {
  SearchOutlined,
  PlusOutlined,
  EditOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  GlobalOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import type { ModelRead, ModelCreate, ModelUpdate, ProviderRead } from '../services/types'
import { modelApi } from '../services/api'
import ModelForm from './ModelForm'

const { Panel } = Collapse
const { Text } = Typography

interface GroupedModels {
  [providerName: string]: ModelRead[]
}

const ModelManagement: React.FC = () => {
  const [models, setModels] = useState<ModelRead[]>([])
  const [providers, setProviders] = useState<ProviderRead[]>([])
  const [loading, setLoading] = useState(false)
  const [searchText, setSearchText] = useState('')
  const [formVisible, setFormVisible] = useState(false)
  const [formMode, setFormMode] = useState<'create' | 'edit'>('create')
  const [editingModel, setEditingModel] = useState<ModelRead | undefined>()

  // 加载数据
  const loadData = async () => {
    setLoading(true)
    try {
      const [modelsData, providersData] = await Promise.all([
        modelApi.getModels(),
        modelApi.getProviders(),
      ])
      setModels(modelsData)
      setProviders(providersData)
    } catch (error) {
      console.error('Failed to load data:', error)
      message.error('加载数据失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  // 过滤模型
  const filteredModels = useMemo(() => {
    if (!searchText.trim()) {
      return models
    }
    const lowerSearch = searchText.toLowerCase()
    return models.filter(
      (model) =>
        model.name.toLowerCase().includes(lowerSearch) ||
        model.provider_name.toLowerCase().includes(lowerSearch) ||
        (model.display_name && model.display_name.toLowerCase().includes(lowerSearch)) ||
        model.tags.some((tag) => tag.toLowerCase().includes(lowerSearch))
    )
  }, [models, searchText])

  // 按Provider分组
  const groupedModels = useMemo(() => {
    const grouped: GroupedModels = {}
    filteredModels.forEach((model) => {
      if (!grouped[model.provider_name]) {
        grouped[model.provider_name] = []
      }
      grouped[model.provider_name].push(model)
    })
    return grouped
  }, [filteredModels])

  // 处理添加模型
  const handleAdd = () => {
    setFormMode('create')
    setEditingModel(undefined)
    setFormVisible(true)
  }

  // 处理编辑模型
  const handleEdit = (model: ModelRead) => {
    setFormMode('edit')
    setEditingModel(model)
    setFormVisible(true)
  }

  // 处理表单提交
  const handleSubmit = async (values: ModelCreate | ModelUpdate) => {
    try {
      if (formMode === 'create') {
        const createValues = values as ModelCreate
        if (!createValues.provider_name || !createValues.name) {
          message.error('Provider和模型名称是必填项')
          return
        }
        await modelApi.createModel(createValues)
        message.success('模型创建成功')
      } else {
        if (!editingModel) {
          message.error('编辑模型信息缺失')
          return
        }
        await modelApi.updateModel(editingModel.provider_name, editingModel.name, values as ModelUpdate)
        message.success('模型更新成功')
      }
      setFormVisible(false)
      await loadData()
    } catch (error: any) {
      console.error('Failed to submit:', error)
      const errorMessage = error.response?.data?.detail || error.message || '操作失败'
      message.error(errorMessage)
    }
  }

  // 获取Provider显示名称
  const getProviderDisplayName = (providerName: string) => {
    const provider = providers.find((p) => p.name === providerName)
    return provider ? `${provider.name} (${provider.type})` : providerName
  }

  // 渲染模型卡片
  const renderModelCard = (model: ModelRead) => {
    return (
      <Card
        key={`${model.provider_name}-${model.name}`}
        size="small"
        style={{ marginBottom: 8 }}
        actions={[
          <Tooltip title="编辑模型" key="edit">
            <EditOutlined onClick={() => handleEdit(model)} />
          </Tooltip>,
        ]}
      >
        <Row gutter={[8, 8]}>
          <Col span={24}>
            <Space>
              <Text strong>{model.display_name || model.name}</Text>
              {model.is_active === false && (
                <Tag color="red">未激活</Tag>
              )}
              {model.is_active !== false && (
                <Tag color="green">激活</Tag>
              )}
            </Space>
          </Col>
          <Col span={24}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {model.provider_name}/{model.name}
            </Text>
          </Col>
          {model.description && (
            <Col span={24}>
              <Text style={{ fontSize: 12 }}>{model.description}</Text>
            </Col>
          )}
          {model.tags && model.tags.length > 0 && (
            <Col span={24}>
              <Space wrap size={[4, 4]}>
                {model.tags.map((tag) => (
                  <Tag key={tag} color="blue" style={{ margin: 0 }}>
                    {tag}
                  </Tag>
                ))}
              </Space>
            </Col>
          )}
          <Col span={24}>
            <Space size="small">
              {model.config?.supports_vision && (
                <Tooltip title="支持视觉">
                  <EyeOutlined style={{ color: '#1890ff' }} />
                </Tooltip>
              )}
              {model.config?.supports_tools && (
                <Tooltip title="支持工具调用">
                  <SettingOutlined style={{ color: '#1890ff' }} />
                </Tooltip>
              )}
              {model.rate_limit && (
                <Tooltip
                  title={`速率限制: ${model.rate_limit.max_requests} 请求/${model.rate_limit.per_seconds}秒`}
                >
                  <GlobalOutlined style={{ color: '#1890ff' }} />
                </Tooltip>
              )}
            </Space>
          </Col>
        </Row>
      </Card>
    )
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%' }} direction="vertical" size="middle">
        <Row gutter={16}>
          <Col flex="auto">
            <Input
              placeholder="搜索模型名称、Provider或标签..."
              prefix={<SearchOutlined />}
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              allowClear
              style={{ width: '100%' }}
            />
          </Col>
          <Col>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
              添加模型
            </Button>
          </Col>
        </Row>
      </Space>

      {Object.keys(groupedModels).length === 0 ? (
        <Empty description={loading ? '加载中...' : '暂无模型数据'} />
      ) : (
        <Collapse defaultActiveKey={Object.keys(groupedModels)}>
          {Object.entries(groupedModels)
            .sort(([a], [b]) => a.localeCompare(b))
            .map(([providerName, providerModels]) => (
              <Panel
                key={providerName}
                header={
                  <Space>
                    <Text strong>{getProviderDisplayName(providerName)}</Text>
                    <Tag>{providerModels.length} 个模型</Tag>
                  </Space>
                }
              >
                <div>
                  {providerModels.map((model) => renderModelCard(model))}
                </div>
              </Panel>
            ))}
        </Collapse>
      )}

      <ModelForm
        visible={formVisible}
        mode={formMode}
        model={editingModel}
        providers={providers}
        onCancel={() => {
          setFormVisible(false)
          setEditingModel(undefined)
        }}
        onSubmit={handleSubmit}
      />
    </div>
  )
}

export default ModelManagement
