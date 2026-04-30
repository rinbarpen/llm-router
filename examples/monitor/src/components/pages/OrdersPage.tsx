import React, { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Card, Col, Descriptions, Empty, Row, Space, Table, Tag, Typography, message } from 'antd'
import { CheckCircleOutlined, ClockCircleOutlined, LinkOutlined, QrcodeOutlined, ReloadOutlined } from '@ant-design/icons'
import { orderApi } from '../../services/api'
import type { RechargeCheckout, RechargeOrderRead } from '../../services/types'

const { Paragraph, Text, Title } = Typography

interface OrdersPageProps {
  checkout: RechargeCheckout | null
}

const POLLABLE_STATUSES = new Set(['pending', 'processing'])

const OrdersPage: React.FC<OrdersPageProps> = ({ checkout }) => {
  const [orders, setOrders] = useState<RechargeOrderRead[]>([])
  const [activeOrder, setActiveOrder] = useState<RechargeOrderRead | null>(null)
  const [selectedOrderNo, setSelectedOrderNo] = useState(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('order_no')?.trim() || checkout?.order_no || ''
  })
  const [loading, setLoading] = useState(false)
  const [polling, setPolling] = useState(false)

  const activeOrderNo = useMemo(() => selectedOrderNo || checkout?.order_no || '', [checkout, selectedOrderNo])

  useEffect(() => {
    if (checkout?.order_no) {
      setSelectedOrderNo(checkout.order_no)
    }
  }, [checkout])

  const loadOrders = async () => {
    try {
      const items = await orderApi.list()
      setOrders(items)
    } catch (error) {
      console.error(error)
      message.error('加载订单列表失败')
    }
  }

  const loadActiveOrder = async (silent = false) => {
    if (!activeOrderNo) {
      setActiveOrder(null)
      return
    }
    if (!silent) {
      setLoading(true)
    }
    try {
      const item = await orderApi.get(activeOrderNo)
      setActiveOrder(item)
      setOrders((prev) => {
        const exists = prev.some((entry) => entry.order_no === item.order_no)
        if (exists) {
          return prev.map((entry) => (entry.order_no === item.order_no ? item : entry))
        }
        return [item, ...prev]
      })
    } catch (error) {
      console.error(error)
      if (!silent) {
        message.error('加载订单详情失败')
      }
    } finally {
      if (!silent) {
        setLoading(false)
      }
    }
  }

  useEffect(() => {
    void loadOrders()
  }, [])

  useEffect(() => {
    void loadActiveOrder()
  }, [activeOrderNo])

  useEffect(() => {
    if (!activeOrder || !POLLABLE_STATUSES.has(activeOrder.status)) {
      setPolling(false)
      return
    }
    setPolling(true)
    const timer = window.setInterval(() => {
      void loadActiveOrder(true)
    }, 3000)
    return () => {
      window.clearInterval(timer)
      setPolling(false)
    }
  }, [activeOrder])

  const copyText = async (value: string, successText: string) => {
    try {
      await navigator.clipboard.writeText(value)
      message.success(successText)
    } catch (error) {
      console.error(error)
      message.error('复制失败')
    }
  }

  const orderColumns = [
    { title: '订单号', dataIndex: 'order_no', render: (value: string) => <Text code>{value}</Text> },
    {
      title: '归属',
      render: (_: unknown, record: RechargeOrderRead) => `${record.owner_type}#${record.owner_id}`,
    },
    { title: '金额', render: (_: unknown, record: RechargeOrderRead) => `${record.amount.toFixed(2)} ${record.currency}` },
    { title: '渠道', dataIndex: 'payment_provider' },
    {
      title: '状态',
      dataIndex: 'status',
      render: (status: string) => (
        <Tag color={status === 'paid' ? 'success' : 'processing'}>{status}</Tag>
      ),
    },
    {
      title: '操作',
      render: (_: unknown, record: RechargeOrderRead) => (
        <Button
          size="small"
          onClick={() => {
            const params = new URLSearchParams(window.location.search)
            params.set('order_no', record.order_no)
            window.history.replaceState({}, '', `${window.location.pathname}?${params.toString()}#orders`)
            setSelectedOrderNo(record.order_no)
            setActiveOrder(record)
          }}
        >
          查看
        </Button>
      ),
    },
  ]

  return (
    <div className="monitor-page orders-page">
      <section className="page-hero compact-hero">
        <div className="page-kicker">Orders & Payments</div>
        <Title level={1}>充值订单</Title>
        <Paragraph>
          查看个人与团队充值订单结果。订单创建后会在这里轮询状态，等待异步支付回调把钱包入账。
        </Paragraph>
      </section>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={10}>
          <Card
            className="editorial-card"
            title="支付结果"
            extra={
              <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void loadActiveOrder()}>
                刷新
              </Button>
            }
          >
            {activeOrder ? (
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                <Alert
                  type={activeOrder.status === 'paid' ? 'success' : 'info'}
                  showIcon
                  icon={activeOrder.status === 'paid' ? <CheckCircleOutlined /> : <ClockCircleOutlined />}
                  message={activeOrder.status === 'paid' ? '订单已支付' : '订单等待支付确认'}
                  description={
                    activeOrder.status === 'paid'
                      ? '支付回调已经完成，余额应已入账。'
                      : '系统每 3 秒轮询一次订单状态，也可以手动刷新。'
                  }
                />
                <Descriptions column={1} size="small" bordered>
                  <Descriptions.Item label="订单号">
                    <Text code>{activeOrder.order_no}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="归属">{`${activeOrder.owner_type}#${activeOrder.owner_id}`}</Descriptions.Item>
                  <Descriptions.Item label="金额">{`${activeOrder.amount.toFixed(2)} ${activeOrder.currency}`}</Descriptions.Item>
                  <Descriptions.Item label="支付渠道">{activeOrder.payment_provider}</Descriptions.Item>
                  <Descriptions.Item label="状态">
                    <Tag color={activeOrder.status === 'paid' ? 'success' : 'processing'}>{activeOrder.status}</Tag>
                    {polling && activeOrder.status !== 'paid' ? <Text type="secondary"> 轮询中</Text> : null}
                  </Descriptions.Item>
                </Descriptions>
                {checkout?.payment_url || checkout?.qr_code_text ? (
                  <Card size="small" title="支付凭据">
                    <Space direction="vertical" size="small" style={{ width: '100%' }}>
                      {checkout?.payment_url ? (
                        <Button
                          icon={<LinkOutlined />}
                          href={checkout.payment_url}
                          target="_blank"
                          rel="noreferrer"
                        >
                          打开支付链接
                        </Button>
                      ) : null}
                      {checkout?.payment_url ? (
                        <Button onClick={() => void copyText(checkout.payment_url || '', '支付链接已复制')}>
                          复制支付链接
                        </Button>
                      ) : null}
                      {checkout?.qr_code_text ? (
                        <>
                          <Text>
                            <QrcodeOutlined /> 二维码内容
                          </Text>
                          <Text code>{checkout.qr_code_text}</Text>
                          <Button onClick={() => void copyText(checkout.qr_code_text || '', '二维码内容已复制')}>
                            复制二维码内容
                          </Button>
                        </>
                      ) : null}
                    </Space>
                  </Card>
                ) : null}
              </Space>
            ) : (
              <Empty description={activeOrderNo ? '订单加载中或不存在' : '当前没有选中的订单'} />
            )}
          </Card>
        </Col>
        <Col xs={24} xl={14}>
          <Card className="editorial-card" title="最近订单">
            <Table
              rowKey="id"
              dataSource={orders}
              pagination={{ pageSize: 8 }}
              columns={orderColumns}
              locale={{ emptyText: <Empty description="暂无充值订单" /> }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default OrdersPage
