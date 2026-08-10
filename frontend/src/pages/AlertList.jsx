import { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Select, Input, Space, Card } from 'antd';
import { useNavigate } from 'react-router-dom';
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { getAlerts } from '../api';

const { Option } = Select;

export default function AlertList() {
  const [data, setData] = useState({ total: 0, alerts: [] });
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [filters, setFilters] = useState({ status: '', severity: '', category: '' });
  const navigate = useNavigate();

  const fetch = useCallback(() => {
    setLoading(true);
    const params = { page, size };
    if (filters.status) params.status = filters.status;
    if (filters.severity) params.severity = filters.severity;
    if (filters.category) params.category = filters.category;
    getAlerts(params).then(setData).catch(() => {}).finally(() => setLoading(false));
  }, [page, size, filters]);

  useEffect(() => { fetch(); }, [fetch]);

  const severityColor = (s) => {
    if (s === 'critical') return 'red';
    if (s === 'warning') return 'orange';
    return 'blue';
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 70, sorter: (a, b) => a.id - b.id },
    { title: '告警名称', dataIndex: 'alertname', width: 200 },
    { title: '状态', dataIndex: 'status', width: 90,
      render: (v) => <Tag color={v === 'firing' ? 'red' : 'green'}>{v === 'firing' ? '🔥 触发' : '✅ 恢复'}</Tag> },
    { title: '等级', dataIndex: 'severity', width: 90,
      render: (v) => <Tag color={severityColor(v)}>{v}</Tag> },
    { title: '实例', dataIndex: 'instance', ellipsis: true },
    { title: '任务', dataIndex: 'job', width: 120 },
    { title: '摘要', dataIndex: 'summary', ellipsis: true, width: 200 },
    { title: '开始时间', dataIndex: 'starts_at', width: 180,
      render: (v) => v || '-' },
    { title: '记录时间', dataIndex: 'created_at', width: 180,
      render: (v) => v ? new Date(v).toLocaleString('zh-CN') : '-' },
    { title: '操作', width: 80, fixed: 'right',
      render: (_, r) => <a onClick={() => navigate(`/alerts/${r.id}`)}>详情</a> },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>告警列表</h2>

      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap>
          <Select placeholder="状态" allowClear style={{ width: 100 }}
            value={filters.status || undefined}
            onChange={(v) => { setFilters(f => ({...f, status: v || ''})); setPage(1); }}>
            <Option value="firing">触发中</Option>
            <Option value="resolved">已恢复</Option>
          </Select>
          <Select placeholder="等级" allowClear style={{ width: 100 }}
            value={filters.severity || undefined}
            onChange={(v) => { setFilters(f => ({...f, severity: v || ''})); setPage(1); }}>
            <Option value="critical">critical</Option>
            <Option value="warning">warning</Option>
            <Option value="info">info</Option>
          </Select>
          <Input placeholder="搜索类别" prefix={<SearchOutlined />} style={{ width: 150 }}
            value={filters.category}
            onChange={(e) => { setFilters(f => ({...f, category: e.target.value})); setPage(1); }}
            onPressEnter={fetch} />
          <a onClick={fetch}><ReloadOutlined /> 刷新</a>
        </Space>
      </Card>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data.alerts}
        loading={loading}
        scroll={{ x: 1100 }}
        pagination={{
          current: page,
          pageSize: size,
          total: data.total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, s) => { setPage(p); setSize(s); },
        }}
        onRow={(r) => ({
          style: { cursor: 'pointer' },
          onDoubleClick: () => navigate(`/alerts/${r.id}`),
        })}
      />
    </div>
  );
}
