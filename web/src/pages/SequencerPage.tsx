import { useEffect } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { SeqTaskList } from '../components/sequencer/SeqTaskList';
import { SeqDetail } from '../components/sequencer/SeqDetail';
import { useSequencerStore } from '../store/sequencerStore';

export function SequencerPage() {
  const fetchTasks = useSequencerStore((s) => s.fetchTasks);

  // The backend is the source of truth; reload on every mount so capture tasks
  // created elsewhere (e.g. "Send to Sequencer") appear.
  useEffect(() => {
    void fetchTasks();
  }, [fetchTasks]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Sequencer" subtitle="Analyze the randomness of session tokens" />
      <div className="flex min-h-0 flex-1">
        <SeqTaskList />
        <SeqDetail />
      </div>
    </div>
  );
}
