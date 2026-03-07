import React, { useEffect, useState } from 'react'
import { Modal, Form, Input, Switch, Select, Space, Button, Typography, Tag, message } from 'antd'
import { MinusCircleOutlined, PlusOutlined, LoginOutlined, DisconnectOutlined } from '@ant-design/icons'
import type { ProviderRead, ProviderCreate, ProviderUpdate, ProviderType } from '../services/types'
import { DEFAULT_PROVIDER_BASE_URLS, getApiUrlPreview } from '../utils/providerConstants'
import { oauthApi } from '../services/api'
import { getApiErrorMessage } from '../utils/errorUtils'

const OAUTH_SUPPORTED_TYPES: ProviderType[] = ['openrouter', 'gemini']

const providerTypes: { label: string; value: ProviderType }[] = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Gemini', value: 'gemini' },
  { label: 'Claude', value: 'claude' },
  { label: 'OpenRouter', value: 'openrouter' },
  { label: '智谱 bigmodel', value: 'bigmodel' },
  { label: 'z.ai', value: 'z.ai' },
  { label: 'Kimi', value: 'kimi' },
  { label: 'Qwen', value: 'qwen' },
  { label: 'Ollama', value: 'ollama' },
  { label: 'Remote HTTP', value: 'remote_http' },
  { label: 'Transformers Local', value: 'transformers_local' },
  { label: 'vLLM Local', value: 'vllm_local' },
  { label: 'Ollama Local', value: 'ollama_local' },
]

interface ProviderFormProps {
  visible: boolean
  mode: 'create' | 'edit'
  provider?: ProviderRead
  onCancel: () => void
  onSubmit: (values: ProviderCreate | ProviderUpdate) => Promise<void>
}

const ProviderForm: React.FC<ProviderFormProps> = ({
  visible,
  mode,
  provider,
  onCancel,
  onSubmit,
}) => {
  const [form] = Form.useForm()
  const watchedType = Form.useWatch('type', form)
  const watchedBaseUrl = Form.useWatch('base_url', form)
  const [hasOAuth, setHasOAuth] = useState(false)
  const [oauthLoading, setOauthLoading] = useState(false)

  useEffect(() => {
    if (visible) {
      if (mode === 'edit' && provider) {
        form.setFieldsValue({
          name: provider.name,
          type: provider.type,
          base_url: provider.base_url || (DEFAULT_PROVIDER_BASE_URLS[provider.type] ?? ''),
          api_key: provider.api_key ?? '',
          is_active: provider.is_active,
          settings: (provider as any).settings ? objectToKeyValuePairs((provider as any).settings) : [{ key: '', value: '' }],
        })
      } else {
        form.resetFields()
        form.setFieldsValue({
          is_active: true,
          settings: [{ key: '', value: '' }],
        })
      }
    }
  }, [visible, mode, provider, form])

  // 创建模式下，选择 Provider 类型时自动填充默认 API 地址
  useEffect(() => {
    if (mode === 'create' && watchedType) {
      const defaultUrl = DEFAULT_PROVIDER_BASE_URLS[watchedType] ?? ''
      form.setFieldsValue({ base_url: defaultUrl })
    }
  }, [mode, watchedType, form])

  // 编辑模式下，检查 OAuth 绑定状态
  useEffect(() => {
    if (mode === 'edit' && provider && OAUTH_SUPPORTED_TYPES.includes(provider.type)) {
      oauthApi.getStatus(provider.type, provider.name).then((r) => setHasOAuth(r.has_oauth)).catch(() => setHasOAuth(false))
    } else {
      setHasOAuth(false)
    }
  }, [mode, provider])

  const handleOAuthLogin = async () => {
    if (!provider || mode !== 'edit') return
    setOauthLoading(true)
    try {
      const callbackUrl = typeof window !== 'undefined' ? window.location.origin + window.location.pathname : '/'
      const url = await oauthApi.getAuthorizeUrl(provider.type, provider.name, callbackUrl)
      window.location.href = url
    } catch (err) {
      message.error(getApiErrorMessage(err, '获取 OAuth 授权链接失败'))
    } finally {
      setOauthLoading(false)
    }
  }

  const handleOAuthRevoke = async () => {
    if (!provider || mode !== 'edit') return
    setOauthLoading(true)
    try {
      await oauthApi.revoke(provider.type, provider.name)
      setHasOAuth(false)
      message.success('已解除 OAuth 绑定')
    } catch (err) {
      message.error(getApiErrorMessage(err, '解除 OAuth 绑定失败'))
    } finally {
      setOauthLoading(false)
    }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()

      // 处理settings字段 - 将键值对转换为对象
      if (values.settings && Array.isArray(values.settings)) {
        const settingsObj: Record<string, any> = {}
        values.settings.forEach((item: { key: string; value: any }) => {
          if (item.key) {
            // 尝试解析value为JSON，如果失败则作为字符串
            try {
              settingsObj[item.key] = JSON.parse(item.value)
            } catch {
              settingsObj[item.key] = item.value
            }
          }
        })
        values.settings = Object.keys(settingsObj).length > 0 ? settingsObj : {}
      }

      await onSubmit(values)
      form.resetFields()
    } catch (error) {
      console.error('Form validation failed:', error)
    }
  }

  const handleCancel = () => {
    form.resetFields()
    onCancel()
  }

  // 将对象转换为键值对数组
  const objectToKeyValuePairs = (obj: Record<string, any> | undefined): Array<{ key: string; value: string }> => {
    if (!obj || typeof obj !== 'object') {
      return [{ key: '', value: '' }]
    }
    const pairs = Object.entries(obj).map(([key, value]) => ({
      key,
      value: typeof value === 'string' ? value : JSON.stringify(value),
    }))
    return pairs.length > 0 ? pairs : [{ key: '', value: '' }]
  }

  return (
    <Modal
      title={mode === 'create' ? '添加Provider' : '编辑Provider'}
      open={visible}
      onCancel={handleCancel}
      onOk={handleSubmit}
      width={700}
      okText="确定"
      cancelText="取消"
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          is_active: true,
          settings: [{ key: '', value: '' }],
        }}
      >
        {mode === 'create' && (
          <Form.Item
            name="name"
            label="Provider名称"
            rules={[{ required: true, message: '请输入Provider名称' }]}
          >
            <Input placeholder="例如: openai" />
          </Form.Item>
        )}

        {mode === 'edit' && (
          <Form.Item name="name" label="Provider名称">
            <Input disabled />
          </Form.Item>
        )}

        <Form.Item
          name="type"
          label="Provider类型"
          rules={[{ required: true, message: '请选择Provider类型' }]}
        >
          <Select placeholder="选择Provider类型" disabled={mode === 'edit'}>
            {providerTypes.map((type) => (
              <Select.Option key={type.value} value={type.value}>
                {type.label}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>

        <Form.Item name="base_url" label="API地址">
          <Input placeholder="例如: https://api.openai.com/v1" />
        </Form.Item>
        {watchedBaseUrl && (
          <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: -16, marginBottom: 16 }}>
            预览: {getApiUrlPreview(watchedBaseUrl, mode === 'edit' && provider ? provider : (watchedType ? { name: '', type: watchedType } : null))}
          </Typography.Text>
        )}

        <Form.Item
          name="api_key"
          label="API密钥"
          extra={
            mode === 'edit' &&
            provider &&
            OAUTH_SUPPORTED_TYPES.includes(provider.type) ? (
              <Space style={{ marginTop: 8 }}>
                {hasOAuth ? (
                  <>
                    <Tag color="green">已通过 OAuth 登录</Tag>
                    <Button
                      type="link"
                      size="small"
                      danger
                      icon={<DisconnectOutlined />}
                      loading={oauthLoading}
                      onClick={handleOAuthRevoke}
                    >
                      解除绑定
                    </Button>
                  </>
                ) : (
                  <Button
                    type="primary"
                    ghost
                    icon={<LoginOutlined />}
                    loading={oauthLoading}
                    onClick={handleOAuthLogin}
                  >
                    通过 OAuth 登录
                  </Button>
                )}
              </Space>
            ) : null
          }
        >
          <Input.Password placeholder="输入API密钥" />
        </Form.Item>

        <Form.Item name="is_active" label="状态" valuePropName="checked">
          <Switch checkedChildren="激活" unCheckedChildren="未激活" />
        </Form.Item>

        <Form.Item label="设置" name="settings">
          <Form.List name="settings">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...restField}
                      name={[name, 'key']}
                      rules={[{ required: false, message: '设置键' }]}
                    >
                      <Input placeholder="设置键" style={{ width: 150 }} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, 'value']}
                      rules={[{ required: false, message: '设置值' }]}
                    >
                      <Input placeholder="设置值 (JSON格式)" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(name)} />
                  </Space>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                    添加设置
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default ProviderForm
