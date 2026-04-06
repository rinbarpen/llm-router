import React, { useEffect, useState } from 'react'
import { Modal, Form, Input, Switch, Select, Space, Button, Typography, Tag, message, InputNumber, Divider, List } from 'antd'
import { MinusCircleOutlined, PlusOutlined, LoginOutlined, DisconnectOutlined } from '@ant-design/icons'
import type { ProviderRead, ProviderCreate, ProviderUpdate, ProviderType, OAuthAccount } from '../services/types'
import { DEFAULT_PROVIDER_BASE_URLS, getApiUrlPreview } from '../utils/providerConstants'
import { oauthApi } from '../services/api'
import { getApiErrorMessage } from '../utils/errorUtils'

const OAUTH_SUPPORTED_TYPES: ProviderType[] = ['openrouter', 'gemini']
interface OAuthAccountDraft {
  id: number
  account_name: string
  is_default: boolean
  is_active: boolean
  settings: Record<string, any>
}

const providerTypes: { label: string; value: ProviderType }[] = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Codex CLI', value: 'codex_cli' },
  { label: 'OpenCode CLI', value: 'opencode_cli' },
  { label: 'Kimi Code CLI', value: 'kimi_code_cli' },
  { label: 'Qwen Code CLI', value: 'qwen_code_cli' },
  { label: 'Gemini', value: 'gemini' },
  { label: 'Claude', value: 'claude' },
  { label: 'Claude Code CLI', value: 'claude_code_cli' },
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
  const [oauthAccounts, setOAuthAccounts] = useState<OAuthAccountDraft[]>([])

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
      refreshOAuthState(provider)
    } else {
      setHasOAuth(false)
      setOAuthAccounts([])
    }
  }, [mode, provider])

  const refreshOAuthState = async (currentProvider: ProviderRead) => {
    try {
      const [status, accounts] = await Promise.all([
        oauthApi.getStatus(currentProvider.type, currentProvider.name),
        oauthApi.listAccounts(currentProvider.type, currentProvider.name),
      ])
      setHasOAuth(status.has_oauth)
      setOAuthAccounts(accounts.map(mapOAuthAccountDraft))
    } catch {
      setHasOAuth(false)
      setOAuthAccounts([])
    }
  }

  const mapOAuthAccountDraft = (item: OAuthAccount): OAuthAccountDraft => ({
    id: item.id,
    account_name: item.account_name || `oauth-${item.id}`,
    is_default: item.is_default,
    is_active: item.is_active,
    settings: item.settings || {},
  })

  const updateDraft = (id: number, patch: Partial<OAuthAccountDraft>) => {
    setOAuthAccounts((prev) => prev.map((item) => (item.id === id ? { ...item, ...patch } : item)))
  }

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
      setOAuthAccounts([])
      message.success('已解除 OAuth 绑定')
    } catch (err) {
      message.error(getApiErrorMessage(err, '解除 OAuth 绑定失败'))
    } finally {
      setOauthLoading(false)
    }
  }

  const handleOAuthAccountSave = async (account: OAuthAccountDraft) => {
    if (!provider || mode !== 'edit') return
    setOauthLoading(true)
    try {
      await oauthApi.updateAccount(provider.type, provider.name, account.id, {
        account_name: account.account_name,
        is_active: account.is_active,
        settings: account.settings,
      })
      await refreshOAuthState(provider)
      message.success('OAuth 账号已更新')
    } catch (err) {
      message.error(getApiErrorMessage(err, '更新 OAuth 账号失败'))
    } finally {
      setOauthLoading(false)
    }
  }

  const handleOAuthAccountSetDefault = async (accountID: number) => {
    if (!provider || mode !== 'edit') return
    setOauthLoading(true)
    try {
      await oauthApi.setDefaultAccount(provider.type, provider.name, accountID)
      await refreshOAuthState(provider)
      message.success('默认 OAuth 账号已更新')
    } catch (err) {
      message.error(getApiErrorMessage(err, '设置默认 OAuth 账号失败'))
    } finally {
      setOauthLoading(false)
    }
  }

  const handleOAuthAccountRevoke = async (accountID: number) => {
    if (!provider || mode !== 'edit') return
    setOauthLoading(true)
    try {
      await oauthApi.revokeAccount(provider.type, provider.name, accountID)
      await refreshOAuthState(provider)
      message.success('OAuth 账号已解除绑定')
    } catch (err) {
      message.error(getApiErrorMessage(err, '解除 OAuth 账号失败'))
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

        {mode === 'edit' && provider && OAUTH_SUPPORTED_TYPES.includes(provider.type) && (
          <>
            <Divider style={{ marginTop: 8 }}>OAuth 账号池</Divider>
            <List
              size="small"
              dataSource={oauthAccounts}
              locale={{ emptyText: '暂无 OAuth 账号，可先点击上方“通过 OAuth 登录”添加。' }}
              renderItem={(account) => (
                <List.Item>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Space wrap>
                      <Input
                        value={account.account_name}
                        onChange={(e) => updateDraft(account.id, { account_name: e.target.value })}
                        placeholder="账号名"
                        style={{ width: 180 }}
                      />
                      <Switch
                        checked={account.is_active}
                        checkedChildren="启用"
                        unCheckedChildren="禁用"
                        onChange={(checked) => updateDraft(account.id, { is_active: checked })}
                      />
                      {account.is_default ? <Tag color="green">默认</Tag> : <Tag>备用</Tag>}
                      <Button size="small" onClick={() => handleOAuthAccountSetDefault(account.id)} disabled={account.is_default} loading={oauthLoading}>设为默认</Button>
                      <Button size="small" danger onClick={() => handleOAuthAccountRevoke(account.id)} loading={oauthLoading}>解绑</Button>
                    </Space>
                    <Space wrap>
                      <span>priority</span>
                      <InputNumber
                        value={Number(account.settings.priority ?? 0)}
                        onChange={(v) => updateDraft(account.id, { settings: { ...account.settings, priority: Number(v ?? 0) } })}
                      />
                      <span>max_requests</span>
                      <InputNumber
                        value={Number(account.settings.max_requests ?? 0)}
                        onChange={(v) => updateDraft(account.id, { settings: { ...account.settings, max_requests: Number(v ?? 0) } })}
                      />
                      <span>per_seconds</span>
                      <InputNumber
                        value={Number(account.settings.per_seconds ?? 0)}
                        onChange={(v) => updateDraft(account.id, { settings: { ...account.settings, per_seconds: Number(v ?? 0) } })}
                      />
                      <span>max_in_flight</span>
                      <InputNumber
                        value={Number(account.settings.max_in_flight ?? 4)}
                        onChange={(v) => updateDraft(account.id, { settings: { ...account.settings, max_in_flight: Number(v ?? 4) } })}
                      />
                      <span>cooldown_seconds</span>
                      <InputNumber
                        value={Number(account.settings.cooldown_seconds ?? 30)}
                        onChange={(v) => updateDraft(account.id, { settings: { ...account.settings, cooldown_seconds: Number(v ?? 30) } })}
                      />
                      <Button type="primary" size="small" onClick={() => handleOAuthAccountSave(account)} loading={oauthLoading}>保存</Button>
                    </Space>
                  </Space>
                </List.Item>
              )}
            />
          </>
        )}

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
