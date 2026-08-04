import React, { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts'
import axios from 'axios'

interface AlertStats {
  total_alerts: number
  webhook_count: number
  email_count: number
  slack_count: number
  success_rate: number
}

interface Alert {
  id: string
  timestamp: string
  source: string
  severity: string
  title: string
  channel: string
  status: string
  request_id: string
}

const Alerts: React.FC = () => {
  const [stats, setStats] = useState<AlertStats | null>(null)
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 30000)
    return () => clearInterval(interval)
  }, [])

  const fetchData = async () => {
    try {
      const [statsRes, alertsRes] = await Promise.all([
        axios.get('/api/v1/alerts/stats'),
        axios.get('/api/v1/alerts'),
      ])
      setStats(statsRes.data)
      setAlerts(alertsRes.data.alerts || [])
    } catch (error) {
      console.error('Failed to fetch alerts data:', error)
    } finally {
      setLoading(false)
    }
  }

  if (!stats) return <div>Loading...</div>

  const pieData = [
    { name: 'Webhook', value: stats.webhook_count, color: '#1890ff' },
    { name: 'Email', value: stats.email_count, color: '#52c41a' },
    { name: 'Slack', value: stats.slack_count, color: '#722ed1' },
  ]

  const columns: ColumnsType<Alert> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
    },
    {
      title: '来源',
      dataIndex: 'source',
      key: 'source',
      width: 100,
    },
    {
      title: '严重级别',
      dataIndex: 'severity',
      key: 'severity',
      width: 100,
      render: (severity) => {
        const colorMap: Record<string, string> = {
          'low': 'green',
          'medium': 'orange',
          'high': 'red',
          'critical': 'magenta',
        }
        return <Tag color={colorMap[severity] || 'default'}>{severity}</Tag>
      },
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
    },
    {
      title: '渠道',
      dataIndex: 'channel',
      key: 'channel',
      width: 100,
      render: (channel) => {
        const colorMap: Record<string, string> = {
          'webhook': 'blue',
          'email': 'green',
          'slack': 'purple',
        }
        return <Tag color={colorMap[channel] || 'default'}>{channel}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => {
        const colorMap: Record<string, string> = {
          'sent': 'success',
          'failed': 'error',
          'pending': 'processing',
        }
        return <Tag color={colorMap[status] || 'default'}>{status}</Tag>
      },
    },
  ]

  return (
    <div>
      <h2>告警</h2>
      
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总告警数" value={stats.total_alerts} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Webhook" value={stats.webhook_count} valueStyle={{ color: '#1890ff' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Email" value={stats.email_count} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="成功率" value={stats.success_rate} suffix="%" precision={2} valueStyle={{ color: '#3f8600' }} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={12}>
          <Card title="渠道分布">
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                  outerRadius={80}
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
          <Card title="最近告警">
            <Table
              columns={columns}
              dataSource={alerts}
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

export default Alerts