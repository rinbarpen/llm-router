import React, { useEffect } from 'react'
import { Modal, Form, Input, Switch, Select, Space, Button } from 'antd'
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import type { ProviderRead, ProviderCreate, ProviderUpdate, ProviderType } from '../services/types'

const providerTypes: { label: string; value: ProviderType }[] = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Gemini', value: 'gemini' },
  { label: 'Claude', value: 'claude' },
  { label: 'OpenRouter', value: 'openrouter' },
  { label: 'GLM', value: 'glm' },
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

  useEffect(() => {
    if (visible) {
      if (mode === 'edit' && provider) {
        form.setFieldsValue({
          name: provider.name,
          type: provider.type,
          base_url: provider.base_url || '',
          is_active: provider.is_active,
          settings: provider.settings ? objectToKeyValuePairs(provider.settings) : [{ key: '', value: '' }],
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
          <Input placeholder="例如: https://api.openai.com" />
        </Form.Item>

        <Form.Item name="api_key" label="API密钥">
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
