import React, { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts'
import axios from 'axios'

interface ContentSafetyStats {
  total_violations: number
  politics_count: number
  pornography_count: number
  violence_count: number
  advertising_count: number
}

interface ContentSafetyViolation {
  id: string
  timestamp: string
  category: string
  action: string
  severity: string
  sample: string
  request_id: string
}

const ContentSafety: React.FC = () => {
  const [stats, setStats] = useState<ContentSafetyStats | null>(null)
  const [violations, setViolations] = useState<ContentSafetyViolation[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 30000)
    return () => clearInterval(interval)
  }, [])

  const fetchData = async () => {
    try {
      const [statsRes, violationsRes] = await Promise.all([
        axios.get('/api/v1/content-safety/stats'),
        axios.get('/api/v1/content-safety/violations'),
      ])
      setStats(statsRes.data)
      setViolations(violationsRes.data.violations || [])
    } catch (error) {
      console.error('Failed to fetch content safety data:', error)
    } finally {
      setLoading(false)
    }
  }

  if (!stats) return <div>Loading...</div>

  const pieData = [
    { name: '政治', value: stats.politics_count, color: '#ff4d4f' },
    { name: '色情', value: stats.pornography_count, color: '#ff7a45' },
    { name: '暴力', value: stats.violence_count, color: '#faad14' },
    { name: '广告', value: stats.advertising_count, color: '#52c41a' },
  ]

  const columns: ColumnsType<ContentSafetyViolation> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
    },
    {
      title: '类别',
      dataIndex: 'category',
      key: 'category',
      width: 100,
      render: (category) => {
        const colorMap: Record<string, string> = {
          'politics': 'red',
          'pornography': 'orange',
          'violence': 'gold',
          'advertising': 'green',
        }
        return <Tag color={colorMap[category] || 'default'}>{category}</Tag>
      },
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
      title: '处理',
      dataIndex: 'action',
      key: 'action',
      width: 100,
    },
    {
      title: '样本',
      dataIndex: 'sample',
      key: 'sample',
      ellipsis: true,
    },
  ]

  return (
    <div>
      <h2>内容安全检测</h2>
      
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总违规数" value={stats.total_violations} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="政治" value={stats.politics_count} valueStyle={{ color: '#ff4d4f' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="色情" value={stats.pornography_count} valueStyle={{ color: '#ff7a45' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="暴力" value={stats.violence_count} valueStyle={{ color: '#faad14' }} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={12}>
          <Card title="类别分布">
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

export default ContentSafety