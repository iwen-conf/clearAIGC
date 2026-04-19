import { BrowserRouter, Link, Route, Routes, useLocation } from 'react-router-dom'
import { ProLayout } from '@ant-design/pro-components'
import {
  FileTextOutlined,
  SettingOutlined,
  ThunderboltFilled,
} from '@ant-design/icons'
import WorkspacePage from '@/pages/WorkspacePage'
import SettingsPage from '@/pages/SettingsPage'

const menuRoute = {
  path: '/',
  routes: [
    { path: '/', name: '工作台', icon: <FileTextOutlined /> },
    { path: '/settings', name: '智能体配置', icon: <SettingOutlined /> },
  ],
}

function Shell() {
  const location = useLocation()
  return (
    <ProLayout
      title="Naturalize"
      logo={<ThunderboltFilled />}
      layout="side"
      fixSiderbar
      fixedHeader
      contentWidth="Fluid"
      siderWidth={220}
      route={menuRoute}
      location={{ pathname: location.pathname }}
      menuItemRender={(item, dom) =>
        item.path ? <Link to={item.path}>{dom}</Link> : dom
      }
      avatarProps={{ style: { display: 'none' } }}
      actionsRender={() => []}
      footerRender={() => (
        <div style={{ textAlign: 'center', padding: '12px 0', color: 'rgba(0,0,0,0.45)' }}>
          Naturalize · AI 文本润色工作台
        </div>
      )}
    >
      <Routes>
        <Route path="/" element={<WorkspacePage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Routes>
    </ProLayout>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Shell />
    </BrowserRouter>
  )
}
