import React from 'react'
import { Modal, Descriptions, Tag, Typography, Tabs, Alert } from 'antd'
import dayjs from 'dayjs'
import type { InvocationRead } from '../services/types'

const { Text, Paragraph } = Typography

interface InvocationDetailProps {
  visible: boolean
  invocation: InvocationRead | null
  onClose: () => void
}

const InvocationDetail: React.FC<InvocationDetailProps> = ({ visible, invocation, onClose }) => {
  if (!invocation) return null

  return (
    <Modal
      title="调用详情"
      open={visible}
      onCancel={onClose}
      footer={null}
      width={800}
    >
      <Tabs
        defaultActiveKey="basic"
        items={[
          {
            key: 'basic',
            label: '基本信息',
            children: (
              <>
                <Descriptions column={2} bordered>
                  <Descriptions.Item label="ID">{invocation.id}</Descriptions.Item>
                  <Descriptions.Item label="状态">
                    <Tag color={invocation.status === 'success' ? 'green' : 'red'}>
                      {invocation.status === 'success' ? '成功' : '失败'}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="模型">
                    <Tag color="blue">{invocation.provider_name}</Tag>
                    {invocation.model_name}
                  </Descriptions.Item>
                  <Descriptions.Item label="延迟">
                    {invocation.duration_ms ? `${invocation.duration_ms.toFixed(2)}ms` : '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="开始时间">
                    {dayjs(invocation.started_at).format('YYYY-MM-DD HH:mm:ss.SSS')}
                  </Descriptions.Item>
                  <Descriptions.Item label="结束时间">
                    {invocation.completed_at
                      ? dayjs(invocation.completed_at).format('YYYY-MM-DD HH:mm:ss.SSS')
                      : '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Prompt Tokens">
                    {invocation.prompt_tokens?.toLocaleString() || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Completion Tokens">
                    {invocation.completion_tokens?.toLocaleString() || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Total Tokens" span={2}>
                    {invocation.total_tokens?.toLocaleString() || '-'}
                  </Descriptions.Item>
                </Descriptions>

                {invocation.error_message && (
                  <Alert
                    message="错误信息"
                    description={invocation.error_message}
                    type="error"
                    style={{ marginTop: 16 }}
                    showIcon
                  />
                )}
              </>
            ),
          },
          {
            key: 'request',
            label: '请求信息',
            children: (
              <>
                {invocation.request_prompt && (
                  <div style={{ marginBottom: 16 }}>
                    <Text strong>Prompt:</Text>
                    <Paragraph
                      style={{
                        background: '#f5f5f5',
                        padding: 12,
                        borderRadius: 4,
                        marginTop: 8,
                        whiteSpace: 'pre-wrap',
                      }}
                    >
                      {invocation.request_prompt}
                    </Paragraph>
                  </div>
                )}

                {invocation.request_messages && invocation.request_messages.length > 0 && (
                  <div style={{ marginBottom: 16 }}>
                    <Text strong>Messages:</Text>
                    {invocation.request_messages.map((msg, idx) => (
                      <div
                        key={idx}
                        style={{
                          background: '#f5f5f5',
                          padding: 12,
                          borderRadius: 4,
                          marginTop: 8,
                        }}
                      >
                        <Tag color="blue">{msg.role}</Tag>
                        <Paragraph style={{ marginTop: 8, marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                          {msg.content}
                        </Paragraph>
                      </div>
                    ))}
                  </div>
                )}

                {Object.keys(invocation.request_parameters).length > 0 && (
                  <div>
                    <Text strong>Parameters:</Text>
                    <pre
                      style={{
                        background: '#f5f5f5',
                        padding: 12,
                        borderRadius: 4,
                        marginTop: 8,
                        overflow: 'auto',
                      }}
                    >
                      {JSON.stringify(invocation.request_parameters, null, 2)}
                    </pre>
                  </div>
                )}
              </>
            ),
          },
          {
            key: 'response',
            label: '响应信息',
            children: (
              <>
                {invocation.response_text ? (
                  <Paragraph
                    style={{
                      background: '#f5f5f5',
                      padding: 12,
                      borderRadius: 4,
                      whiteSpace: 'pre-wrap',
                      maxHeight: 400,
                      overflow: 'auto',
                    }}
                  >
                    {invocation.response_text}
                  </Paragraph>
                ) : (
                  <Text type="secondary">无响应内容</Text>
                )}

                {invocation.response_text_length && (
                  <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>
                    响应长度: {invocation.response_text_length} 字符
                  </Text>
                )}
              </>
            ),
          },
          ...(invocation.raw_response
            ? [
                {
                  key: 'raw',
                  label: '原始响应',
                  children: (
                    <pre
                      style={{
                        background: '#f5f5f5',
                        padding: 12,
                        borderRadius: 4,
                        maxHeight: 500,
                        overflow: 'auto',
                      }}
                    >
                      {JSON.stringify(invocation.raw_response, null, 2)}
                    </pre>
                  ),
                },
              ]
            : []),
        ]}
      />
    </Modal>
  )
}

export default InvocationDetail

