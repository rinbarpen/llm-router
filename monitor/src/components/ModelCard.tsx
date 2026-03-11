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
      style={{ 
        marginBottom: 12, 
        borderRadius: '12px',
        border: '1px solid #e5e7eb',
        overflow: 'hidden'
      }}
      bodyStyle={{ padding: '16px' }}
    >
      <Row gutter={[16, 12]}>
        <Col span={24}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Space direction="vertical" size={0}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Text strong style={{ fontSize: '16px' }}>{model.display_name || model.name}</Text>
                {model.is_active !== false ? (
                  <Tag color="success" bordered={false} style={{ borderRadius: '4px', margin: 0 }}>Active</Tag>
                ) : (
                  <Tag color="error" bordered={false} style={{ borderRadius: '4px', margin: 0 }}>Inactive</Tag>
                )}
              </div>
              <Text type="secondary" style={{ fontSize: '12px' }}>
                {getProviderDisplayName(model.provider_name)} · {model.name}
              </Text>
            </Space>
            <Space size="middle">
              <Tooltip title="编辑模型">
                <Button 
                  type="text" 
                  icon={<EditOutlined />} 
                  onClick={() => onEdit(model)}
                  style={{ color: '#6366f1' }}
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
            <Text type="secondary" style={{ fontSize: '13px', display: 'block' }}>
              {model.description}
            </Text>
          </Col>
        )}

        <Col span={24}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Space wrap size={[4, 4]}>
              {model.tags
                ?.filter((tag) => tag && typeof tag === 'string')
                .map((tag) => {
                  const TagIcon = getTagIcon(tag)
                  return (
                    <Tag key={tag} bordered={false} style={{ background: '#f5f3ff', color: '#6366f1', borderRadius: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}>
                      {TagIcon && <TagIcon style={{ fontSize: '12px' }} />}
                      {tag}
                    </Tag>
                  )
                })}
            </Space>
            
            <Space size="large">
              <Space size="small">
                {model.config?.supports_vision && (
                  <Tooltip title="支持视觉输入">
                    <EyeOutlined style={{ color: '#6366f1' }} />
                  </Tooltip>
                )}
                {model.config?.supports_tools && (
                  <Tooltip title="支持工具调用">
                    <SettingOutlined style={{ color: '#6366f1' }} />
                  </Tooltip>
                )}
                {model.rate_limit && (
                  <Tooltip title={`速率限制: ${model.rate_limit.max_requests} req / ${model.rate_limit.per_seconds}s`}>
                    <GlobalOutlined style={{ color: '#6366f1' }} />
                  </Tooltip>
                )}
              </Space>

              <div style={{ textAlign: 'right' }}>
                {(model.config?.cost_per_1k_tokens !== undefined ||
                  model.config?.cost_per_1k_completion_tokens !== undefined) ? (
                  <Tooltip title={
                    <div style={{ fontSize: '12px' }}>
                      <div>Input: ${model.config?.cost_per_1k_tokens || 0} / 1k</div>
                      <div>Output: ${model.config?.cost_per_1k_completion_tokens || 0} / 1k</div>
                    </div>
                  }>
                    <Space size={4} style={{ color: '#10b981', fontWeight: 600, fontSize: '13px' }}>
                      <DollarOutlined />
                      <span>${model.config?.cost_per_1k_tokens || 0}</span>
                    </Space>
                  </Tooltip>
                ) : (
                  <Text type="secondary" style={{ fontSize: '12px' }}>无定价</Text>
                )}
              </div>
            </Space>
          </div>
        </Col>

        {pricingSuggestion && pricingSuggestion.has_update && (
          <Col span={24}>
            <div style={{ 
              background: '#fffbeb', 
              padding: '8px 12px', 
              borderRadius: '8px', 
              border: '1px solid #fef3c7',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center'
            }}>
              <Space size={8}>
                <SyncOutlined spin={pricingLoading} style={{ color: '#d97706' }} />
                <Text style={{ fontSize: '12px', color: '#92400e' }}>
                  发现定价更新: ${pricingSuggestion.current_input_price?.toFixed(4)} → ${pricingSuggestion.latest_input_price?.toFixed(4)}
                </Text>
              </Space>
              <Button 
                type="link" 
                size="small" 
                onClick={() => onSyncPricing(model.id, model.name)}
                style={{ color: '#d97706', padding: 0, height: 'auto' }}
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
