import { useEffect } from 'react';
import { ProxyToolbar } from '../components/proxy/ProxyToolbar';
import { TrafficTable } from '../components/proxy/TrafficTable';
import { DetailView } from '../components/proxy/DetailView';
import { InterceptPanel } from '../components/proxy/InterceptPanel';
import { useProxyStore } from '../store/proxyStore';

export function ProxyPage() {
  const fetchList = useProxyStore((s) => s.fetchList);
  const fetchHosts = useProxyStore((s) => s.fetchHosts);
  const fetchIntercept = useProxyStore((s) => s.fetchIntercept);

  useEffect(() => {
    void fetchList();
    void fetchHosts();
    void fetchIntercept();
  }, [fetchList, fetchHosts, fetchIntercept]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ProxyToolbar />

      {/* Top: traffic table. Bottom: request/response detail. */}
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-[3] flex-col overflow-hidden border-b border-zinc-800">
          <TrafficTable />
        </div>
        <div className="min-h-0 flex-[2] overflow-hidden bg-panel/30">
          <DetailView />
        </div>
      </div>

      <InterceptPanel />
    </div>
  );
}
