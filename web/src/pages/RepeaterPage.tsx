import { useEffect } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { TabList } from '../components/repeater/TabList';
import { RepeaterEditor } from '../components/repeater/RepeaterEditor';
import { useRepeaterStore } from '../store/repeaterStore';

export function RepeaterPage() {
  const fetchTabs = useRepeaterStore((s) => s.fetchTabs);

  // The backend is the source of truth; reload the tab list on every mount so
  // tabs created elsewhere (e.g. "Send to Repeater") appear.
  useEffect(() => {
    void fetchTabs();
  }, [fetchTabs]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Repeater" subtitle="Craft, resend, and iterate on HTTP requests" />
      <div className="flex min-h-0 flex-1">
        <TabList />
        <RepeaterEditor />
      </div>
    </div>
  );
}
