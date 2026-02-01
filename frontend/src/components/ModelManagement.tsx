import React, { useState, useEffect, useMemo } from 'react'
import {
  Space,
  Input,
  Button,
  Card,
  Row,
  Col,
  Typography,
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
  SettingOutlined,
  ReloadOutlined,
  LinkOutlined,
  SyncOutlined,
  UpOutlined,
  DownOutlined,
} from '@ant-design/icons'
import type { ModelRead, ModelCreate, ModelUpdate, ProviderRead, ProviderCreate, ProviderUpdate, PricingSuggestion } from '../services/types'
import { modelApi, providerApi, configApi, pricingApi } from '../services/api'
import { getApiErrorMessage, getPricingErrorMessage } from '../utils/errorUtils'
import {
  DEFAULT_PROVIDER_BASE_URLS,
  getApiKeyUrl,
  getApiKeyLinkText,
  getProviderDisplayLabel,
  getProviderDisplayName,
  getApiUrlPreview,
} from '../utils/providerConstants'
import ModelCard from './ModelCard'
import ModelListSection from './ModelListSection'
import ModelForm from './ModelForm'
import ProviderForm from './ProviderForm'
import { useCollapsedProviders } from '../hooks/useCollapsedProviders'

const { Text } = Typography

const COLLAPSED_PROVIDERS_KEY = 'llm-router-collapsed-providers'

const ModelManagement: React.FC = () => {
  const [providers, setProviders] = useState<ProviderRead[]>([])
  const [selectedProvider, setSelectedProvider] = useState<ProviderRead | null>(null)
  const [models, setModels] = useState<ModelRead[]>([])
  const [loading, setLoading] = useState(false)
  const [providerSearchText, setProviderSearchText] = useState('')
  const [modelSearchText, setModelSearchText] = useState('')
  const [providerFormVisible, setProviderFormVisible] = useState(false)
  const [providerFormMode, setProviderFormMode] = useState<'create' | 'edit'>('create')
  const [editingProvider, setEditingProvider] = useState<ProviderRead | null>(null)
  const [modelFormVisible, setModelFormVisible] = useState(false)
  const [modelFormMode, setModelFormMode] = useState<'create' | 'edit'>('create')
  const [editingModel, setEditingModel] = useState<ModelRead | undefined>()
  const [providerConfig, setProviderConfig] = useState<{ api_key?: string; base_url?: string }>({})
  const [collapsedProviders, handleCollapse] = useCollapsedProviders(COLLAPSED_PROVIDERS_KEY)
  const [pricingSuggestions, setPricingSuggestions] = useState<PricingSuggestion[]>([])
  const [pricingLoading, setPricingLoading] = useState(false)

  // 加载Provider列表
  const loadProviders = async () => {
    try {
      const data = await providerApi.getProviders()
      setProviders(data)
    } catch (error) {
      console.error('Failed to load providers:', error)
      message.error(getApiErrorMessage(error, '加载Provider列表失败'))
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
      message.error(getApiErrorMessage(error, '加载模型列表失败'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadProviders()
    loadModels() // 自动加载所有模型
    fetchPricingSuggestions() // 加载定价建议
  }, [])

  useEffect(() => {
    if (selectedProvider) {
      loadModels(selectedProvider.name)
      // 初始化配置（api_key 从 .env 同步后存在后端，在此显示）
      setProviderConfig({
        base_url:
          selectedProvider.base_url ||
          (DEFAULT_PROVIDER_BASE_URLS[selectedProvider.name] ?? DEFAULT_PROVIDER_BASE_URLS[selectedProvider.type]) ||
          '',
        api_key: selectedProvider.api_key ?? '',
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
    } catch (error) {
      console.error('Failed to update provider:', error)
      message.error(getApiErrorMessage(error, '操作失败'))
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
    } catch (error) {
      console.error('Failed to update model:', error)
      message.error(getApiErrorMessage(error, '操作失败'))
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

  // 处理编辑Provider（从右侧卡片入口）
  const handleEditProvider = () => {
    if (!selectedProvider) return
    setEditingProvider(selectedProvider)
    setProviderFormMode('edit')
    setProviderFormVisible(true)
  }

  // 从左侧列表或「所有模型」卡片点击编辑时使用，传入要编辑的 provider
  const handleEditProviderFromList = (provider: ProviderRead) => {
    setEditingProvider(provider)
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
        const providerToEdit = editingProvider ?? selectedProvider
        if (!providerToEdit) {
          message.error('编辑Provider信息缺失')
          return
        }
        await providerApi.updateProvider(providerToEdit.name, values as ProviderUpdate)
        message.success('Provider更新成功')
      }
      setProviderFormVisible(false)
      setEditingProvider(null)
      await loadProviders()
      // 如果是创建，选中新创建的Provider
      if (providerFormMode === 'create' && 'name' in values) {
        const updatedProviders = await providerApi.getProviders()
        const newProvider = updatedProviders.find((p) => p.name === values.name)
        if (newProvider) {
          setSelectedProvider(newProvider)
        }
      }
    } catch (error) {
      console.error('Failed to submit provider:', error)
      message.error(getApiErrorMessage(error, '操作失败'))
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
    } catch (error) {
      console.error('Failed to save config:', error)
      message.error(getApiErrorMessage(error, '操作失败'))
    }
  }

  // 处理重置API地址
  const handleResetApiUrl = () => {
    if (!selectedProvider) return
    const defaultUrl =
      DEFAULT_PROVIDER_BASE_URLS[selectedProvider.name] ?? DEFAULT_PROVIDER_BASE_URLS[selectedProvider.type] ?? ''
    setProviderConfig({ ...providerConfig, base_url: defaultUrl })
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
    } catch (error) {
      console.error('Failed to submit model:', error)
      message.error(getApiErrorMessage(error, '操作失败'))
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
    } catch (error) {
      console.error('Failed to sync config:', error)
      message.error(getApiErrorMessage(error, '同步配置失败'))
    } finally {
      setLoading(false)
    }
  }

  // 加载定价建议
  const fetchPricingSuggestions = async () => {
    setPricingLoading(true)
    try {
      const suggestions = await pricingApi.getPricingSuggestions()
      setPricingSuggestions(suggestions)
    } catch (error) {
      console.error('Failed to load pricing suggestions:', error)
      message.error(getPricingErrorMessage(error, '加载定价建议失败'))
    } finally {
      setPricingLoading(false)
    }
  }

  // 同步单个模型定价
  const handleSyncModelPricing = async (modelId: number, modelName: string) => {
    try {
      const result = await pricingApi.syncModelPricing(modelId)
      if (result.success) {
        message.success(`模型 ${modelName} 的定价已更新`)
        await loadModels(selectedProvider?.name)
        await fetchPricingSuggestions()
      } else {
        message.warning(result.message)
      }
    } catch (error) {
      console.error('Failed to sync model pricing:', error)
      message.error(getPricingErrorMessage(error, '同步定价失败'))
    }
  }

  // 批量同步所有模型定价
  const handleSyncAllPricing = async () => {
    setPricingLoading(true)
    try {
      const result = await pricingApi.syncAllPricing()
      message.success(result.message)
      await loadModels(selectedProvider?.name)
      await fetchPricingSuggestions()
    } catch (error) {
      console.error('Failed to sync all pricing:', error)
      message.error(getPricingErrorMessage(error, '批量同步定价失败'))
    } finally {
      setPricingLoading(false)
    }
  }

  const getModelPricingSuggestion = (modelId: number): PricingSuggestion | undefined =>
    pricingSuggestions.find((s) => s.model_id === modelId)

  const renderModelCard = (model: ModelRead) => (
    <ModelCard
      key={`${model.provider_name}-${model.name}`}
      model={model}
      pricingSuggestion={getModelPricingSuggestion(model.id)}
      pricingLoading={pricingLoading}
      onToggle={handleModelToggle}
      onEdit={handleEditModel}
      onSyncPricing={handleSyncModelPricing}
    />
  )

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
                      />
                    </div>
                    <div>
                      <Text strong={selectedProvider?.name === provider.name}>
                        {getProviderDisplayName(provider.name)}
                      </Text>
                      <br />
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {getProviderDisplayLabel(provider)}
                      </Text>
                    </div>
                  </Space>
                  <Space onClick={(e) => e.stopPropagation()}>
                    <Button
                      type="link"
                      size="small"
                      icon={<EditOutlined />}
                      onClick={() => handleEditProviderFromList(provider)}
                    >
                      编辑
                    </Button>
                    {provider.is_active && (
                      <Tag color="green" style={{ margin: 0 }}>ON</Tag>
                    )}
                  </Space>
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
                  <Text strong>{getProviderDisplayName(selectedProvider.name)}</Text>
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
                    {selectedProvider && getApiKeyUrl(selectedProvider.name, selectedProvider.type) ? (
                      <a
                        href={getApiKeyUrl(selectedProvider.name, selectedProvider.type)!}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {getApiKeyLinkText(selectedProvider.name, selectedProvider.type)}
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
                    placeholder={
                      (selectedProvider &&
                        (DEFAULT_PROVIDER_BASE_URLS[selectedProvider.name] ??
                          DEFAULT_PROVIDER_BASE_URLS[selectedProvider.type])) ??
                      '例如: https://api.example.com'
                    }
                    value={providerConfig.base_url}
                    onChange={(e) => setProviderConfig({ ...providerConfig, base_url: e.target.value })}
                    style={{ marginTop: 8 }}
                  />
                  {providerConfig.base_url && (
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                      预览: {getApiUrlPreview(providerConfig.base_url, selectedProvider)}
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
            <ModelListSection
              title={<Text strong>模型 {filteredModels.length}</Text>}
              extra={
                <Space>
                  <Button
                    icon={<SyncOutlined />}
                    onClick={handleSyncAllPricing}
                    loading={pricingLoading}
                    title="同步所有模型定价"
                  >
                    同步定价
                  </Button>
                  <Button type="primary" icon={<PlusOutlined />} onClick={handleAddModel}>
                    添加模型
                  </Button>
                </Space>
              }
              searchValue={modelSearchText}
              onSearchChange={setModelSearchText}
              loading={loading}
              isEmpty={filteredModels.length === 0}
            >
              <div>
                {filteredModels.map((model) => renderModelCard(model))}
              </div>
            </ModelListSection>
          </Space>
        ) : (
          <ModelListSection
            title={<Text strong>所有模型</Text>}
            titleExtra={<Tag>{filteredModels.length} 个模型</Tag>}
            extra={
              <Space>
                <Button
                  icon={<SyncOutlined />}
                  onClick={handleSyncAllPricing}
                  loading={pricingLoading}
                  title="同步所有模型定价"
                >
                  同步定价
                </Button>
                <Button type="primary" icon={<PlusOutlined />} onClick={handleAddModel}>
                  添加模型
                </Button>
              </Space>
            }
            searchValue={modelSearchText}
            onSearchChange={setModelSearchText}
            loading={loading}
            isEmpty={Object.keys(modelsByProvider).length === 0}
          >
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              {Object.entries(modelsByProvider).map(([providerName, providerModels]) => {
                const provider = providers.find((p) => p.name === providerName)
                return (
                <Card
                  key={providerName}
                  title={
                    <Space>
                      <Text strong>{getProviderDisplayName(providerName)}</Text>
                      <Tag>{providerModels.length} 个模型</Tag>
                    </Space>
                  }
                  size="small"
                  extra={
                    <Space>
                      {provider && (
                        <Button
                          type="link"
                          size="small"
                          icon={<EditOutlined />}
                          onClick={(e) => {
                            e.stopPropagation()
                            handleEditProviderFromList(provider)
                          }}
                        >
                          编辑
                        </Button>
                      )}
                      <Button
                        type="text"
                        icon={collapsedProviders.has(providerName) ? <DownOutlined /> : <UpOutlined />}
                        onClick={(e) => {
                          e.stopPropagation()
                          handleCollapse(providerName, !collapsedProviders.has(providerName))
                        }}
                      />
                    </Space>
                  }
                >
                  {!collapsedProviders.has(providerName) && (
                    <div>
                      {providerModels.map((model) => renderModelCard(model))}
                    </div>
                  )}
                </Card>
                )
              })}
            </Space>
          </ModelListSection>
        )}
      </Col>

      {/* Provider表单Modal */}
      <ProviderForm
        visible={providerFormVisible}
        mode={providerFormMode}
        provider={editingProvider ?? selectedProvider ?? undefined}
        onCancel={() => {
          setProviderFormVisible(false)
          setEditingProvider(null)
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
