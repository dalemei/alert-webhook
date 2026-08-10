import { BrowserRouter, Routes, Route, Link, useLocation } from 'react-router-dom';
import { Layout, Menu, ConfigProvider } from 'antd';
import { DashboardOutlined, AlertOutlined, SettingOutlined } from '@ant-design/icons';
import Dashboard from './pages/Dashboard';
import AlertList from './pages/AlertList';
import AlertDetail from './pages/AlertDetail';
import zhCN from 'antd/locale/zh_CN';

const { Header, Sider, Content } = Layout;

function AppLayout() {
  const location = useLocation();
  const selectedKey = location.pathname === '/' ? '/' : location.pathname.startsWith('/alerts') ? '/alerts' : location.pathname;

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider breakpoint="lg" collapsedWidth="0">
        <div style={{ height: 48, margin: 16, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <span style={{ color: '#fff', fontSize: 18, fontWeight: 'bold' }}>🛠 AIOps</span>
        </div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey]}
          items={[
            { key: '/', icon: <DashboardOutlined />, label: <Link to="/">仪表盘</Link> },
            { key: '/alerts', icon: <AlertOutlined />, label: <Link to="/alerts">告警列表</Link> },
          ]}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', borderBottom: '1px solid #f0f0f0', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span style={{ fontSize: 16, fontWeight: 500 }}>AIOps 智能告警分析平台</span>
          <span style={{ color: '#999', fontSize: 13 }}>Worker v2.0</span>
        </Header>
        <Content style={{ margin: 24 }}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/alerts" element={<AlertList />} />
            <Route path="/alerts/:id" element={<AlertDetail />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  );
}

export default function App() {
  return (
    <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: '#1677ff' } }}>
      <BrowserRouter>
        <AppLayout />
      </BrowserRouter>
    </ConfigProvider>
  );
}
