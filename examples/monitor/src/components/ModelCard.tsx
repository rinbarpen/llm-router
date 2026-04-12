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
      className="model-card"
      style={{ marginBottom: 12 }}
      bodyStyle={{ padding: '16px' }}
    >
      <Row gutter={[16, 12]} className="model-card-grid">
        <Col span={24}>
          <div className="model-card-header">
            <Space direction="vertical" size={2} className="model-card-heading">
              <div className="model-card-title-wrap">
                <Text strong className="model-card-title">{model.display_name || model.name}</Text>
                {model.is_active !== false ? (
                  <Tag color="success" bordered={false} className="model-card-status-tag">Active</Tag>
                ) : (
                  <Tag color="error" bordered={false} className="model-card-status-tag">Inactive</Tag>
                )}
              </div>
              <Text type="secondary" className="model-card-provider">
                {getProviderDisplayName(model.provider_name)} · {model.name}
              </Text>
            </Space>
            <Space size="middle" className="model-card-controls">
              <Tooltip title="编辑模型">
                <Button 
                  type="text" 
                  icon={<EditOutlined />} 
                  onClick={() => onEdit(model)}
                  className="model-card-edit-btn"
                />
              </Tooltip>
              <Switch
                size="small"
                checked={model.is_active ?? true}
                onChange={(checked) => onToggle(model, checked)}
              />
            </Space>
          </div>
        </Col>

        {model.description && (
          <Col span={24}>
            <Text type="secondary" className="model-card-description">
              {model.description}
            </Text>
          </Col>
        )}

        <Col span={24}>
          <div className="model-card-meta">
            <Space wrap size={[4, 6]} className="model-card-tags">
              {model.tags
                ?.filter((tag) => tag && typeof tag === 'string')
                .map((tag) => {
                  const TagIcon = getTagIcon(tag)
                  return (
                    <Tag key={tag} bordered={false} className="model-card-tag">
                      {TagIcon && <TagIcon style={{ fontSize: '12px' }} />}
                      {tag}
                    </Tag>
                  )
                })}
            </Space>
            
            <div className="model-card-meta-side">
              <Space size="small" className="model-card-capabilities">
                {model.config?.supports_vision && (
                  <Tooltip title="支持视觉输入">
                    <EyeOutlined className="model-card-capability-icon" />
                  </Tooltip>
                )}
                {model.config?.supports_tools && (
                  <Tooltip title="支持工具调用">
                    <SettingOutlined className="model-card-capability-icon" />
                  </Tooltip>
                )}
                {model.rate_limit && (
                  <Tooltip title={`速率限制: ${model.rate_limit.max_requests} req / ${model.rate_limit.per_seconds}s`}>
                    <GlobalOutlined className="model-card-capability-icon" />
                  </Tooltip>
                )}
              </Space>

              <div className="model-card-price">
                {(model.config?.cost_per_1k_tokens !== undefined ||
                  model.config?.cost_per_1k_completion_tokens !== undefined) ? (
                  <Tooltip title={
                    <div style={{ fontSize: '12px' }}>
                      <div>Input: ${model.config?.cost_per_1k_tokens || 0} / 1k</div>
                      <div>Output: ${model.config?.cost_per_1k_completion_tokens || 0} / 1k</div>
                    </div>
                  }>
                    <Space size={4} className="model-card-price-value">
                      <DollarOutlined />
                      <span>${model.config?.cost_per_1k_tokens || 0}</span>
                    </Space>
                  </Tooltip>
                ) : (
                  <Text type="secondary" style={{ fontSize: '12px' }}>无定价</Text>
                )}
              </div>
            </div>
          </div>
        </Col>

        {pricingSuggestion && pricingSuggestion.has_update && (
          <Col span={24}>
            <div className="model-card-pricing-update">
              <Space size={8}>
                <SyncOutlined spin={pricingLoading} className="model-card-pricing-sync-icon" />
                <Text className="model-card-pricing-text">
                  发现定价更新: ${pricingSuggestion.current_input_price?.toFixed(4)} → ${pricingSuggestion.latest_input_price?.toFixed(4)}
                </Text>
              </Space>
              <Button 
                type="link" 
                size="small" 
                onClick={() => onSyncPricing(model.id, model.name)}
                className="model-card-pricing-action"
              >
                立即同步
              </Button>
            </div>
          </Col>
        )}
      </Row>
    </Card>
  )
}

export default ModelCard
