import React from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import MainLayout from './layouts/MainLayout'
import Dashboard from './pages/Dashboard'
import AuditLogs from './pages/AuditLogs'
import PII from './pages/PII'
import ContentSafety from './pages/ContentSafety'
import Alerts from './pages/Alerts'
import Configuration from './pages/Configuration'

const App: React.FC = () => {
  return (
    <ConfigProvider locale={zhCN}>
      <BrowserRouter basename="/dashboard">
        <Routes>
          <Route path="/" element={<MainLayout />}>
            <Route index element={<Navigate to="/overview" replace />} />
            <Route path="overview" element={<Dashboard />} />
            <Route path="audit-logs" element={<AuditLogs />} />
            <Route path="pii" element={<PII />} />
            <Route path="content-safety" element={<ContentSafety />} />
            <Route path="alerts" element={<Alerts />} />
            <Route path="configuration" element={<Configuration />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  )
}

export default App