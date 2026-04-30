import React, { useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { DollarOutlined, PlusOutlined, TeamOutlined, UserOutlined, WalletOutlined } from '@ant-design/icons'
import { consoleTeamApi, consoleUserApi, orderApi, walletApi } from '../../services/api'
import type {
  ConsoleUser,
  RechargeCheckout,
  TeamCreate,
  TeamMemberCreate,
  TeamMemberRead,
  TeamInviteCreate,
  TeamInviteRead,
  TeamRead,
  WalletRead,
} from '../../services/types'

const { Paragraph, Title } = Typography

interface AdminPageProps {
  onOpenOrder: (orderNo: string, checkout?: RechargeCheckout | null) => void
}

const AdminPage: React.FC<AdminPageProps> = ({ onOpenOrder }) => {
  const [users, setUsers] = useState<ConsoleUser[]>([])
  const [teams, setTeams] = useState<TeamRead[]>([])
  const [selectedTeamId, setSelectedTeamId] = useState<number | null>(null)
  const [members, setMembers] = useState<TeamMemberRead[]>([])
  const [invites, setInvites] = useState<TeamInviteRead[]>([])
  const [teamWallet, setTeamWallet] = useState<WalletRead | null>(null)
  const [loading, setLoading] = useState(false)
  const [creatingTeam, setCreatingTeam] = useState(false)
  const [addingMember, setAddingMember] = useState(false)
  const [rechargingTeam, setRechargingTeam] = useState(false)
  const [creatingInvite, setCreatingInvite] = useState(false)
  const [teamModalOpen, setTeamModalOpen] = useState(false)
  const [memberModalOpen, setMemberModalOpen] = useState(false)
  const [rechargeModalOpen, setRechargeModalOpen] = useState(false)
  const [inviteModalOpen, setInviteModalOpen] = useState(false)
  const [teamForm] = Form.useForm<TeamCreate>()
  const [memberForm] = Form.useForm<TeamMemberCreate>()
  const [rechargeForm] = Form.useForm<{ amount: number; currency: string; payment_provider: string }>()
  const [inviteForm] = Form.useForm<TeamInviteCreate>()

  const loadBase = async () => {
    setLoading(true)
    try {
      const [usersResult, teamsResult] = await Promise.all([consoleUserApi.list(), consoleTeamApi.list()])
      setUsers(usersResult)
      setTeams(teamsResult)
      setSelectedTeamId((current) => current ?? teamsResult[0]?.id ?? null)
    } catch (error) {
      console.error(error)
      message.error('加载平台管理数据失败')
    } finally {
      setLoading(false)
    }
  }

  const loadMembers = async (teamId: number) => {
    try {
      const [items, wallet, teamInvites] = await Promise.all([
        consoleTeamApi.listMembers(teamId),
        walletApi.team(teamId),
        consoleTeamApi.listInvites(teamId),
      ])
      setMembers(items)
      setTeamWallet(wallet)
      setInvites(teamInvites)
    } catch (error) {
      console.error(error)
      message.error('加载团队成员失败')
    }
  }

  useEffect(() => {
    void loadBase()
  }, [])

  useEffect(() => {
    if (selectedTeamId != null) {
      void loadMembers(selectedTeamId)
    } else {
      setMembers([])
      setTeamWallet(null)
      setInvites([])
    }
  }, [selectedTeamId])

  const activeUsers = useMemo(() => users.filter((item) => item.status === 'active').length, [users])

  const handleUserStatus = async (user: ConsoleUser, status: 'active' | 'disabled') => {
    try {
      const updated = await consoleUserApi.update(user.id, { status })
      setUsers((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
      message.success(`用户状态已更新为 ${status}`)
    } catch (error) {
      console.error(error)
      message.error('更新用户状态失败')
    }
  }

  const handleCreateTeam = async () => {
    try {
      const values = await teamForm.validateFields()
      setCreatingTeam(true)
      const created = await consoleTeamApi.create(values)
      setTeams((prev) => [created, ...prev])
      setSelectedTeamId(created.id)
      setTeamModalOpen(false)
      teamForm.resetFields()
      message.success(`团队 ${created.name} 已创建`)
    } catch (error) {
      console.error(error)
      message.error('创建团队失败')
    } finally {
      setCreatingTeam(false)
    }
  }

  const handleMemberRole = async (member: TeamMemberRead, role: string) => {
    if (selectedTeamId == null) return
    try {
      const updated = await consoleTeamApi.updateMember(selectedTeamId, member.user_id, { role, status: member.status })
      setMembers((prev) => prev.map((item) => (item.user_id === updated.user_id ? updated : item)))
      message.success(`成员角色已调整为 ${role}`)
    } catch (error) {
      console.error(error)
      message.error('更新团队成员失败')
    }
  }

  const handleAddMember = async () => {
    if (selectedTeamId == null) return
    try {
      const values = await memberForm.validateFields()
      setAddingMember(true)
      const created = await consoleTeamApi.addMember(selectedTeamId, values)
      setMembers((prev) => [...prev, created])
      setMemberModalOpen(false)
      memberForm.resetFields()
      message.success('团队成员已添加')
    } catch (error) {
      console.error(error)
      message.error('添加团队成员失败')
    } finally {
      setAddingMember(false)
    }
  }

  const handleTeamRecharge = async () => {
    if (selectedTeamId == null) return
    try {
      const values = await rechargeForm.validateFields()
      setRechargingTeam(true)
      const result = await orderApi.createTeamRecharge(selectedTeamId, values)
      setRechargeModalOpen(false)
      rechargeForm.resetFields()
      await loadMembers(selectedTeamId)
      onOpenOrder(result.order.order_no, result.checkout)
      message.success(`团队充值订单已创建：${result.order.order_no}`)
    } catch (error) {
      console.error(error)
      message.error('创建团队充值订单失败')
    } finally {
      setRechargingTeam(false)
    }
  }

  const handleCreateInvite = async () => {
    if (selectedTeamId == null) return
    try {
      const values = await inviteForm.validateFields()
      setCreatingInvite(true)
      const created = await consoleTeamApi.createInvite(selectedTeamId, values)
      setInvites((prev) => [created, ...prev])
      setInviteModalOpen(false)
      inviteForm.resetFields()
      message.success(`邀请已创建：${created.email}`)
    } catch (error) {
      console.error(error)
      message.error('创建邀请失败')
    } finally {
      setCreatingInvite(false)
    }
  }

  return (
    <div className="monitor-page admin-page">
      <section className="page-hero compact-hero">
        <div className="page-kicker">Platform Admin</div>
        <Title level={1}>平台管理</Title>
        <Paragraph>
          统一管理平台用户、团队与成员角色。这里是平台化账户体系的运营入口，不再只是只读视图。
        </Paragraph>
      </section>

      <Row gutter={[16, 16]} className="summary-grid">
        <Col xs={24} md={8}>
          <Card className="metric-card">
            <Statistic title="Users" value={users.length} prefix={<UserOutlined />} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card className="metric-card">
            <Statistic title="Active Users" value={activeUsers} prefix={<WalletOutlined />} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card className="metric-card">
            <Statistic title="Teams" value={teams.length} prefix={<TeamOutlined />} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={13}>
          <Card className="editorial-card" title="控制台用户">
            <Table
              rowKey="id"
              loading={loading}
              dataSource={users}
              pagination={{ pageSize: 8 }}
              columns={[
                { title: '用户', dataIndex: 'display_name', render: (value, record) => value || record.email },
                { title: '邮箱', dataIndex: 'email' },
                {
                  title: '角色',
                  dataIndex: 'roles',
                  render: (roles: string[]) => roles.map((role) => <Tag key={role}>{role}</Tag>),
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (status: string) => <Tag color={status === 'active' ? 'success' : 'default'}>{status}</Tag>,
                },
                {
                  title: '操作',
                  render: (_, record) => (
                    <Space>
                      {record.status === 'active' ? (
                        <Button size="small" onClick={() => void handleUserStatus(record, 'disabled')}>
                          禁用
                        </Button>
                      ) : (
                        <Button size="small" type="primary" onClick={() => void handleUserStatus(record, 'active')}>
                          启用
                        </Button>
                      )}
                    </Space>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={11}>
          <Card
            className="editorial-card"
            title="团队目录"
            extra={
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setTeamModalOpen(true)}>
                新建团队
              </Button>
            }
          >
            <Table
              rowKey="id"
              loading={loading}
              dataSource={teams}
              pagination={{ pageSize: 8 }}
              rowSelection={{
                type: 'radio',
                selectedRowKeys: selectedTeamId != null ? [selectedTeamId] : [],
                onChange: (keys) => setSelectedTeamId(Number(keys[0])),
              }}
              columns={[
                { title: '团队', dataIndex: 'name' },
                { title: 'Slug', dataIndex: 'slug' },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (status: string) => <Tag color={status === 'active' ? 'blue' : 'default'}>{status}</Tag>,
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={8}>
          <Card
            className="editorial-card"
            title={selectedTeamId ? `团队钱包 · #${selectedTeamId}` : '团队钱包'}
            extra={
              <Button
                type="primary"
                icon={<DollarOutlined />}
                disabled={selectedTeamId == null}
                onClick={() => setRechargeModalOpen(true)}
              >
                团队充值
              </Button>
            }
          >
            <Statistic
              title="Balance"
              value={teamWallet?.balance ?? 0}
              suffix={teamWallet?.currency ?? 'CNY'}
              precision={2}
              prefix={<WalletOutlined />}
            />
            <Paragraph type="secondary" style={{ marginTop: 12 }}>
              选中团队后，可直接为团队钱包创建充值订单。
            </Paragraph>
          </Card>
        </Col>
        <Col xs={24} xl={16}>
          <Card
            className="editorial-card"
            title={selectedTeamId ? `团队成员 · #${selectedTeamId}` : '团队成员'}
            extra={
              <Space>
                <Button
                  icon={<PlusOutlined />}
                  disabled={selectedTeamId == null}
                  onClick={() => setInviteModalOpen(true)}
                >
                  发起邀请
                </Button>
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  disabled={selectedTeamId == null}
                  onClick={() => setMemberModalOpen(true)}
                >
                  添加成员
                </Button>
              </Space>
            }
          >
        <Table
          rowKey="id"
          loading={loading}
          dataSource={members}
          pagination={{ pageSize: 8 }}
          columns={[
            { title: '成员', dataIndex: 'display_name', render: (value, record) => value || record.user_email },
            { title: '邮箱', dataIndex: 'user_email' },
            {
              title: '角色',
              dataIndex: 'role',
              render: (role: string, record) => (
                <Select
                  value={role}
                  style={{ width: 160 }}
                  options={[
                    { label: 'team_owner', value: 'team_owner' },
                    { label: 'team_admin', value: 'team_admin' },
                    { label: 'billing', value: 'billing' },
                    { label: 'member', value: 'member' },
                  ]}
                  onChange={(nextRole) => void handleMemberRole(record, nextRole)}
                />
              ),
            },
            {
              title: '状态',
              dataIndex: 'status',
              render: (status: string) => <Tag color={status === 'active' ? 'success' : 'default'}>{status}</Tag>,
            },
          ]}
        />
          </Card>
        </Col>
      </Row>

      <Card className="editorial-card" title={selectedTeamId ? `团队邀请 · #${selectedTeamId}` : '团队邀请'}>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={invites}
          pagination={{ pageSize: 6 }}
          columns={[
            { title: '邮箱', dataIndex: 'email' },
            { title: '角色', dataIndex: 'role', render: (role: string) => <Tag>{role}</Tag> },
            { title: '状态', dataIndex: 'status', render: (status: string) => <Tag color="processing">{status}</Tag> },
            { title: '邀请码', dataIndex: 'invite_token', render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
          ]}
        />
      </Card>

      <Modal
        title="新建团队"
        open={teamModalOpen}
        onCancel={() => setTeamModalOpen(false)}
        onOk={() => void handleCreateTeam()}
        confirmLoading={creatingTeam}
        destroyOnClose
      >
        <Form form={teamForm} layout="vertical">
          <Form.Item name="name" label="团队名称" rules={[{ required: true, message: '请输入团队名称' }]}>
            <Input placeholder="Growth" />
          </Form.Item>
          <Form.Item name="slug" label="团队标识" rules={[{ required: true, message: '请输入 slug' }]}>
            <Input placeholder="growth" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="团队说明" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="添加团队成员"
        open={memberModalOpen}
        onCancel={() => setMemberModalOpen(false)}
        onOk={() => void handleAddMember()}
        confirmLoading={addingMember}
        destroyOnClose
      >
        <Form form={memberForm} layout="vertical" initialValues={{ role: 'member' }}>
          <Form.Item name="user_id" label="用户" rules={[{ required: true, message: '请选择用户' }]}>
            <Select
              showSearch
              placeholder="选择控制台用户"
              options={users.map((item) => ({
                label: `${item.display_name || item.email} (${item.email})`,
                value: item.id,
              }))}
            />
          </Form.Item>
          <Form.Item name="role" label="团队角色" rules={[{ required: true, message: '请选择角色' }]}>
            <Select
              options={[
                { label: 'team_admin', value: 'team_admin' },
                { label: 'billing', value: 'billing' },
                { label: 'member', value: 'member' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="团队充值"
        open={rechargeModalOpen}
        onCancel={() => setRechargeModalOpen(false)}
        onOk={() => void handleTeamRecharge()}
        confirmLoading={rechargingTeam}
        destroyOnClose
      >
        <Form
          form={rechargeForm}
          layout="vertical"
          initialValues={{ currency: 'CNY', payment_provider: 'stripe', amount: 100 }}
        >
          <Form.Item name="amount" label="充值金额" rules={[{ required: true, message: '请输入金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="currency" label="币种" rules={[{ required: true, message: '请输入币种' }]}>
            <Select options={[{ label: 'CNY', value: 'CNY' }, { label: 'USD', value: 'USD' }]} />
          </Form.Item>
          <Form.Item name="payment_provider" label="支付渠道" rules={[{ required: true, message: '请选择支付渠道' }]}>
            <Select
              options={[
                { label: 'Stripe', value: 'stripe' },
                { label: '支付宝', value: 'alipay' },
                { label: '微信支付', value: 'wechatpay' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="发起团队邀请"
        open={inviteModalOpen}
        onCancel={() => setInviteModalOpen(false)}
        onOk={() => void handleCreateInvite()}
        confirmLoading={creatingInvite}
        destroyOnClose
      >
        <Form form={inviteForm} layout="vertical" initialValues={{ role: 'member' }}>
          <Form.Item name="email" label="受邀邮箱" rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]}>
            <Input placeholder="new@example.com" />
          </Form.Item>
          <Form.Item name="role" label="邀请角色" rules={[{ required: true, message: '请选择角色' }]}>
            <Select
              options={[
                { label: 'team_admin', value: 'team_admin' },
                { label: 'billing', value: 'billing' },
                { label: 'member', value: 'member' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default AdminPage
