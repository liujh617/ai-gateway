import React, { useEffect, useState } from 'react'
import { Tabs, Card, Form, Input, Switch, Select, Button, Space, message, Table, Tag, Modal } from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import axios from 'axios'

const Configuration: React.FC = () => {
  return (
    <div>
      <h2>配置管理</h2>
      <Card style={{ marginTop: 24 }}>
        <Tabs
          defaultActiveKey="providers"
          items={[
            {
              key: 'providers',
              label: '模型配置',
              children: <ProviderConfig />,
            },
            {
              key: 'audit',
              label: '审计配置',
              children: <AuditConfig />,
            },
            {
              key: 'detection',
              label: '检测配置',
              children: <DetectionConfig />,
            },
          ]}
        />
      </Card>
    </div>
  )
}

// Provider Configuration Tab
const ProviderConfig: React.FC = () => {
  const [providers, setProviders] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalVisible, setModalVisible] = useState(false)
  const [editingProvider, setEditingProvider] = useState<any>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    fetchProviders()
  }, [])

  const fetchProviders = async () => {
    setLoading(true)
    try {
      const response = await axios.get('/api/v1/config/providers')
      setProviders(response.data.providers || [])
    } catch (error) {
      message.error('加载Provider配置失败')
    } finally {
      setLoading(false)
    }
  }

  const handleAdd = () => {
    setEditingProvider(null)
    form.resetFields()
    setModalVisible(true)
  }

  const handleEdit = (provider: any) => {
    setEditingProvider(provider)
    form.setFieldsValue(provider)
    setModalVisible(true)
  }

  const handleDelete = async (name: string) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除Provider "${name}" 吗？`,
      onOk: async () => {
        try {
          await axios.delete(`/api/v1/config/providers/${name}`)
          message.success('删除成功')
          fetchProviders()
        } catch (error) {
          message.error('删除失败')
        }
      },
    })
  }

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      if (editingProvider) {
        await axios.put(`/api/v1/config/providers/${editingProvider.name}`, values)
        message.success('更新成功')
      } else {
        await axios.post('/api/v1/config/providers', values)
        message.success('添加成功')
      }
      setModalVisible(false)
      fetchProviders()
    } catch (error) {
      message.error('保存失败')
    }
  }

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Base URL',
      dataIndex: 'base_url',
      key: 'base_url',
      ellipsis: true,
    },
    {
      title: 'API Key',
      dataIndex: 'api_key_masked',
      key: 'api_key_masked',
    },
    {
      title: '模型',
      dataIndex: 'models',
      key: 'models',
      render: (models: string[]) => (
        <Space>
          {models?.slice(0, 3).map(m => <Tag key={m}>{m}</Tag>)}
          {models?.length > 3 && <Tag>+{models.length - 3}</Tag>}
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(record)}>编辑</Button>
          <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.name)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>添加Provider</Button>
      </Space>
      
      <Table
        columns={columns}
        dataSource={providers}
        rowKey="name"
        loading={loading}
      />

      <Modal
        title={editingProvider ? '编辑Provider' : '添加Provider'}
        open={modalVisible}
        onOk={handleSave}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input disabled={!!editingProvider} />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="api_key" label="API Key" rules={[{ required: !editingProvider }]}>
            <Input.Password placeholder={editingProvider ? '留空保持不变' : ''} />
          </Form.Item>
          <Form.Item name="default_model" label="默认模型">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

// Audit Configuration Tab
const AuditConfig: React.FC = () => {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetchConfig()
  }, [])

  const fetchConfig = async () => {
    setLoading(true)
    try {
      const response = await axios.get('/api/v1/config/audit')
      form.setFieldsValue(response.data)
    } catch (error) {
      message.error('加载审计配置失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      await axios.put('/api/v1/config/audit', values)
      message.success('保存成功')
      fetchConfig()
    } catch (error) {
      message.error('保存失败')
    }
  }

  const handleGenerateKey = () => {
    Modal.confirm({
      title: '生成新密钥',
      content: '生成新的加密密钥将使旧密钥失效，确定继续吗？',
      onOk: async () => {
        try {
          const response = await axios.post('/api/v1/config/audit/key')
          message.success(response.data.message)
        } catch (error) {
          message.error('生成密钥失败')
        }
      },
    })
  }

  return (
    <div>
      <Form form={form} layout="vertical" loading={loading}>
        <Form.Item name="enabled" label="启用审计" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="path" label="审计日志路径">
          <Input />
        </Form.Item>
        <Form.Item name="max_file_bytes" label="最大文件大小(bytes)">
          <Input type="number" />
        </Form.Item>
        
        <Card title="加密设置" style={{ marginTop: 16 }}>
          <Form.Item name="encryption_enabled" label="启用加密" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="encryption_key_path" label="密钥路径">
            <Input disabled />
          </Form.Item>
          <Button type="primary" onClick={handleGenerateKey}>生成新密钥</Button>
        </Card>

        <Card title="PII审计设置" style={{ marginTop: 16 }}>
          <Form.Item name="log_pii" label="记录PII" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="redact_pii" label="脱敏PII" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Card>

        <Button type="primary" onClick={handleSave} style={{ marginTop: 16 }}>保存配置</Button>
      </Form>
    </div>
  )
}

// Detection Configuration Tab
const DetectionConfig: React.FC = () => {
  const [piiForm] = Form.useForm()
  const [csForm] = Form.useForm()
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetchConfig()
  }, [])

  const fetchConfig = async () => {
    setLoading(true)
    try {
      const [piiRes, csRes] = await Promise.all([
        axios.get('/api/v1/config/pii'),
        axios.get('/api/v1/config/content-safety'),
      ])
      piiForm.setFieldsValue(piiRes.data)
      csForm.setFieldsValue(csRes.data)
    } catch (error) {
      message.error('加载检测配置失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSavePII = async () => {
    try {
      const values = await piiForm.validateFields()
      await axios.put('/api/v1/config/pii', values)
      message.success('PII配置保存成功')
    } catch (error) {
      message.error('保存失败')
    }
  }

  const handleSaveCS = async () => {
    try {
      const values = await csForm.validateFields()
      await axios.put('/api/v1/config/content-safety', values)
      message.success('内容安全配置保存成功')
    } catch (error) {
      message.error('保存失败')
    }
  }

  return (
    <div>
      <Card title="PII检测配置" style={{ marginBottom: 16 }}>
        <Form form={piiForm} layout="vertical" loading={loading}>
          <Form.Item name="enabled" label="启用PII检测" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="action" label="处理模式">
            <Select>
              <Select.Option value="alert">告警(alert)</Select.Option>
              <Select.Option value="reject">拒绝(reject)</Select.Option>
              <Select.Option value="allow">允许(allow)</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="detect_phone_number" label="检测手机号" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="detect_id_card" label="检测身份证" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="detect_bank_card_number" label="检测银行卡" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Button type="primary" onClick={handleSavePII}>保存PII配置</Button>
        </Form>
      </Card>

      <Card title="内容安全检测配置">
        <Form form={csForm} layout="vertical" loading={loading}>
          <Form.Item name="enabled" label="启用内容安全检测" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="action" label="处理模式">
            <Select>
              <Select.Option value="alert">告警(alert)</Select.Option>
              <Select.Option value="reject">拒绝(reject)</Select.Option>
              <Select.Option value="allow">允许(allow)</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="threshold" label="阈值级别">
            <Select>
              <Select.Option value="low">低(low)</Select.Option>
              <Select.Option value="medium">中(medium)</Select.Option>
              <Select.Option value="high">高(high)</Select.Option>
            </Select>
          </Form.Item>
          <Button type="primary" onClick={handleSaveCS}>保存内容安全配置</Button>
        </Form>
      </Card>
    </div>
  )
}

export default Configuration