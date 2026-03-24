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
      className="invocation-detail-modal"
    >
      <Tabs
        className="invocation-detail-tabs"
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
                    className="invocation-detail-error-alert"
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
                  <div className="invocation-detail-section">
                    <Text strong>Prompt:</Text>
                    <Paragraph
                      className="invocation-detail-codeblock"
                    >
                      {invocation.request_prompt}
                    </Paragraph>
                  </div>
                )}

                {invocation.request_messages && invocation.request_messages.length > 0 && (
                  <div className="invocation-detail-section">
                    <Text strong>Messages:</Text>
                    {invocation.request_messages.map((msg, idx) => (
                      <div
                        key={idx}
                        className="invocation-detail-message-block"
                      >
                        <Tag color="blue">{msg.role}</Tag>
                        <Paragraph className="invocation-detail-message-content">
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
                      className="invocation-detail-pre"
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
                    className="invocation-detail-response"
                  >
                    {invocation.response_text}
                  </Paragraph>
                ) : (
                  <Text type="secondary">无响应内容</Text>
                )}

                {invocation.response_text_length && (
                  <Text type="secondary" className="invocation-detail-response-length">
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
                      className="invocation-detail-pre invocation-detail-raw-pre"
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
