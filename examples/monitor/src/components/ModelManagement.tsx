import React from 'react'
import { Tabs } from 'antd'
import ProviderModelManagementPane from './ProviderModelManagementPane'
import APIKeyManagementPane from './APIKeyManagementPane'

const ModelManagement: React.FC = () => {
  return (
    <Tabs
      defaultActiveKey="provider-models"
      items={[
        {
          key: 'provider-models',
          label: 'Provider / 模型',
          children: <ProviderModelManagementPane />,
        },
        {
          key: 'api-keys',
          label: '系统 API Key',
          children: <APIKeyManagementPane />,
        },
      ]}
    />
  )
}

export default ModelManagement
