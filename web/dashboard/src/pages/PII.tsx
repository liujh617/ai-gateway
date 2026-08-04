import React, { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PieChart, Pie, Cell, Tooltip, Legend, ResponsiveContainer } from 'recharts'
import axios from 'axios'

interface PIIStats {
  total_violations: number
  phone_number_count: number
  id_card_count: number
  bank_card_count: number
  by_action: Array<{ action: string; count: number }>
}

interface PIIViolation {
  id: string
  timestamp: string
  type: string
  action: string
  request_id: string
  sample: string
}

const PII: React.FC = () => {
  const [stats, setStats] = useState<PIIStats | null>(null)
  const [violations, setViolations] = useState<PIIViolation[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 30000)
    return () => clearInterval(interval)
  }, [])

  const fetchData = async () => {
    try {
      const [statsRes, violationsRes] = await Promise.all([
        axios.get('/api/v1/pii/stats'),
        axios.get('/api/v1/pii/violations'),
      ])
      setStats(statsRes.data)
      setViolations(violationsRes.data.violations || [])
    } catch (error) {
      console.error('Failed to fetch PII data:', error)
    } finally {
      setLoading(false)
    }
  }

  if (!stats) return <div>Loading...</div>

  const pieData = [
    { name: '手机号', value: stats.phone_number_count, color: '#1890ff' },
    { name: '身份证', value: stats.id_card_count, color: '#52c41a' },
    { name: '银行卡', value: stats.bank_card_count, color: '#faad14' },
  ]

  const columns: ColumnsType<PIIViolation> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type) => {
        const colorMap: Record<string, string> = {
          'phone_number': 'blue',
          'id_card': 'green',
          'bank_card': 'orange',
        }
        return <Tag color={colorMap[type] || 'default'}>{type}</Tag>
      },
    },
    {
      title: '处理',
      dataIndex: 'action',
      key: 'action',
      width: 100,
      render: (action) => {
        const colorMap: Record<string, string> = {
          'alert': 'orange',
          'reject': 'red',
          'allow': 'green',
        }
        return <Tag color={colorMap[action] || 'default'}>{action}</Tag>
      },
    },
    {
      title: '样本',
      dataIndex: 'sample',
      key: 'sample',
      ellipsis: true,
    },
    {
      title: '请求ID',
      dataIndex: 'request_id',
      key: 'request_id',
      ellipsis: true,
    },
  ]

  return (
    <div>
      <h2>PII检测</h2>
      
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总违规数" value={stats.total_violations} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="手机号" value={stats.phone_number_count} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="身份证" value={stats.id_card_count} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="银行卡" value={stats.bank_card_count} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={12}>
          <Card title="PII类型分布">
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {pieData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="最近违规">
            <Table
              columns={columns}
              dataSource={violations}
              rowKey="id"
              loading={loading}
              pagination={{ pageSize: 5 }}
              size="small"
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default PII