import { useEffect } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { Sidebar } from './components/layout/Sidebar';
import { DashboardPage } from './pages/DashboardPage';
import { ProxyPage } from './pages/ProxyPage';
import { SiteMapPage } from './pages/SiteMapPage';
import { RepeaterPage } from './pages/RepeaterPage';
import { DecoderPage } from './pages/DecoderPage';
import { ComparerPage } from './pages/ComparerPage';
import { SettingsPage } from './pages/SettingsPage';
import { useWebSocket } from './hooks/useWebSocket';
import { useProxyStore } from './store/proxyStore';

export default function App() {
  // Single WebSocket connection for the whole app.
  useWebSocket();

  const fetchIntercept = useProxyStore((s) => s.fetchIntercept);

  // Prime the intercept state so the sidebar badge is accurate before the
  // Proxy page mounts.
  useEffect(() => {
    void fetchIntercept();
  }, [fetchIntercept]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-canvas text-zinc-200">
      <Sidebar />
      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/proxy" element={<ProxyPage />} />
          <Route path="/sitemap" element={<SiteMapPage />} />
          <Route path="/repeater" element={<RepeaterPage />} />
          <Route path="/decoder" element={<DecoderPage />} />
          <Route path="/comparer" element={<ComparerPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}
