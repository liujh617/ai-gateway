import React, { useEffect, useState } from 'react'
import { Table, Input, Select, DatePicker, Space, Tag, Modal, Descriptions } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import axios from 'axios'

interface AuditLog {
  id: string
  timestamp: string
  provider: string
  model: string
  method: string
  path: string
  status_code: number
  duration_ms: number
  request_id: string
  has_pii: boolean
  has_content_safety: boolean
}

const AuditLogs: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [filters, setFilters] = useState({
    provider: '',
    model: '',
    status_code: '',
    start_time: '',
    end_time: '',
  })
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null)
  const [detailModal, setDetailModal] = useState(false)

  useEffect(() => {
    fetchLogs()
  }, [page, pageSize, filters])

  const fetchLogs = async () => {
    setLoading(true)
    try {
      const params = {
        page,
        page_size: pageSize,
        ...filters,
      }
      const response = await axios.get('/api/v1/audit/logs', { params })
      setLogs(response.data.logs)
      setTotal(response.data.total)
    } catch (error) {
      console.error('Failed to fetch audit logs:', error)
    } finally {
      setLoading(false)
    }
  }

  const columns: ColumnsType<AuditLog> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (text) => dayjs(text).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: 'Provider',
      dataIndex: 'provider',
      key: 'provider',
      width: 120,
    },
    {
      title: 'Model',
      dataIndex: 'model',
      key: 'model',
      width: 150,
    },
    {
      title: 'Method',
      dataIndex: 'method',
      key: 'method',
      width: 80,
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: 'Path',
      dataIndex: 'path',
      key: 'path',
      ellipsis: true,
    },
    {
      title: '状态码',
      dataIndex: 'status_code',
      key: 'status_code',
      width: 100,
      render: (code) => (
        <Tag color={code < 400 ? 'success' : 'error'}>{code}</Tag>
      ),
    },
    {
      title: '耗时(ms)',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 100,
      render: (ms) => `${ms.toFixed(2)}`,
    },
    {
      title: 'PII',
      dataIndex: 'has_pii',
      key: 'has_pii',
      width: 80,
      render: (flag) => flag ? <Tag color="red">有</Tag> : <Tag>无</Tag>,
    },
    {
      title: '内容安全',
      dataIndex: 'has_content_safety',
      key: 'has_content_safety',
      width: 100,
      render: (flag) => flag ? <Tag color="orange">有</Tag> : <Tag>无</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <a onClick={() => viewDetail(record)}>查看</a>
      ),
    },
  ]

  const viewDetail = async (log: AuditLog) => {
    try {
      const response = await axios.get(`/api/v1/audit/logs/${log.id}`)
      setSelectedLog(response.data)
      setDetailModal(true)
    } catch (error) {
      console.error('Failed to fetch log detail:', error)
    }
  }

  return (
    <div>
      <h2>审计日志</h2>
      
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          style={{ width: 150 }}
          placeholder="Provider"
          allowClear
          onChange={(value) => setFilters({ ...filters, provider: value || '' })}
        >
          <Select.Option value="openai">OpenAI</Select.Option>
          <Select.Option value="anthropic">Anthropic</Select.Option>
          <Select.Option value="azure">Azure</Select.Option>
        </Select>
        
        <Input
          style={{ width: 200 }}
          placeholder="Model"
          allowClear
          onChange={(e) => setFilters({ ...filters, model: e.target.value })}
        />
        
        <Select
          style={{ width: 120 }}
          placeholder="状态码"
          allowClear
          onChange={(value) => setFilters({ ...filters, status_code: value || '' })}
        >
          <Select.Option value="200">200</Select.Option>
          <Select.Option value="400">400</Select.Option>
          <Select.Option value="401">401</Select.Option>
          <Select.Option value="429">429</Select.Option>
          <Select.Option value="500">500</Select.Option>
        </Select>
        
        <DatePicker.RangePicker
          onChange={(dates) => {
            if (dates && dates[0] && dates[1]) {
              setFilters({
                ...filters,
                start_time: dates[0].toISOString(),
                end_time: dates[1].toISOString(),
              })
            }
          }}
        />
      </Space>

      <Table
        columns={columns}
        dataSource={logs}
        rowKey="id"
        loading={loading}
        pagination={{
          current: page,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (page, pageSize) => {
            setPage(page)
            setPageSize(pageSize)
          },
        }}
      />

      <Modal
        title="审计日志详情"
        open={detailModal}
        onCancel={() => setDetailModal(false)}
        footer={null}
        width={800}
      >
        {selectedLog && (
          <Descriptions bordered column={2}>
            <Descriptions.Item label="请求ID">{selectedLog.request_id}</Descriptions.Item>
            <Descriptions.Item label="时间">{selectedLog.timestamp}</Descriptions.Item>
            <Descriptions.Item label="Provider">{selectedLog.provider}</Descriptions.Item>
            <Descriptions.Item label="Model">{selectedLog.model}</Descriptions.Item>
            <Descriptions.Item label="Method">{selectedLog.method}</Descriptions.Item>
            <Descriptions.Item label="Path">{selectedLog.path}</Descriptions.Item>
            <Descriptions.Item label="状态码">{selectedLog.status_code}</Descriptions.Item>
            <Descriptions.Item label="耗时">{selectedLog.duration_ms}ms</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}

export default AuditLogs