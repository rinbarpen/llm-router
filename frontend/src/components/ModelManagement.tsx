import React, { useState, useEffect, useMemo } from 'react'
import {
  Space,
  Input,
  Button,
  Card,
  Row,
  Col,
  Typography,
  Empty,
  List,
  Switch,
  Tag,
  Tooltip,
  message,
} from 'antd'
import {
  SearchOutlined,
  PlusOutlined,
  EditOutlined,
  EyeOutlined,
  SettingOutlined,
  GlobalOutlined,
  ReloadOutlined,
  LinkOutlined,
  SyncOutlined,
} from '@ant-design/icons'
import type { ModelRead, ModelCreate, ModelUpdate, ProviderRead, ProviderCreate, ProviderUpdate } from '../services/types'
import { modelApi, providerApi, configApi } from '../services/api'
import { getTagIcon } from '../utils/tagIcons'
import ModelForm from './ModelForm'
import ProviderForm from './ProviderForm'

const { Text } = Typography

const ModelManagement: React.FC = () => {
  const [providers, setProviders] = useState<ProviderRead[]>([])
  const [selectedProvider, setSelectedProvider] = useState<ProviderRead | null>(null)
  const [models, setModels] = useState<ModelRead[]>([])
  const [loading, setLoading] = useState(false)
  const [providerSearchText, setProviderSearchText] = useState('')
  const [modelSearchText, setModelSearchText] = useState('')
  const [providerFormVisible, setProviderFormVisible] = useState(false)
  const [providerFormMode, setProviderFormMode] = useState<'create' | 'edit'>('create')
  const [modelFormVisible, setModelFormVisible] = useState(false)
  const [modelFormMode, setModelFormMode] = useState<'create' | 'edit'>('create')
  const [editingModel, setEditingModel] = useState<ModelRead | undefined>()
  const [providerConfig, setProviderConfig] = useState<{ api_key?: string; base_url?: string }>({})

  // 加载Provider列表
  const loadProviders = async () => {
    try {
      const data = await providerApi.getProviders()
      setProviders(data)
    } catch (error) {
      console.error('Failed to load providers:', error)
      message.error('加载Provider列表失败')
    }
  }

  // 加载模型列表
  const loadModels = async (providerName?: string) => {
    setLoading(true)
    try {
      if (providerName) {
        // 加载特定 Provider 的模型
        const data = await modelApi.getProviderModels(providerName)
        setModels(data)
      } else {
        // 加载所有模型
        const data = await modelApi.getModels()
        setModels(data)
      }
    } catch (error) {
      console.error('Failed to load models:', error)
      message.error('加载模型列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadProviders()
    loadModels() // 自动加载所有模型
  }, [])

  useEffect(() => {
    if (selectedProvider) {
      loadModels(selectedProvider.name)
      // 初始化配置（注意：api_key可能为空，出于安全考虑）
      setProviderConfig({
        base_url: selectedProvider.base_url || '',
        api_key: '', // 不显示现有密钥
      })
    } else {
      loadModels() // 未选择时加载所有模型
    }
  }, [selectedProvider])

  // 过滤Provider列表
  const filteredProviders = useMemo(() => {
    if (!providerSearchText.trim()) {
      return providers
    }
    const lowerSearch = providerSearchText.toLowerCase()
    return providers.filter(
      (provider) =>
        provider.name.toLowerCase().includes(lowerSearch) ||
        provider.type.toLowerCase().includes(lowerSearch)
    )
  }, [providers, providerSearchText])

  // 过滤模型列表
  const filteredModels = useMemo(() => {
    if (!modelSearchText.trim()) {
      return models
    }
    const lowerSearch = modelSearchText.toLowerCase()
    return models.filter(
      (model) =>
        model.name?.toLowerCase().includes(lowerSearch) ||
        (model.display_name && model.display_name.toLowerCase().includes(lowerSearch)) ||
        (model.tags && Array.isArray(model.tags) && model.tags.some((tag) => tag && typeof tag === 'string' && tag.toLowerCase().includes(lowerSearch)))
    )
  }, [models, modelSearchText])

  // 按 Provider 分组的模型
  const modelsByProvider = useMemo(() => {
    const grouped: Record<string, ModelRead[]> = {}
    filteredModels.forEach((model) => {
      if (!grouped[model.provider_name]) {
        grouped[model.provider_name] = []
      }
      grouped[model.provider_name].push(model)
    })
    return grouped
  }, [filteredModels])

  // 处理Provider开关切换
  const handleProviderToggle = async (provider: ProviderRead, checked: boolean) => {
    try {
      await providerApi.updateProvider(provider.name, { is_active: checked })
      message.success(`Provider ${checked ? '已激活' : '已禁用'}`)
      await loadProviders()
      // 如果禁用了当前选中的Provider，提示用户
      if (!checked && selectedProvider?.name === provider.name) {
        message.warning('当前Provider已被禁用')
      }
    } catch (error: any) {
      console.error('Failed to update provider:', error)
      const errorMessage = error.response?.data?.detail || error.message || '操作失败'
      message.error(errorMessage)
    }
  }

  // 处理模型开关切换
  const handleModelToggle = async (model: ModelRead, checked: boolean) => {
    try {
      await modelApi.updateModel(model.provider_name, model.name, { is_active: checked })
      message.success(`模型 ${checked ? '已激活' : '已禁用'}`)
      if (selectedProvider) {
        await loadModels(selectedProvider.name)
      } else {
        await loadModels()
      }
    } catch (error: any) {
      console.error('Failed to update model:', error)
      const errorMessage = error.response?.data?.detail || error.message || '操作失败'
      message.error(errorMessage)
    }
  }

  // 处理Provider选择
  const handleProviderSelect = (provider: ProviderRead) => {
    // 如果点击已选中的 Provider，则取消选择
    if (selectedProvider?.name === provider.name) {
      setSelectedProvider(null)
    } else {
      setSelectedProvider(provider)
    }
    setModelSearchText('') // 清空模型搜索
  }

  // 处理添加Provider
  const handleAddProvider = () => {
    setProviderFormMode('create')
    setProviderFormVisible(true)
  }

  // 处理编辑Provider
  const handleEditProvider = () => {
    if (!selectedProvider) return
    setProviderFormMode('edit')
    setProviderFormVisible(true)
  }

  // 处理Provider表单提交
  const handleProviderSubmit = async (values: ProviderCreate | ProviderUpdate) => {
    try {
      if (providerFormMode === 'create') {
        const createValues = values as ProviderCreate
        if (!createValues.name || !createValues.type) {
          message.error('Provider名称和类型是必填项')
          return
        }
        await providerApi.createProvider(createValues)
        message.success('Provider创建成功')
      } else {
        if (!selectedProvider) {
          message.error('编辑Provider信息缺失')
          return
        }
        await providerApi.updateProvider(selectedProvider.name, values as ProviderUpdate)
        message.success('Provider更新成功')
      }
      setProviderFormVisible(false)
      await loadProviders()
      // 如果是创建，选中新创建的Provider
      if (providerFormMode === 'create' && 'name' in values) {
        const updatedProviders = await providerApi.getProviders()
        const newProvider = updatedProviders.find((p) => p.name === values.name)
        if (newProvider) {
          setSelectedProvider(newProvider)
        }
      }
    } catch (error: any) {
      console.error('Failed to submit provider:', error)
      const errorMessage = error.response?.data?.detail || error.message || '操作失败'
      message.error(errorMessage)
    }
  }

  // 处理保存Provider配置
  const handleSaveProviderConfig = async () => {
    if (!selectedProvider) return
    try {
      await providerApi.updateProvider(selectedProvider.name, {
        base_url: providerConfig.base_url || undefined,
        api_key: providerConfig.api_key || undefined,
      })
      message.success('配置保存成功')
      await loadProviders()
      // 更新选中的Provider
      const updated = await providerApi.getProviders()
      const updatedProvider = updated.find((p) => p.name === selectedProvider.name)
      if (updatedProvider) {
        setSelectedProvider(updatedProvider)
      }
    } catch (error: any) {
      console.error('Failed to save config:', error)
      const errorMessage = error.response?.data?.detail || error.message || '操作失败'
      message.error(errorMessage)
    }
  }

  // 处理重置API地址
  const handleResetApiUrl = () => {
    if (!selectedProvider) return
    // 根据Provider类型设置默认地址
    const defaultUrls: Record<string, string> = {
      openai: 'https://api.openai.com',
      openrouter: 'https://openrouter.ai/api/v1',
      gemini: 'https://generativelanguage.googleapis.com',
      claude: 'https://api.anthropic.com',
    }
    const defaultUrl = defaultUrls[selectedProvider.type] || ''
    setProviderConfig({ ...providerConfig, base_url: defaultUrl })
  }

  // 生成API地址预览
  const getApiUrlPreview = (baseUrl: string | null | undefined): string => {
    if (!baseUrl) return ''
    // 移除末尾的斜杠
    const cleanUrl = baseUrl.replace(/\/+$/, '')
    // 根据Provider类型添加端点
    if (selectedProvider?.type === 'openrouter') {
      return `${cleanUrl}/chat/completions`
    } else if (selectedProvider?.type === 'openai') {
      return `${cleanUrl}/v1/chat/completions`
    } else {
      return `${cleanUrl}/chat/completions`
    }
  }

  // 获取Provider的API Key获取URL
  const getApiKeyUrl = (providerType: string): string | null => {
    const urlMap: Record<string, string> = {
      openai: 'https://platform.openai.com/api-keys',
      claude: 'https://console.anthropic.com/settings/keys',
      gemini: 'https://aistudio.google.com/app/apikey',
      openrouter: 'https://openrouter.ai/settings/keys',
      glm: 'https://bigmodel.cn/dev/api',
      kimi: 'https://platform.moonshot.ai',
      qwen: 'https://modelstudio.console.alibabacloud.com/?tab=playground#/api-key',
    }
    return urlMap[providerType] || null
  }

  // 处理添加模型
  const handleAddModel = () => {
    setModelFormMode('create')
    setEditingModel(undefined)
    setModelFormVisible(true)
  }

  // 处理编辑模型
  const handleEditModel = (model: ModelRead) => {
    setModelFormMode('edit')
    setEditingModel(model)
    setModelFormVisible(true)
  }

  // 处理模型表单提交
  const handleModelSubmit = async (values: ModelCreate | ModelUpdate) => {
    try {
      if (modelFormMode === 'create') {
        const createValues = values as ModelCreate
        if (!selectedProvider || !createValues.name) {
          message.error('Provider和模型名称是必填项')
          return
        }
        createValues.provider_name = selectedProvider.name
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
      setModelFormVisible(false)
      if (selectedProvider) {
        await loadModels(selectedProvider.name)
      } else {
        await loadModels() // 未选择 Provider 时重新加载所有模型
      }
    } catch (error: any) {
      console.error('Failed to submit model:', error)
      const errorMessage = error.response?.data?.detail || error.message || '操作失败'
      message.error(errorMessage)
    }
  }

  // 处理同步配置文件
  const handleSyncConfig = async () => {
    setLoading(true)
    try {
      const result = await configApi.syncFromFile()
      message.success(result.message || '配置同步成功')
      // 重新加载数据
      await loadProviders()
      await loadModels(selectedProvider?.name)
    } catch (error: any) {
      console.error('Failed to sync config:', error)
      const errorMessage = error.response?.data?.detail || error.message || '同步配置失败'
      message.error(errorMessage)
    } finally {
      setLoading(false)
    }
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
            <EditOutlined onClick={() => handleEditModel(model)} />
          </Tooltip>,
        ]}
      >
        <Row gutter={[8, 8]}>
          <Col span={24}>
            <Space style={{ width: '100%', justifyContent: 'space-between' }}>
              <Space>
                <Text strong>{model.display_name || model.name}</Text>
                {model.is_active === false && <Tag color="red">未激活</Tag>}
                {model.is_active !== false && <Tag color="green">激活</Tag>}
              </Space>
              <div onClick={(e) => e.stopPropagation()}>
                <Switch
                  size="small"
                  checked={model.is_active ?? true}
                  onChange={(checked) => handleModelToggle(model, checked)}
                  onClick={(e) => e.stopPropagation()}
                />
              </div>
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
                {model.tags
                  .filter((tag) => tag && typeof tag === 'string')
                  .map((tag) => {
                    const TagIcon = getTagIcon(tag)
                    return (
                      <Tag key={tag} color="blue" style={{ margin: 0 }}>
                        {TagIcon && <TagIcon style={{ marginRight: 4 }} />}
                        {tag}
                      </Tag>
                    )
                  })}
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
    <Row gutter={16} style={{ height: '100%' }}>
      {/* 左侧Provider列表 */}
      <Col span={6}>
        <Card
          title={
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Input
                placeholder="搜索模型平台..."
                prefix={<SearchOutlined />}
                value={providerSearchText}
                onChange={(e) => setProviderSearchText(e.target.value)}
                allowClear
                size="small"
                style={{ flex: 1 }}
              />
              <Button
                icon={<SyncOutlined />}
                size="small"
                onClick={handleSyncConfig}
                loading={loading}
                title="从 router.toml 和 .env 文件同步配置"
              />
              <Button
                type="primary"
                icon={<PlusOutlined />}
                size="small"
                onClick={handleAddProvider}
                title="添加Provider"
              />
            </div>
          }
          style={{ height: '100%' }}
          bodyStyle={{ padding: '12px', maxHeight: 'calc(100vh - 200px)', overflowY: 'auto' }}
        >
          <List
            dataSource={filteredProviders}
            renderItem={(provider) => (
              <List.Item
                style={{
                  cursor: 'pointer',
                  backgroundColor: selectedProvider?.name === provider.name ? '#e6f7ff' : 'transparent',
                  padding: '8px 12px',
                  borderRadius: '4px',
                  marginBottom: '4px',
                }}
                onClick={() => handleProviderSelect(provider)}
              >
                <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                  <Space>
                    <div onClick={(e) => e.stopPropagation()}>
                      <Switch
                        size="small"
                        checked={provider.is_active ?? true}
                        onChange={(checked) => handleProviderToggle(provider, checked)}
                        onClick={(e) => e.stopPropagation()}
                      />
                    </div>
                    <div>
                      <Text strong={selectedProvider?.name === provider.name}>
                        {provider.name}
                      </Text>
                      <br />
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {provider.type}
                      </Text>
                    </div>
                  </Space>
                  {provider.is_active && (
                    <Tag color="green" style={{ margin: 0 }}>ON</Tag>
                  )}
                </Space>
              </List.Item>
            )}
          />
        </Card>
      </Col>

      {/* 右侧配置和模型列表 */}
      <Col span={18}>
        {selectedProvider ? (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            {/* Provider配置区域 */}
            <Card
              title={
                <Space>
                  <Text strong>{selectedProvider.name}</Text>
                  <LinkOutlined />
                </Space>
              }
              extra={
                <Button
                  type="link"
                  icon={<EditOutlined />}
                  onClick={handleEditProvider}
                >
                  编辑
                </Button>
              }
            >
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                <div>
                  <Text strong>API 密钥</Text>
                  <Input.Password
                    placeholder="API 密钥"
                    value={providerConfig.api_key}
                    onChange={(e) => setProviderConfig({ ...providerConfig, api_key: e.target.value })}
                    style={{ marginTop: 8 }}
                  />
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {selectedProvider && getApiKeyUrl(selectedProvider.type) ? (
                      <a
                        href={getApiKeyUrl(selectedProvider.type)!}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        点击这里获取密钥
                      </a>
                    ) : selectedProvider?.type === 'ollama' || 
                        selectedProvider?.type === 'transformers_local' || 
                        selectedProvider?.type === 'vllm_local' ? (
                      <span>本地模型，无需API密钥</span>
                    ) : (
                      <span>请查看该Provider的官方文档获取API密钥</span>
                    )}
                  </Text>
                </div>

                <div>
                  <Space>
                    <Text strong>API 地址</Text>
                    <Tooltip title="帮助">
                      <Button type="text" size="small" icon={<SettingOutlined />} />
                    </Tooltip>
                    <Tooltip title="刷新">
                      <Button type="text" size="small" icon={<ReloadOutlined />} />
                    </Tooltip>
                  </Space>
                  <Input
                    placeholder="https://openrouter.ai/api/v1"
                    value={providerConfig.base_url}
                    onChange={(e) => setProviderConfig({ ...providerConfig, base_url: e.target.value })}
                    style={{ marginTop: 8 }}
                  />
                  {providerConfig.base_url && (
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                      预览: {getApiUrlPreview(providerConfig.base_url)}
                    </Text>
                  )}
                  <Button
                    type="link"
                    size="small"
                    onClick={handleResetApiUrl}
                    style={{ padding: 0, marginTop: 4 }}
                  >
                    重置
                  </Button>
                </div>

                <Button type="primary" onClick={handleSaveProviderConfig}>
                  保存配置
                </Button>
              </Space>
            </Card>

            {/* 模型列表区域 */}
            <Card
              title={
                <Space>
                  <Text strong>模型 {filteredModels.length}</Text>
                </Space>
              }
              extra={
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={handleAddModel}
                >
                  添加模型
                </Button>
              }
            >
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                <Input
                  placeholder="搜索模型..."
                  prefix={<SearchOutlined />}
                  value={modelSearchText}
                  onChange={(e) => setModelSearchText(e.target.value)}
                  allowClear
                />

                {loading ? (
                  <Empty description="加载中..." />
                ) : filteredModels.length === 0 ? (
                  <Empty description="暂无模型数据" />
                ) : (
                  <div>
                    {filteredModels.map((model) => renderModelCard(model))}
                  </div>
                )}
              </Space>
            </Card>
          </Space>
        ) : (
          <Card
            title={
              <Space>
                <Text strong>所有模型</Text>
                <Tag>{filteredModels.length} 个模型</Tag>
              </Space>
            }
            extra={
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={handleAddModel}
              >
                添加模型
              </Button>
            }
          >
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Input
                placeholder="搜索模型..."
                prefix={<SearchOutlined />}
                value={modelSearchText}
                onChange={(e) => setModelSearchText(e.target.value)}
                allowClear
              />

              {loading ? (
                <Empty description="加载中..." />
              ) : Object.keys(modelsByProvider).length === 0 ? (
                <Empty description="暂无模型数据" />
              ) : (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  {Object.entries(modelsByProvider).map(([providerName, providerModels]) => (
                    <Card
                      key={providerName}
                      title={
                        <Space>
                          <Text strong>{providerName}</Text>
                          <Tag>{providerModels.length} 个模型</Tag>
                        </Space>
                      }
                      size="small"
                    >
                      <div>
                        {providerModels.map((model) => renderModelCard(model))}
                      </div>
                    </Card>
                  ))}
                </Space>
              )}
            </Space>
          </Card>
        )}
      </Col>

      {/* Provider表单Modal */}
      <ProviderForm
        visible={providerFormVisible}
        mode={providerFormMode}
        provider={selectedProvider || undefined}
        onCancel={() => {
          setProviderFormVisible(false)
        }}
        onSubmit={handleProviderSubmit}
      />

      {/* 模型表单Modal */}
      <ModelForm
        visible={modelFormVisible}
        mode={modelFormMode}
        model={editingModel}
        providers={providers}
        onCancel={() => {
          setModelFormVisible(false)
          setEditingModel(undefined)
        }}
        onSubmit={handleModelSubmit}
      />
    </Row>
  )
}

export default ModelManagement
