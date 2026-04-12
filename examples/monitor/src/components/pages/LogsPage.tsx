import React, { useState } from 'react'
import { Card, Segmented, Typography } from 'antd'
import InvocationList from '../InvocationList'
import LoginRecordList from '../LoginRecordList'

const { Paragraph, Title } = Typography

type LogTab = 'invocations' | 'logins'

const LogsPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<LogTab>('invocations')

  return (
    <div className="monitor-page logs-page">
      <section className="page-hero compact-hero">
        <div className="page-kicker">Operations Timeline</div>
        <Title level={1}>日志信息</Title>
        <Paragraph>
          将模型调用历史和认证访问记录放在同一个工作区，便于排障、审计和导出监控数据。
        </Paragraph>
      </section>

      <Card className="editorial-card log-switch-card">
        <Segmented
          value={activeTab}
          onChange={(value) => setActiveTab(value as LogTab)}
          options={[
            { label: '调用日志', value: 'invocations' },
            { label: '登录记录', value: 'logins' },
          ]}
        />
      </Card>

      <div className="ops-panel">
        {activeTab === 'invocations' ? <InvocationList /> : <LoginRecordList />}
      </div>
    </div>
  )
}

export default LogsPage
