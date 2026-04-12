import React from 'react'
import { Card, Space, Input, Empty } from 'antd'
import { SearchOutlined } from '@ant-design/icons'

export interface ModelListSectionProps {
  title: React.ReactNode
  titleExtra?: React.ReactNode
  extra: React.ReactNode
  searchValue: string
  onSearchChange: (value: string) => void
  loading: boolean
  isEmpty: boolean
  children: React.ReactNode
}

const ModelListSection: React.FC<ModelListSectionProps> = ({
  title,
  titleExtra,
  extra,
  searchValue,
  onSearchChange,
  loading,
  isEmpty,
  children,
}) => {
  return (
    <Card
      className="model-list-section"
      title={
        <div className="model-list-section-head">
          <Space className="model-list-section-title" size="small">
            {title}
            {titleExtra}
          </Space>
        </div>
      }
      extra={<div className="model-list-section-actions">{extra}</div>}
    >
      <Space direction="vertical" size="middle" className="model-list-section-body">
        <div className="model-list-section-search-row">
          <Input
            placeholder="搜索模型..."
            prefix={<SearchOutlined />}
            value={searchValue}
            onChange={(e) => onSearchChange(e.target.value)}
            allowClear
            className="model-list-section-search"
          />
        </div>
        {loading ? (
          <Empty description="加载中..." />
        ) : isEmpty ? (
          <Empty description="暂无模型数据" />
        ) : (
          children
        )}
      </Space>
    </Card>
  )
}

export default ModelListSection
