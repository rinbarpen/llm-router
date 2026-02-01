import React from 'react'
import { Card, Row, Col, Space, Typography, Tag, Tooltip, Button, Switch } from 'antd'
import {
  EditOutlined,
  EyeOutlined,
  SettingOutlined,
  GlobalOutlined,
  SyncOutlined,
  DollarOutlined,
} from '@ant-design/icons'
import type { ModelRead, PricingSuggestion } from '../services/types'
import { getTagIcon } from '../utils/tagIcons'
import { getProviderDisplayName } from '../utils/providerConstants'

const { Text } = Typography

export interface ModelCardProps {
  model: ModelRead
  pricingSuggestion?: PricingSuggestion
  pricingLoading: boolean
  onToggle: (model: ModelRead, checked: boolean) => void
  onEdit: (model: ModelRead) => void
  onSyncPricing: (modelId: number, modelName: string) => void
}

const ModelCard: React.FC<ModelCardProps> = ({
  model,
  pricingSuggestion,
  pricingLoading,
  onToggle,
  onEdit,
  onSyncPricing,
}) => {
  return (
    <Card
      key={`${model.provider_name}-${model.name}`}
      size="small"
      style={{ marginBottom: 8 }}
    >
      <Row gutter={[8, 8]}>
        <Col span={24}>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Text strong>{model.display_name || model.name}</Text>
              {model.is_active === false && <Tag color="red">未激活</Tag>}
              {model.is_active !== false && <Tag color="green">激活</Tag>}
            </Space>
            <div
              onClick={(e) => e.stopPropagation()}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
              }}
            >
              <Tooltip title="编辑模型">
                <EditOutlined
                  onClick={(e) => {
                    e.stopPropagation()
                    onEdit(model)
                  }}
                  style={{ cursor: 'pointer', fontSize: '16px' }}
                />
              </Tooltip>
              <div onClick={(e) => e.stopPropagation()}>
                <Switch
                  size="small"
                  checked={model.is_active ?? true}
                  onChange={(checked) => onToggle(model, checked)}
                />
              </div>
            </div>
          </Space>
        </Col>
        <Col span={24}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {getProviderDisplayName(model.provider_name)}/{model.name}
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
          <Space size="small" style={{ width: '100%', justifyContent: 'space-between' }}>
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
              {(model.config?.cost_per_1k_tokens !== undefined ||
                model.config?.cost_per_1k_completion_tokens !== undefined) && (
                <Tooltip
                  title={
                    <div>
                      <div>输入: ${model.config?.cost_per_1k_tokens || 0}/1k tokens</div>
                      <div>输出: ${model.config?.cost_per_1k_completion_tokens || 0}/1k tokens</div>
                    </div>
                  }
                >
                  <DollarOutlined style={{ color: '#52c41a' }} />
                </Tooltip>
              )}
            </Space>
            <Tooltip title="同步定价">
              <Button
                type="text"
                size="small"
                icon={<SyncOutlined />}
                loading={pricingLoading}
                onClick={(e) => {
                  e.stopPropagation()
                  onSyncPricing(model.id, model.name)
                }}
              />
            </Tooltip>
          </Space>
        </Col>
        {pricingSuggestion && pricingSuggestion.has_update && (
          <Col span={24}>
            <Space size="small" style={{ fontSize: 12, color: '#faad14' }}>
              <DollarOutlined />
              <span>
                定价可更新: $
                {pricingSuggestion.current_input_price?.toFixed(4) || 'N/A'} → $
                {pricingSuggestion.latest_input_price?.toFixed(4) || 'N/A'} (输入)
              </span>
            </Space>
          </Col>
        )}
      </Row>
    </Card>
  )
}

export default ModelCard
