import React from 'react'
import {
  MessageOutlined,
  FileTextOutlined,
  CodeOutlined,
  FileSearchOutlined,
  BarChartOutlined,
  FileImageOutlined,
  AudioOutlined,
  VideoCameraAddOutlined,
  BulbOutlined,
  ThunderboltOutlined,
  FileSyncOutlined,
  ApiOutlined,
  LinkOutlined,
  CodeSandboxOutlined,
  PartitionOutlined,
  CloudServerOutlined,
  UserOutlined,
  GoogleOutlined,
  GatewayOutlined,
  TranslationOutlined,
  TwitterOutlined,
  SettingOutlined,
  DollarCircleOutlined,
  GiftOutlined,
  LaptopOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  GithubOutlined,
  CrownOutlined,
  AppstoreOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons'

// 标签到图标的映射
const tagIconMap: Record<string, React.ComponentType<any>> = {
  // Functions (功能)
  general: AppstoreOutlined,
  chat: MessageOutlined,
  writing: FileTextOutlined,
  coding: CodeOutlined,
  summary: FileSearchOutlined,
  analysis: BarChartOutlined,
  'instruction-tuned': CheckCircleOutlined,
  planning: BarChartOutlined,

  // Abilities (能力)
  image: FileImageOutlined,
  audio: AudioOutlined,
  video: VideoCameraAddOutlined,
  reasoning: BulbOutlined,
  'long-context': FileSyncOutlined,
  'function-call': ApiOutlined,
  'web-search': LinkOutlined,
  'code-execution': CodeSandboxOutlined,
  agentic: PartitionOutlined,

  // Sources (来源/厂商)
  openai: CloudServerOutlined,
  claude: UserOutlined,
  gemini: GoogleOutlined,
  google: GoogleOutlined,
  openrouter: GatewayOutlined,
  qwen: TranslationOutlined,
  kimi: TranslationOutlined,
  glm: TranslationOutlined,
  'x-ai': TwitterOutlined,
  mistral: ThunderboltOutlined,
  ollama: DatabaseOutlined,
  vllm: DatabaseOutlined,
  custom: SettingOutlined,

  // Features (特性)
  cheap: DollarCircleOutlined,
  free: GiftOutlined,
  fast: ThunderboltOutlined,
  chinese: GlobalOutlined,
  local: LaptopOutlined,
  'open-source': GithubOutlined,
  'high-quality': CrownOutlined,
}

/**
 * 根据标签名称获取对应的图标组件
 * @param tagName 标签名称
 * @returns 图标组件，如果未找到则返回 null
 */
export function getTagIcon(tagName: string | null | undefined): React.ComponentType<any> | null {
  if (!tagName || typeof tagName !== 'string') {
    return null
  }
  return tagIconMap[tagName.toLowerCase()] || null
}

/**
 * 渲染带图标的标签内容
 * @param tagName 标签名称
 * @param text 显示的文本（默认为标签名称）
 * @returns React 元素
 */
export function renderTagWithIcon(tagName: string, text?: string): React.ReactNode {
  const Icon = getTagIcon(tagName)
  const displayText = text || tagName

  if (Icon) {
    return (
      <>
        <Icon style={{ marginRight: 4 }} />
        {displayText}
      </>
    )
  }

  return displayText
}
