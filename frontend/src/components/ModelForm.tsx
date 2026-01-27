import React, { useEffect } from 'react'
import { Modal, Form, Input, Switch, Select, InputNumber, Space, Button, Tag } from 'antd'
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import type { ModelRead, ModelCreate, ModelUpdate, ProviderRead } from '../services/types'
import { getTagIcon } from '../utils/tagIcons'

const { TextArea } = Input

interface ModelFormProps {
  visible: boolean
  mode: 'create' | 'edit'
  model?: ModelRead
  providers: ProviderRead[]
  onCancel: () => void
  onSubmit: (values: ModelCreate | ModelUpdate) => Promise<void>
}

const ModelForm: React.FC<ModelFormProps> = ({
  visible,
  mode,
  model,
  providers,
  onCancel,
  onSubmit,
}) => {
  const [form] = Form.useForm()

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

  useEffect(() => {
    if (visible) {
      if (mode === 'edit' && model) {
        form.setFieldsValue({
          provider_name: model.provider_name,
          name: model.name,
          display_name: model.display_name || '',
          description: model.description || '',
          tags: model.tags || [],
          is_active: model.is_active !== false,
          default_params: objectToKeyValuePairs(model.default_params),
          config: objectToKeyValuePairs(model.config),
          rate_limit: model.rate_limit
            ? {
                max_requests: model.rate_limit.max_requests,
                per_seconds: model.rate_limit.per_seconds,
                burst_size: model.rate_limit.burst_size,
                notes: model.rate_limit.notes || '',
              }
            : undefined,
        })
      } else {
        form.resetFields()
        form.setFieldsValue({
          is_active: true,
          tags: [],
          default_params: [{ key: '', value: '' }],
          config: [{ key: '', value: '' }],
        })
      }
    }
  }, [visible, mode, model, form])

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      
      // 处理config字段 - 将键值对转换为对象
      if (values.config && Array.isArray(values.config)) {
        const configObj: Record<string, any> = {}
        values.config.forEach((item: { key: string; value: any }) => {
          if (item.key) {
            // 尝试解析value为JSON，如果失败则作为字符串
            try {
              configObj[item.key] = JSON.parse(item.value)
            } catch {
              configObj[item.key] = item.value
            }
          }
        })
        values.config = configObj
      }

      // 处理default_params字段
      if (values.default_params && Array.isArray(values.default_params)) {
        const paramsObj: Record<string, any> = {}
        values.default_params.forEach((item: { key: string; value: any }) => {
          if (item.key) {
            try {
              paramsObj[item.key] = JSON.parse(item.value)
            } catch {
              paramsObj[item.key] = item.value
            }
          }
        })
        values.default_params = paramsObj
      }

      // 处理rate_limit - 如果所有字段都为空，则设为undefined
      if (values.rate_limit) {
        if (!values.rate_limit.max_requests && !values.rate_limit.per_seconds) {
          values.rate_limit = undefined
        } else {
          // 移除空字段
          if (!values.rate_limit.burst_size) {
            delete values.rate_limit.burst_size
          }
          if (!values.rate_limit.notes) {
            delete values.rate_limit.notes
          }
          if (!values.rate_limit.config || Object.keys(values.rate_limit.config).length === 0) {
            delete values.rate_limit.config
          }
        }
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

  return (
    <Modal
      title={mode === 'create' ? '添加模型' : '编辑模型'}
      open={visible}
      onCancel={handleCancel}
      onOk={handleSubmit}
      width={800}
      okText="确定"
      cancelText="取消"
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          is_active: true,
          tags: [],
        }}
      >
        {mode === 'create' && (
          <>
            <Form.Item
              name="provider_name"
              label="Provider"
              rules={[{ required: true, message: '请选择Provider' }]}
            >
              <Select placeholder="选择Provider">
                {providers.map((provider) => (
                  <Select.Option key={provider.id} value={provider.name}>
                    {provider.name} ({provider.type})
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
            <Form.Item
              name="name"
              label="模型名称"
              rules={[{ required: true, message: '请输入模型名称' }]}
            >
              <Input placeholder="例如: gpt-4o" />
            </Form.Item>
          </>
        )}

        {mode === 'edit' && (
          <>
            <Form.Item name="provider_name" label="Provider">
              <Input disabled />
            </Form.Item>
            <Form.Item name="name" label="模型名称">
              <Input disabled />
            </Form.Item>
          </>
        )}

        <Form.Item name="display_name" label="显示名称">
          <Input placeholder="例如: GPT-4o" />
        </Form.Item>

        <Form.Item name="description" label="描述">
          <TextArea rows={3} placeholder="模型描述" />
        </Form.Item>

        <Form.Item name="tags" label="标签">
          <Select
            mode="tags"
            placeholder="输入标签后按回车"
            tokenSeparators={[',']}
            style={{ width: '100%' }}
            tagRender={(props) => {
              const { label, value, closable, onClose } = props
              const Icon = getTagIcon(value && typeof value === 'string' ? value : String(value || ''))
              return (
                <Tag
                  color="blue"
                  closable={closable}
                  onClose={onClose}
                  style={{ marginRight: 3 }}
                >
                  {Icon && <Icon style={{ marginRight: 4 }} />}
                  {label}
                </Tag>
              )
            }}
          />
        </Form.Item>

        <Form.Item name="is_active" label="状态" valuePropName="checked">
          <Switch checkedChildren="激活" unCheckedChildren="未激活" />
        </Form.Item>

        <Form.Item label="默认参数" name="default_params">
          <Form.List name="default_params">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...restField}
                      name={[name, 'key']}
                      rules={[{ required: false, message: '参数名' }]}
                    >
                      <Input placeholder="参数名" style={{ width: 150 }} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, 'value']}
                      rules={[{ required: false, message: '参数值' }]}
                    >
                      <Input placeholder="参数值 (JSON格式)" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(name)} />
                  </Space>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                    添加参数
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        </Form.Item>

        <Form.Item label="配置" name="config">
          <Form.List name="config">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...restField}
                      name={[name, 'key']}
                      rules={[{ required: false, message: '配置键' }]}
                    >
                      <Input placeholder="配置键" style={{ width: 150 }} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, 'value']}
                      rules={[{ required: false, message: '配置值' }]}
                    >
                      <Input placeholder="配置值 (JSON格式)" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(name)} />
                  </Space>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                    添加配置
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        </Form.Item>

        <Form.Item label="速率限制" name="rate_limit">
          <Space direction="vertical" style={{ width: '100%' }}>
            <Form.Item name={['rate_limit', 'max_requests']} label="最大请求数">
              <InputNumber min={1} placeholder="例如: 50" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name={['rate_limit', 'per_seconds']} label="时间窗口(秒)">
              <InputNumber min={1} placeholder="例如: 60" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name={['rate_limit', 'burst_size']} label="突发大小 (可选)">
              <InputNumber min={1} placeholder="例如: 100" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name={['rate_limit', 'notes']} label="备注 (可选)">
              <Input placeholder="速率限制备注" />
            </Form.Item>
          </Space>
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default ModelForm
