import React, { useMemo, useState } from 'react'
import { Button, Card, Input, Select, Space, Tabs, Upload, message, Typography } from 'antd'
import type { UploadFile } from 'antd/es/upload/interface'
import { multimodalApi } from '../services/api'

const { TextArea } = Input
const { Text } = Typography

const MultimodalWorkbench: React.FC = () => {
  const [model, setModel] = useState('openai/gpt-4.1')
  const [input, setInput] = useState('')
  const [prompt, setPrompt] = useState('')
  const [result, setResult] = useState<Record<string, any> | null>(null)
  const [videoJobId, setVideoJobId] = useState('')
  const [audioFile, setAudioFile] = useState<File | null>(null)
  const [loading, setLoading] = useState(false)

  const fileList = useMemo<UploadFile[]>(
    () =>
      audioFile
        ? [
            {
              uid: 'audio',
              name: audioFile.name,
              status: 'done',
            },
          ]
        : [],
    [audioFile]
  )

  const runEmbeddings = async () => {
    setLoading(true)
    try {
      const data = await multimodalApi.embeddings({ model, input })
      setResult(data)
      message.success('Embedding 调用成功')
    } catch (error) {
      console.error(error)
      message.error('Embedding 调用失败')
    } finally {
      setLoading(false)
    }
  }

  const runTTS = async () => {
    setLoading(true)
    try {
      const blob = await multimodalApi.speech({
        model,
        input,
        voice: 'alloy',
        response_format: 'mp3',
      })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'speech.mp3'
      a.click()
      URL.revokeObjectURL(url)
      message.success('TTS 生成成功，已下载音频')
    } catch (error) {
      console.error(error)
      message.error('TTS 调用失败')
    } finally {
      setLoading(false)
    }
  }

  const runASR = async (translate: boolean) => {
    if (!audioFile) {
      message.warning('请先上传音频文件')
      return
    }
    setLoading(true)
    try {
      const req = { model, file: audioFile, prompt }
      const data = translate ? await multimodalApi.translate(req) : await multimodalApi.transcribe(req)
      setResult(data)
      message.success(translate ? '音频翻译成功' : '音频转写成功')
    } catch (error) {
      console.error(error)
      message.error(translate ? '音频翻译失败' : '音频转写失败')
    } finally {
      setLoading(false)
    }
  }

  const runImage = async () => {
    setLoading(true)
    try {
      const data = await multimodalApi.generateImage({
        model,
        prompt: input,
        response_format: 'url',
      })
      setResult(data)
      message.success('生图请求已完成')
    } catch (error) {
      console.error(error)
      message.error('生图失败')
    } finally {
      setLoading(false)
    }
  }

  const runVideo = async () => {
    setLoading(true)
    try {
      const data = await multimodalApi.generateVideo({
        model,
        prompt: input,
        response_format: 'url',
      })
      setResult(data)
      if (data?.id) {
        setVideoJobId(String(data.id))
      }
      message.success('视频任务已创建')
    } catch (error) {
      console.error(error)
      message.error('生视频失败')
    } finally {
      setLoading(false)
    }
  }

  const queryVideoJob = async () => {
    if (!videoJobId.trim()) {
      message.warning('请输入任务 ID')
      return
    }
    setLoading(true)
    try {
      const data = await multimodalApi.getVideoJob(videoJobId.trim())
      setResult(data)
      message.success('任务状态已刷新')
    } catch (error) {
      console.error(error)
      message.error('查询任务失败')
    } finally {
      setLoading(false)
    }
  }

  const commonControls = (
    <Space direction="vertical" className="multimodal-controls">
      <Select
        value={model}
        onChange={setModel}
        options={[
          { value: 'openai/gpt-4.1', label: 'openai/gpt-4.1' },
          { value: 'openai/text-embedding-3-large', label: 'openai/text-embedding-3-large' },
          { value: 'openai/gpt-image-1', label: 'openai/gpt-image-1' },
        ]}
      />
      <TextArea value={input} onChange={(e) => setInput(e.target.value)} rows={4} placeholder="输入文本或提示词" />
    </Space>
  )

  const asrControls = (
    <Space direction="vertical" className="multimodal-controls">
      <Select value={model} onChange={setModel} options={[{ value: 'openai/whisper-1', label: 'openai/whisper-1' }]} />
      <Upload
        fileList={fileList}
        beforeUpload={(file) => {
          setAudioFile(file)
          return false
        }}
        onRemove={() => {
          setAudioFile(null)
        }}
        maxCount={1}
      >
        <Button>上传音频</Button>
      </Upload>
      <Input value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder="可选提示词" />
      <Space>
        <Button onClick={() => runASR(false)} loading={loading}>转写</Button>
        <Button onClick={() => runASR(true)} loading={loading}>翻译</Button>
      </Space>
    </Space>
  )

  const videoControls = (
    <Space direction="vertical" className="multimodal-controls">
      {commonControls}
      <Space className="multimodal-video-actions" wrap>
        <Button onClick={runVideo} loading={loading}>创建视频任务</Button>
        <Input value={videoJobId} onChange={(e) => setVideoJobId(e.target.value)} placeholder="任务 ID" className="multimodal-video-job-input" />
        <Button onClick={queryVideoJob} loading={loading}>查询任务</Button>
      </Space>
    </Space>
  )

  return (
    <Card title="多能力调试台" className="multimodal-workbench">
      <Tabs
        className="multimodal-tabs"
        items={[
          { key: 'embed', label: 'Embedding', children: <Space direction="vertical" className="multimodal-controls">{commonControls}<Button onClick={runEmbeddings} loading={loading}>执行 Embedding</Button></Space> },
          { key: 'tts', label: 'TTS', children: <Space direction="vertical" className="multimodal-controls">{commonControls}<Button onClick={runTTS} loading={loading}>生成语音</Button></Space> },
          {
            key: 'asr',
            label: 'ASR',
            children: asrControls,
          },
          { key: 'image', label: '生图', children: <Space direction="vertical" className="multimodal-controls">{commonControls}<Button onClick={runImage} loading={loading}>生成图片</Button></Space> },
          {
            key: 'video',
            label: '生视频',
            children: videoControls,
          },
        ]}
      />

      <Card size="small" title="结果" className="multimodal-result-card">
        <Text code className="multimodal-result-text">
          {result ? JSON.stringify(result, null, 2) : '暂无结果'}
        </Text>
      </Card>
    </Card>
  )
}

export default MultimodalWorkbench
