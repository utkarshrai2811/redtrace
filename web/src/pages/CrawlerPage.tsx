import { useEffect } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { CrawlList } from '../components/crawler/CrawlList';
import { CrawlDetail } from '../components/crawler/CrawlDetail';
import { useCrawlerStore } from '../store/crawlerStore';

export function CrawlerPage() {
  const fetchTasks = useCrawlerStore((s) => s.fetchTasks);

  // The backend is the source of truth; reload on every mount so crawls
  // created elsewhere (e.g. "Send to Crawler") appear.
  useEffect(() => {
    void fetchTasks();
  }, [fetchTasks]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Crawler" subtitle="Discover content and endpoints from a seed URL" />
      <div className="flex min-h-0 flex-1">
        <CrawlList />
        <CrawlDetail />
      </div>
    </div>
  );
}
