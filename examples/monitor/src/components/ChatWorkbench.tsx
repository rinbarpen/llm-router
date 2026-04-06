import React, { useEffect, useMemo, useRef, useState } from 'react'
import {
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  List,
  Modal,
  Segmented,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd'
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  SendOutlined,
  StopOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeSanitize from 'rehype-sanitize'
import { chatApi } from '../services/api'
import type {
  ChatCompletionRequest,
  ChatCompletionResponse,
  ChatDebugTrace,
  ChatMessage,
  ChatSession,
  ChatSettings,
  ChatToolCall,
  ChatToolCallDelta,
} from '../services/types'

const { Text, Title } = Typography
const { TextArea } = Input

const STORAGE_KEY = 'llm-router-chat-sessions-v1'

const TEMPLATE_OPTIONS = [
  {
    value: 'balanced',
    label: '平衡',
    settings: { temperature: 0.7, topP: 1, maxTokens: 1024 },
  },
  {
    value: 'precise',
    label: '精确',
    settings: { temperature: 0.2, topP: 1, maxTokens: 1024 },
  },
  {
    value: 'creative',
    label: '创意',
    settings: { temperature: 1, topP: 1, maxTokens: 1536 },
  },
] as const

const generateId = () =>
  `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`

const extractAssistantOutput = (response: ChatCompletionResponse): { content: string; toolCalls: ChatToolCall[] } => {
  const choice = response.choices?.[0]
  const content = choice?.message?.content ?? ''
  const toolCalls = (choice?.message?.tool_calls ?? []).map((toolCall, index) => ({
    id: toolCall.id ?? generateId(),
    index,
    type: toolCall.type ?? 'function',
    name: toolCall.function?.name ?? 'unknown_tool',
    arguments: toolCall.function?.arguments ?? '',
  }))
  return { content, toolCalls }
}

const mergeToolCallDelta = (
  current: Record<number, ChatToolCall>,
  deltas: ChatToolCallDelta[]
): Record<number, ChatToolCall> => {
  const next = { ...current }
  for (const delta of deltas) {
    const existing = next[delta.index] ?? {
      id: delta.id ?? generateId(),
      index: delta.index,
      type: delta.type ?? 'function',
      name: delta.name ?? 'unknown_tool',
      arguments: '',
    }
    next[delta.index] = {
      ...existing,
      id: delta.id ?? existing.id,
      type: delta.type ?? existing.type,
      name: delta.name ?? existing.name,
      arguments: `${existing.arguments}${delta.argumentsPart ?? ''}`,
    }
  }
  return next
}

const createDefaultSettings = (model: string): ChatSettings => ({
  model,
  temperature: 0.7,
  maxTokens: 1024,
  topP: 1,
  stream: true,
  systemPrompt: '',
})

const createSession = (model: string): ChatSession => {
  const now = new Date().toISOString()
  return {
    id: generateId(),
    title: '新会话',
    createdAt: now,
    updatedAt: now,
    settings: createDefaultSettings(model),
    messages: [],
    traces: [],
  }
}

const inferTitleFromContent = (content: string) => {
  const text = content.trim().replace(/\s+/g, ' ')
  if (!text) {
    return '新会话'
  }
  return text.length > 24 ? `${text.slice(0, 24)}...` : text
}

const normalizeSessions = (sessions: ChatSession[]): ChatSession[] =>
  sessions.map((session) => ({
    ...session,
    messages: Array.isArray(session.messages) ? session.messages : [],
    traces: Array.isArray(session.traces) ? session.traces : [],
  }))

const ChatWorkbench: React.FC = () => {
  const [models, setModels] = useState<string[]>([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [activeSessionId, setActiveSessionId] = useState<string>('')
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [activeTraceIndex, setActiveTraceIndex] = useState<number>(-1)
  const [viewMode, setViewMode] = useState<'rendered' | 'raw'>('rendered')
  const [templateKey, setTemplateKey] = useState<string>('balanced')
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    let mounted = true
    setLoadingModels(true)
    chatApi
      .listActiveModels()
      .then((list) => {
        if (!mounted) {
          return
        }
        setModels(list)
      })
      .catch((error) => {
        console.error(error)
        message.error('加载模型列表失败')
      })
      .finally(() => {
        if (mounted) {
          setLoadingModels(false)
        }
      })
    return () => {
      mounted = false
    }
  }, [])

  useEffect(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (!raw) {
        return
      }
      const parsed = JSON.parse(raw) as ChatSession[]
      const restored = normalizeSessions(parsed)
      setSessions(restored)
      if (restored[0]) {
        setActiveSessionId(restored[0].id)
      }
    } catch (error) {
      console.error('Failed to parse chat sessions', error)
    }
  }, [])

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions))
  }, [sessions])

  useEffect(() => {
    if (models.length === 0) {
      return
    }
    setSessions((prev) => {
      if (prev.length === 0) {
        const first = createSession(models[0])
        setActiveSessionId(first.id)
        return [first]
      }
      return prev.map((session) => {
        if (models.includes(session.settings.model)) {
          return session
        }
        return {
          ...session,
          settings: {
            ...session.settings,
            model: models[0],
          },
        }
      })
    })
  }, [models])

  const patchSession = (sessionId: string, updater: (session: ChatSession) => ChatSession) => {
    setSessions((prev) =>
      prev.map((session) => {
        if (session.id !== sessionId) {
          return session
        }
        const updated = updater(session)
        return {
          ...updated,
          updatedAt: new Date().toISOString(),
        }
      })
    )
  }

  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionId) ?? null,
    [sessions, activeSessionId]
  )

  const activeTrace =
    activeSession && activeTraceIndex >= 0 && activeTraceIndex < activeSession.traces.length
      ? activeSession.traces[activeTraceIndex]
      : null

  const createNewSession = () => {
    if (!models[0]) {
      message.warning('暂无可用模型，请先在模型管理中激活模型')
      return
    }
    const next = createSession(models[0])
    setSessions((prev) => [next, ...prev])
    setActiveSessionId(next.id)
    setActiveTraceIndex(-1)
    setDraft('')
  }

  const renameSession = (session: ChatSession) => {
    const title = window.prompt('请输入会话标题', session.title)
    if (!title || !title.trim()) {
      return
    }
    patchSession(session.id, (current) => ({
      ...current,
      title: title.trim(),
    }))
  }

  const deleteSession = (sessionId: string) => {
    Modal.confirm({
      title: '删除会话',
      content: '确认删除该会话及其所有记录？',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: () => {
        setSessions((prev) => {
          const next = prev.filter((session) => session.id !== sessionId)
          if (next.length === 0 && models[0]) {
            const fallback = createSession(models[0])
            setActiveSessionId(fallback.id)
            return [fallback]
          }
          if (!next.find((session) => session.id === activeSessionId) && next[0]) {
            setActiveSessionId(next[0].id)
          }
          return next
        })
      },
    })
  }

  const clearActiveSession = () => {
    if (!activeSession) {
      return
    }
    patchSession(activeSession.id, (session) => ({
      ...session,
      title: '新会话',
      messages: [],
      traces: [],
    }))
    setActiveTraceIndex(-1)
  }

  const buildPayload = (session: ChatSession, userContent: string): ChatCompletionRequest => {
    const messages: Array<{ role: 'system' | 'user' | 'assistant'; content: string }> = []
    if (session.settings.systemPrompt.trim()) {
      messages.push({ role: 'system', content: session.settings.systemPrompt.trim() })
    }
    for (const item of session.messages) {
      if (item.role === 'user' || item.role === 'assistant') {
        messages.push({ role: item.role, content: item.content })
      }
    }
    messages.push({ role: 'user', content: userContent })
    return {
      model: session.settings.model,
      messages,
      stream: session.settings.stream,
      temperature: session.settings.temperature,
      max_tokens: session.settings.maxTokens,
      top_p: session.settings.topP,
    }
  }

  const sendMessage = async (input: string) => {
    const text = input.trim()
    if (!text || !activeSession || sending) {
      return
    }

    const userMessage: ChatMessage = {
      id: generateId(),
      role: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    }

    const assistantMessageId = generateId()
    const basePayload = buildPayload(activeSession, text)
    const trace: ChatDebugTrace = {
      request: basePayload,
      events: [],
    }

    patchSession(activeSession.id, (session) => ({
      ...session,
      title: session.messages.length === 0 ? inferTitleFromContent(text) : session.title,
      messages: [
        ...session.messages,
        userMessage,
        {
          id: assistantMessageId,
          role: 'assistant',
          content: '',
          createdAt: new Date().toISOString(),
        },
      ],
      traces: [trace, ...session.traces],
    }))
    setActiveTraceIndex(0)
    setDraft('')
    setSending(true)

    const sessionId = activeSession.id
    const controller = new AbortController()
    abortRef.current = controller

    try {
      if (activeSession.settings.stream) {
        const toolCallMap: Record<number, ChatToolCall> = {}
        const events: ChatCompletionResponse[] = []
        let usageSnapshot: ChatCompletionResponse['usage']
        let costSnapshot: number | undefined

        await chatApi.chatCompletionsStream(basePayload, controller.signal, {
          onEvent: (event) => {
            events.push(event)
            if (event.usage) {
              usageSnapshot = event.usage
            }
            if (typeof event.cost === 'number') {
              costSnapshot = event.cost
            }
          },
          onTextDelta: (chunk) => {
            patchSession(sessionId, (session) => ({
              ...session,
              messages: session.messages.map((item) =>
                item.id === assistantMessageId
                  ? {
                      ...item,
                      content: `${item.content}${chunk}`,
                    }
                  : item
              ),
            }))
          },
          onToolCallDelta: (deltas) => {
            const merged = mergeToolCallDelta(toolCallMap, deltas)
            for (const [key, value] of Object.entries(merged)) {
              toolCallMap[Number(key)] = value
            }
            patchSession(sessionId, (session) => ({
              ...session,
              messages: session.messages.map((item) =>
                item.id === assistantMessageId
                  ? {
                      ...item,
                      toolCalls: Object.values(toolCallMap).sort((a, b) => a.index - b.index),
                    }
                  : item
              ),
            }))
          },
        })

        patchSession(sessionId, (session) => {
          const traces = [...session.traces]
          if (traces[0]) {
            traces[0] = {
              ...traces[0],
              events,
              response: events[events.length - 1]
                ? {
                    ...events[events.length - 1],
                    usage: usageSnapshot,
                    cost: costSnapshot,
                  }
                : undefined,
            }
          }
          return { ...session, traces }
        })
      } else {
        const response = await chatApi.chatCompletions({ ...basePayload, stream: false })
        const output = extractAssistantOutput(response)
        patchSession(sessionId, (session) => ({
          ...session,
          messages: session.messages.map((item) =>
            item.id === assistantMessageId
              ? {
                  ...item,
                  content: output.content,
                  toolCalls: output.toolCalls.length > 0 ? output.toolCalls : undefined,
                }
              : item
          ),
          traces: session.traces.map((item, index) =>
            index === 0
              ? {
                  ...item,
                  response,
                  events: [],
                }
              : item
          ),
        }))
      }
    } catch (error) {
      if ((error as DOMException).name === 'AbortError') {
        message.warning('已停止生成')
      } else {
        const textError = error instanceof Error ? error.message : '聊天调用失败'
        patchSession(sessionId, (session) => ({
          ...session,
          messages: session.messages.map((item) =>
            item.id === assistantMessageId
              ? {
                  ...item,
                  content: item.content || `请求失败：${textError}`,
                }
              : item
          ),
          traces: session.traces.map((item, index) =>
            index === 0
              ? {
                  ...item,
                  error: textError,
                }
              : item
          ),
        }))
        message.error(textError)
      }
    } finally {
      setSending(false)
      abortRef.current = null
    }
  }

  const handleReplay = (msg: ChatMessage) => {
    void sendMessage(msg.content)
  }

  const stopStreaming = () => {
    abortRef.current?.abort()
  }

  const activeToolCalls = useMemo(() => {
    if (!activeSession) {
      return []
    }
    const latestAssistant = [...activeSession.messages]
      .reverse()
      .find((item) => item.role === 'assistant' && item.toolCalls && item.toolCalls.length > 0)
    return latestAssistant?.toolCalls ?? []
  }, [activeSession])

  return (
    <div className="chat-workbench">
      <div className="chat-workbench-sidebar">
        <Card
          title="会话"
          extra={
            <Button type="primary" icon={<PlusOutlined />} onClick={createNewSession}>
              新建
            </Button>
          }
          className="chat-panel-card"
        >
          <List
            className="chat-session-list"
            dataSource={sessions}
            locale={{ emptyText: '暂无会话' }}
            renderItem={(session) => (
              <List.Item
                className={`chat-session-item ${session.id === activeSessionId ? 'chat-session-item-active' : ''}`}
                onClick={() => {
                  setActiveSessionId(session.id)
                  setActiveTraceIndex(-1)
                }}
                actions={[
                  <Button
                    key="rename"
                    type="text"
                    size="small"
                    icon={<EditOutlined />}
                    onClick={(event) => {
                      event.stopPropagation()
                      renameSession(session)
                    }}
                  />,
                  <Button
                    key="delete"
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={(event) => {
                      event.stopPropagation()
                      deleteSession(session.id)
                    }}
                  />,
                ]}
              >
                <div className="chat-session-title">{session.title}</div>
                <Text type="secondary" className="chat-session-time">
                  {new Date(session.updatedAt).toLocaleString()}
                </Text>
              </List.Item>
            )}
          />
        </Card>
      </div>

      <div className="chat-workbench-main">
        <Card className="chat-panel-card chat-main-card">
          <div className="chat-main-header">
            <div>
              <Title level={5} className="chat-main-title">
                Chat Web
              </Title>
              <Text type="secondary">统一通过 /v1/chat/completions，支持流式和所有激活模型</Text>
            </div>
            <Space>
              <Button onClick={clearActiveSession}>清空会话</Button>
              <Button
                type="primary"
                icon={sending ? <StopOutlined /> : <SendOutlined />}
                onClick={() => (sending ? stopStreaming() : void sendMessage(draft))}
              >
                {sending ? '停止生成' : '发送'}
              </Button>
            </Space>
          </div>

          <div className="chat-messages-wrap">
            {!activeSession || activeSession.messages.length === 0 ? (
              <Empty description="开始你的第一条消息" />
            ) : (
              <div className="chat-message-list">
                {activeSession.messages.map((msg) => (
                  <div key={msg.id} className={`chat-message chat-message-${msg.role}`}>
                    <div className="chat-message-meta">
                      <Tag bordered={false}>{msg.role.toUpperCase()}</Tag>
                      <Text type="secondary">{new Date(msg.createdAt).toLocaleTimeString()}</Text>
                      {msg.role === 'user' && (
                        <Button
                          type="link"
                          size="small"
                          icon={<ReloadOutlined />}
                          onClick={() => handleReplay(msg)}
                        >
                          重放
                        </Button>
                      )}
                    </div>
                    <div className="chat-message-body">
                      {viewMode === 'rendered' ? (
                        <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]}>
                          {msg.content || ' '}
                        </ReactMarkdown>
                      ) : (
                        <pre>{msg.content || ''}</pre>
                      )}
                    </div>
                    {msg.toolCalls && msg.toolCalls.length > 0 && (
                      <div className="chat-tool-call-inline">
                        <Text strong>
                          <ToolOutlined /> 工具调用
                        </Text>
                        {msg.toolCalls.map((toolCall) => (
                          <pre key={`${msg.id}-${toolCall.index}`}>{JSON.stringify(toolCall, null, 2)}</pre>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="chat-composer">
            <TextArea
              rows={4}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onPressEnter={(event) => {
                if (event.shiftKey) {
                  return
                }
                event.preventDefault()
                void sendMessage(draft)
              }}
              placeholder="输入消息，Enter 发送，Shift+Enter 换行"
            />
          </div>
        </Card>
      </div>

      <div className="chat-workbench-right">
        <Card title="参数" className="chat-panel-card">
          {loadingModels ? (
            <Spin />
          ) : (
            <Space direction="vertical" className="chat-settings-stack">
              <div>
                <Text type="secondary">模型</Text>
                <Select
                  value={activeSession?.settings.model}
                  options={models.map((model) => ({ value: model, label: model }))}
                  onChange={(value) => {
                    if (!activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        model: value,
                      },
                    }))
                  }}
                  className="chat-full-width"
                />
              </div>

              <div>
                <Text type="secondary">参数模板</Text>
                <Select
                  value={templateKey}
                  options={TEMPLATE_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
                  onChange={(value) => {
                    setTemplateKey(value)
                    const template = TEMPLATE_OPTIONS.find((item) => item.value === value)
                    if (!template || !activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        temperature: template.settings.temperature,
                        topP: template.settings.topP,
                        maxTokens: template.settings.maxTokens,
                      },
                    }))
                  }}
                  className="chat-full-width"
                />
              </div>

              <div className="chat-setting-row">
                <Text type="secondary">流式输出</Text>
                <Switch
                  checked={activeSession?.settings.stream}
                  onChange={(checked) => {
                    if (!activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        stream: checked,
                      },
                    }))
                  }}
                />
              </div>

              <div>
                <Text type="secondary">Temperature</Text>
                <InputNumber
                  min={0}
                  max={2}
                  step={0.1}
                  value={activeSession?.settings.temperature}
                  onChange={(value) => {
                    if (value == null || !activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        temperature: Number(value),
                      },
                    }))
                  }}
                  className="chat-full-width"
                />
              </div>

              <div>
                <Text type="secondary">Top P</Text>
                <InputNumber
                  min={0}
                  max={1}
                  step={0.1}
                  value={activeSession?.settings.topP}
                  onChange={(value) => {
                    if (value == null || !activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        topP: Number(value),
                      },
                    }))
                  }}
                  className="chat-full-width"
                />
              </div>

              <div>
                <Text type="secondary">Max Tokens</Text>
                <InputNumber
                  min={1}
                  max={16384}
                  step={64}
                  value={activeSession?.settings.maxTokens}
                  onChange={(value) => {
                    if (value == null || !activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        maxTokens: Number(value),
                      },
                    }))
                  }}
                  className="chat-full-width"
                />
              </div>

              <div>
                <Text type="secondary">System Prompt</Text>
                <TextArea
                  rows={4}
                  value={activeSession?.settings.systemPrompt}
                  onChange={(event) => {
                    if (!activeSession) {
                      return
                    }
                    patchSession(activeSession.id, (session) => ({
                      ...session,
                      settings: {
                        ...session.settings,
                        systemPrompt: event.target.value,
                      },
                    }))
                  }}
                  placeholder="可选：系统提示词"
                />
              </div>
            </Space>
          )}
        </Card>

        <Card title="调试" className="chat-panel-card chat-debug-card">
          <Segmented
            value={viewMode}
            options={[
              { label: 'Markdown', value: 'rendered' },
              { label: 'Raw', value: 'raw' },
            ]}
            onChange={(value) => setViewMode(value as 'rendered' | 'raw')}
          />

          <div className="chat-trace-select-wrap">
            <Select
              value={activeTraceIndex >= 0 ? activeTraceIndex : undefined}
              placeholder="选择请求记录"
              options={(activeSession?.traces ?? []).map((trace, index) => ({
                value: index,
                label: `${index + 1}. ${trace.request.model}`,
              }))}
              onChange={(value) => setActiveTraceIndex(Number(value))}
              allowClear
              className="chat-full-width"
            />
          </div>

          <div className="chat-debug-content">
            {activeTrace ? (
              <pre>{JSON.stringify(activeTrace, null, 2)}</pre>
            ) : (
              <Empty description="暂无调试数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </div>

          <div className="chat-tool-calls-panel">
            <Text strong>
              <ToolOutlined /> 工具调用面板
            </Text>
            {activeToolCalls.length === 0 ? (
              <Text type="secondary">暂无工具调用</Text>
            ) : (
              activeToolCalls.map((toolCall) => (
                <pre key={`${toolCall.id}-${toolCall.index}`}>{JSON.stringify(toolCall, null, 2)}</pre>
              ))
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}

export default ChatWorkbench
