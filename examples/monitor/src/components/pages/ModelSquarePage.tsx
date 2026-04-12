import React, { useEffect, useMemo, useState } from 'react'
import { Button, Card, Col, Empty, Input, Row, Select, Space, Spin, Tag, Typography, message } from 'antd'
import { MessageOutlined, ReloadOutlined, SearchOutlined, SettingOutlined } from '@ant-design/icons'
import { modelApi, pricingApi, providerApi } from '../../services/api'
import type { ModelRead, ProviderRead } from '../../services/types'
import { getProviderDisplayName } from '../../utils/providerConstants'

const { Paragraph, Text, Title } = Typography

interface ModelSquarePageProps {
  onNavigate: (page: 'chat' | 'token-management') => void
}

const getModelStatus = (model: ModelRead, provider?: ProviderRead) => {
  if (model.is_active === false) return { label: 'inactive', color: 'default' }
  if (provider && provider.is_active === false) return { label: 'needs setup', color: 'warning' }
  return { label: 'available', color: 'success' }
}

const ModelSquarePage: React.FC<ModelSquarePageProps> = ({ onNavigate }) => {
  const [models, setModels] = useState<ModelRead[]>([])
  const [providers, setProviders] = useState<ProviderRead[]>([])
  const [pricingCount, setPricingCount] = useState(0)
  const [loading, setLoading] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [providerName, setProviderName] = useState<string | undefined>()
  const [capability, setCapability] = useState<string | undefined>()

  const loadData = async () => {
    setLoading(true)
    try {
      const [modelList, providerList, pricing] = await Promise.all([
        modelApi.getModels(),
        providerApi.getProviders(),
        pricingApi.getLatestPricing().catch(() => ({} as Record<string, any[]>)),
      ])
      setModels(modelList)
      setProviders(providerList)
      setPricingCount(Object.values(pricing).reduce((total, items) => total + items.length, 0))
    } catch (error) {
      console.error(error)
      message.error('模型广场加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const providerMap = useMemo(
    () => new Map(providers.map((provider) => [provider.name, provider])),
    [providers],
  )

  const filteredModels = useMemo(() => {
    const lower = keyword.trim().toLowerCase()
    return models.filter((model) => {
      const haystack = [
        model.name,
        model.display_name,
        model.description,
        model.provider_name,
        ...(model.tags ?? []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      const matchesKeyword = !lower || haystack.includes(lower)
      const matchesProvider = !providerName || model.provider_name === providerName
      const matchesCapability =
        !capability ||
        (capability === 'vision' && model.config?.supports_vision) ||
        (capability === 'tools' && model.config?.supports_tools) ||
        (capability === 'stream' && model.config?.supports_stream)
      return matchesKeyword && matchesProvider && matchesCapability
    })
  }, [capability, keyword, models, providerName])

  return (
    <div className="monitor-page model-square-page">
      <section className="page-hero">
        <div className="page-kicker">Model Atlas</div>
        <Title level={1}>模型广场</Title>
        <Paragraph>
          浏览已接入的模型、能力标签和 Provider 状态。选择模型后可进入 Chat 测试，或返回控制台完善令牌与接入配置。
        </Paragraph>
        <Space wrap className="model-square-stats">
          <Tag>{models.length} models</Tag>
          <Tag>{providers.length} providers</Tag>
          <Tag>{pricingCount} pricing rows</Tag>
        </Space>
      </section>

      <Card className="editorial-card filter-card">
        <Space wrap className="model-square-filters">
          <Input
            allowClear
            prefix={<SearchOutlined />}
            placeholder="搜索模型、Provider、标签..."
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
          <Select
            allowClear
            placeholder="Provider"
            value={providerName}
            onChange={setProviderName}
            options={providers.map((provider) => ({
              label: getProviderDisplayName(provider.name),
              value: provider.name,
            }))}
          />
          <Select
            allowClear
            placeholder="能力"
            value={capability}
            onChange={setCapability}
            options={[
              { label: 'Vision', value: 'vision' },
              { label: 'Tools', value: 'tools' },
              { label: 'Stream', value: 'stream' },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>
            刷新
          </Button>
        </Space>
      </Card>

      {loading ? (
        <div className="monitor-content-loading">
          <Spin />
        </div>
      ) : filteredModels.length === 0 ? (
        <Card className="editorial-card">
          <Empty description="没有匹配的模型" />
        </Card>
      ) : (
        <Row gutter={[16, 16]}>
          {filteredModels.map((model) => {
            const provider = providerMap.get(model.provider_name)
            const status = getModelStatus(model, provider)
            return (
              <Col xs={24} md={12} xl={8} key={`${model.provider_name}/${model.name}`}>
                <Card className="editorial-card square-model-card">
                  <div className="square-model-card-head">
                    <div>
                      <Text className="square-model-provider">
                        {getProviderDisplayName(model.provider_name)}
                      </Text>
                      <Title level={4}>{model.display_name || model.name}</Title>
                    </div>
                    <Tag color={status.color}>{status.label}</Tag>
                  </div>
                  <Paragraph type="secondary" ellipsis={{ rows: 3 }}>
                    {model.description || `${model.provider_name}/${model.name}`}
                  </Paragraph>
                  <Space wrap className="square-model-tags">
                    {(model.tags ?? []).slice(0, 6).map((tag) => (
                      <Tag key={tag}>{tag}</Tag>
                    ))}
                    {model.config?.supports_vision && <Tag color="blue">vision</Tag>}
                    {model.config?.supports_tools && <Tag color="cyan">tools</Tag>}
                    {model.config?.supports_stream && <Tag color="green">stream</Tag>}
                  </Space>
                  <div className="square-model-actions">
                    <Button type="primary" icon={<MessageOutlined />} onClick={() => onNavigate('chat')}>
                      在 Chat 中使用
                    </Button>
                    <Button icon={<SettingOutlined />} onClick={() => onNavigate('token-management')}>
                      查看配置
                    </Button>
                  </div>
                </Card>
              </Col>
            )
          })}
        </Row>
      )}
    </div>
  )
}

export default ModelSquarePage
