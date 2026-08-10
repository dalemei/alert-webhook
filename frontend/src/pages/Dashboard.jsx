import { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Table, Tag, Spin } from 'antd';
import {
  AlertOutlined, CheckCircleOutlined, ExperimentOutlined,
  ThunderboltOutlined, ClockCircleOutlined
} from '@ant-design/icons';
import { getStats, getAlerts } from '../api';

export default function Dashboard() {
  const [stats, setStats] = useState(null);
  const [recentAlerts, setRecentAlerts] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      getStats(),
      getAlerts({ page: 1, size: 5 })
    ]).then(([s, a]) => {
      setStats(s);
      setRecentAlerts(a.alerts || []);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const severityColor = (s) => {
    if (s === 'critical') return 'red';
    if (s === 'warning') return 'orange';
    return 'blue';
  };

  const statusColor = (s) => s === 'firing' ? 'red' : 'green';

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '告警名称', dataIndex: 'alertname', width: 160 },
    { title: '状态', dataIndex: 'status', width: 80,
      render: (v) => <Tag color={statusColor(v)}>{v === 'firing' ? '触发' : '恢复'}</Tag> },
    { title: '等级', dataIndex: 'severity', width: 80,
      render: (v) => <Tag color={severityColor(v)}>{v}</Tag> },
    { title: '实例', dataIndex: 'instance', ellipsis: true },
    { title: '时间', dataIndex: 'created_at', width: 180,
      render: (v) => v ? new Date(v).toLocaleString('zh-CN') : '-' },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>AIOps 告警分析平台</h2>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card><Statistic title="告警总数" value={stats?.total_alerts || 0} prefix={<AlertOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card><Statistic title="触发中" value={stats?.firing_count || 0} valueStyle={{ color: '#cf1322' }} prefix={<ThunderboltOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card><Statistic title="已恢复" value={stats?.resolved_count || 0} valueStyle={{ color: '#3f8600' }} prefix={<CheckCircleOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card><Statistic title="AI 分析数" value={stats?.analyzed_count || 0} prefix={<ExperimentOutlined />} /></Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} sm={12} md={6}>
          <Card><Statistic title="今日告警" value={stats?.today_count || 0} /></Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card><Statistic title="平均分析耗时" value={stats?.avg_analysis_ms ? `${(stats.avg_analysis_ms / 1000).toFixed(1)}s` : '-'} prefix={<ClockCircleOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic title="分类统计" value={stats?.by_category?.length || 0} suffix="类" />
            <div style={{ marginTop: 8 }}>
              {(stats?.by_category || []).slice(0, 4).map(c => (
                <Tag key={c.category}>{c.category}: {c.count}</Tag>
              ))}
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic title="等级分布" value={stats?.by_severity?.length || 0} suffix="级" />
            <div style={{ marginTop: 8 }}>
              {(stats?.by_severity || []).map(s => (
                <Tag key={s.severity} color={severityColor(s.severity)}>{s.severity}: {s.count}</Tag>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      <Card title="最近告警" style={{ marginTop: 24 }}>
        <Table rowKey="id" columns={columns} dataSource={recentAlerts} pagination={false} size="small" />
      </Card>
    </div>
  );
}
