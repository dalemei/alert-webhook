import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Descriptions, Tag, List, Spin, Button, Space, Typography, Timeline, Empty } from 'antd';
import { ArrowLeftOutlined, RobotOutlined, SendOutlined, ExclamationCircleOutlined } from '@ant-design/icons';
import { getAlertDetail } from '../api';

const { Text, Paragraph } = Typography;

export default function AlertDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [detail, setDetail] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getAlertDetail(id).then(setDetail).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;
  if (!detail) return <Empty description="告警不存在" />;

  const alert = detail.alert;
  const analysis = detail.analysis;
  const notifications = detail.notifications || [];

  const severityColor = (s) => {
    if (s === 'critical') return 'red';
    if (s === 'warning') return 'orange';
    return 'blue';
  };

  const rootCauses = analysis?.root_causes ? JSON.parse(analysis.root_causes) : [];
  const actions = analysis?.actions ? JSON.parse(analysis.actions) : [];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/alerts')}>返回列表</Button>
        <h2 style={{ margin: 0 }}>告警详情 #{id}</h2>
      </Space>

      <Card title="基本信息" style={{ marginBottom: 16 }}>
        <Descriptions column={{ xs: 1, sm: 2 }} bordered size="small">
          <Descriptions.Item label="告警名称"><Text strong>{alert.alertname}</Text></Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={alert.status === 'firing' ? 'red' : 'green'}>
              {alert.status === 'firing' ? '🔥 触发中' : '✅ 已恢复'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="等级">
            <Tag color={severityColor(alert.severity)}>{alert.severity}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="实例">{alert.instance || '-'}</Descriptions.Item>
          <Descriptions.Item label="任务">{alert.job || '-'}</Descriptions.Item>
          <Descriptions.Item label="开始时间">{alert.starts_at || '-'}</Descriptions.Item>
          <Descriptions.Item label="结束时间" span={2}>{alert.ends_at && alert.ends_at !== '0001-01-01T00:00:00Z' ? alert.ends_at : '（未结束）'}</Descriptions.Item>
          <Descriptions.Item label="摘要" span={2}>{alert.summary || '-'}</Descriptions.Item>
          <Descriptions.Item label="描述" span={2}>{alert.description || '-'}</Descriptions.Item>
        </Descriptions>
      </Card>

      {analysis ? (
        <Card
          title={<span><RobotOutlined /> AI 根因分析</span>}
          style={{ marginBottom: 16 }}
          extra={<Tag>{analysis.duration_ms ? `${(analysis.duration_ms / 1000).toFixed(1)}s` : '-'}</Tag>}
        >
          <Descriptions column={1} bordered size="small" style={{ marginBottom: 16 }}>
            <Descriptions.Item label="类别">
              <Tag color="blue">{analysis.category}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="等级">
              <Tag color={severityColor(analysis.severity)}>{analysis.severity}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="概要">
              <Text>{analysis.summary}</Text>
            </Descriptions.Item>
          </Descriptions>

          <Card type="inner" title={<span><ExclamationCircleOutlined /> 可能根因</span>} style={{ marginBottom: 12 }}
            styles={{ header: { background: '#fff2e8' } }}>
            {rootCauses.length > 0 ? (
              <ol style={{ paddingLeft: 20 }}>
                {rootCauses.map((rc, i) => <li key={i}>{rc}</li>)}
              </ol>
            ) : <Text type="secondary">暂未分析</Text>}
          </Card>

          <Card type="inner" title="建议操作"
            styles={{ header: { background: '#e6f7ff' } }}>
            {actions.length > 0 ? (
              <ol style={{ paddingLeft: 20 }}>
                {actions.map((a, i) => <li key={i}>{a}</li>)}
              </ol>
            ) : <Text type="secondary">暂无建议</Text>}
          </Card>
        </Card>
      ) : (
        <Card style={{ marginBottom: 16 }}>
          <Empty description="暂无 AI 分析结果（可能分析中或已恢复的告警不进行分析）" />
        </Card>
      )}

      <Card title={<span><SendOutlined /> 通知记录</span>}>
        {notifications.length > 0 ? (
          <Timeline
            items={notifications.map(n => ({
              color: n.status === 'success' ? 'green' : 'red',
              children: (
                <span>
                  <Tag>{n.channel}</Tag>
                  {n.status === 'success' ? '发送成功' : `发送失败: ${n.error_msg}`}
                  <Text type="secondary" style={{ marginLeft: 12 }}>
                    {new Date(n.created_at).toLocaleString('zh-CN')}
                  </Text>
                </span>
              ),
            }))}
          />
        ) : <Empty description="暂无通知记录" />}
      </Card>
    </div>
  );
}
