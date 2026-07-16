import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Shell } from '../components/Shell';
import { Loading } from '../components/ui';
import {
  api,
  cleanError,
  fetchDashboard,
  queryKeys,
  type DashboardData,
  type SyncJob,
} from '../lib/api';
import { ago } from '../lib/format';
import { defaultNotifications, notify, usePreference } from '../lib/preferences';
import type { PageName } from './navigation';
import { Activity } from '../pages/Activity';
import { Overview } from '../pages/Overview';
import { PullRequests } from '../pages/PullRequests';
import { Repositories } from '../pages/Repositories';
import { Reports } from '../pages/Reports';
import { Risks } from '../pages/Risks';
import { Settings } from '../pages/Settings';
import { Team } from '../pages/Team';

const SYNC_POLL_MS = 2500;
const SYNC_CLOSE_DELAY_MS = 5000;

export function App() {
  const [page, setPage] = useState<PageName>('Overview');
  const [activeJobId, setActiveJobId] = useState('');
  const [syncSelection, setSyncSelection] = useState<string[]>([]);
  const [notifications] = usePreference('teampulse.notifications', defaultNotifications);
  const notifiedRisks = useRef(
    new Set<string>(JSON.parse(localStorage.getItem('teampulse.notifiedRisks') || '[]')),
  );
  const lastJobStatus = useRef('');
  const weeklyReminderKey = useRef('');
  const queryClient = useQueryClient();
  const dashboard = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: fetchDashboard,
    staleTime: 15_000,
  });
  const activeJob = useQuery({
    queryKey: queryKeys.job(activeJobId),
    queryFn: async () => (await api<SyncJob[]>(`/jobs/${activeJobId}`))[0],
    enabled: !!activeJobId,
    refetchInterval: (query) => {
      const status = (query.state.data as SyncJob | undefined)?.status;
      return isTerminalJob(status) ? false : SYNC_POLL_MS;
    },
  });
  const syncTracked = useMutation({
    mutationFn: async () => {
      const repositories = (dashboard.data?.repositories || [])
        .map((repo) => String(repo.full_name))
        .filter(Boolean);
      if (!repositories.length) throw new Error('Select repositories in Settings before syncing.');
      setSyncSelection(repositories);
      return api<{ job_id: string }>('/repositories/sync', {
        method: 'POST',
        json: { repositories },
      });
    },
    onSuccess: (result) => {
      setActiveJobId(result.job_id);
      setPage('Settings');
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
    },
  });
  const data = dashboard.data;
  const connected = !!data?.auth?.authenticated;
  const riskCount = data?.risks?.length || 0;
  const openPRCount = data?.prs?.filter((pr) => pr.state === 'open').length || 0;
  const repoCount = data?.repositories?.length || 0;
  const lastSync = lastSyncLabel(data);

  useEffect(() => {
    if (activeJobId || !data?.jobs?.length) return;
    const running = data.jobs.find(isRecentActiveJob);
    if (running?.id) setActiveJobId(String(running.id));
  }, [activeJobId, data?.jobs]);

  useEffect(() => {
    const status = activeJob.data?.status;
    if (isTerminalJob(status)) {
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
    }
    if (!activeJob.data || lastJobStatus.current === status) return;
    lastJobStatus.current = String(status || '');
    if (status === 'completed' && notifications.syncSuccess)
      notify(
        'TeamPulse sync complete',
        activeJob.data.message || 'GitHub activity has been synced.',
      );
    if (status === 'failed' && notifications.syncFailed)
      notify(
        'TeamPulse sync failed',
        activeJob.data.error || activeJob.data.message || 'Open TeamPulse for details.',
      );
  }, [activeJob.data?.status, queryClient, notifications.syncSuccess, notifications.syncFailed]);

  useEffect(() => {
    if (!activeJob.data || !isTerminalJob(activeJob.data.status)) return;
    const finishedJobId = activeJob.data.id;
    const timer = window.setTimeout(() => {
      setActiveJobId((current) => (current === finishedJobId ? '' : current));
      setSyncSelection([]);
    }, SYNC_CLOSE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [activeJob.data?.id, activeJob.data?.status]);

  useEffect(() => {
    if (!data || !notifications.highRisk) return;
    const highRisks = (data.risks || []).filter((risk) => risk.severity === 'high');
    highRisks.slice(0, 3).forEach((risk) => {
      const id = String(risk.id || `${risk.repository}:${risk.pr_number}:${risk.reason}`);
      if (notifiedRisks.current.has(id)) return;
      notifiedRisks.current.add(id);
      notify(
        'High risk detected',
        `${risk.repository || 'Repository'}${risk.pr_number ? ` #${risk.pr_number}` : ''}: ${risk.reason || 'Needs attention'}`,
      );
    });
    localStorage.setItem(
      'teampulse.notifiedRisks',
      JSON.stringify(Array.from(notifiedRisks.current).slice(-200)),
    );
  }, [data?.risks, notifications.highRisk]);

  useEffect(() => {
    if (!notifications.weeklyReminder) return;
    const timer = window.setInterval(() => {
      const now = new Date();
      const day = now.toLocaleDateString('en-US', { weekday: 'long' });
      const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`;
      const key = `${now.toISOString().slice(0, 10)}:${time}`;
      if (
        day === notifications.weeklyReminderDay &&
        time === notifications.weeklyReminderTime &&
        weeklyReminderKey.current !== key
      ) {
        weeklyReminderKey.current = key;
        notify('Weekly report reminder', 'Generate your TeamPulse weekly engineering summary.');
      }
    }, 30_000);
    return () => window.clearInterval(timer);
  }, [
    notifications.weeklyReminder,
    notifications.weeklyReminderDay,
    notifications.weeklyReminderTime,
  ]);

  const handleSync = () => {
    if (!connected || repoCount === 0) {
      setPage('Settings');
      return;
    }
    syncTracked.mutate();
  };

  return (
    <Shell
      page={page}
      setPage={setPage}
      connected={connected}
      riskCount={riskCount}
      openPRCount={openPRCount}
      repoCount={repoCount}
      lastSync={lastSync}
      refreshing={dashboard.isFetching}
      syncing={
        syncTracked.isPending ||
        activeJob.data?.status === 'running' ||
        activeJob.data?.status === 'pending'
      }
      activeJob={activeJob.data}
      syncSelection={syncSelection}
      error={
        dashboard.error || syncTracked.error ? cleanError(dashboard.error || syncTracked.error) : ''
      }
      onRefresh={() => dashboard.refetch()}
      onSync={handleSync}
    >
      {dashboard.isLoading || !data ? (
        <Loading />
      ) : (
        <Page
          page={page}
          data={data}
          activeJobId={activeJobId}
          syncSelection={syncSelection}
          onJobStart={(id, selection) => {
            setActiveJobId(id);
            setSyncSelection(selection);
          }}
          onNavigate={setPage}
        />
      )}
    </Shell>
  );
}

function Page({
  page,
  data,
  activeJobId,
  syncSelection,
  onJobStart,
  onNavigate,
}: {
  page: PageName;
  data: DashboardData;
  activeJobId: string;
  syncSelection: string[];
  onJobStart: (id: string, selection: string[]) => void;
  onNavigate: (page: PageName) => void;
}) {
  if (page === 'Overview') return <Overview data={data} onNavigate={onNavigate} />;
  if (page === 'Activity') return <Activity rows={data.activity} prs={data.prs} />;
  if (page === 'Pull Requests') return <PullRequests rows={data.prs} />;
  if (page === 'Team') return <Team rows={data.members || []} activity={data.activity || []} />;
  if (page === 'Repositories')
    return (
      <Repositories
        rows={data.repositories || []}
        prs={data.prs || []}
        activity={data.activity || []}
      />
    );
  if (page === 'Risks') return <Risks rows={data.risks || []} />;
  if (page === 'Reports') return <Reports rows={data.reports || []} />;
  return (
    <Settings
      auth={data.auth || {}}
      jobs={data.jobs || []}
      externalJobId={activeJobId}
      externalSelection={syncSelection}
      onJobStart={onJobStart}
    />
  );
}

function lastSyncLabel(data?: DashboardData) {
  const jobs = data?.jobs || [];
  const completed = jobs.find((job) => job.status === 'completed' && job.ended_at);
  const latest = completed?.ended_at || jobs[0]?.created_at;
  return latest ? ago(latest) : 'never';
}

function isActiveJob(status?: string) {
  return status === 'pending' || status === 'running';
}
function isTerminalJob(status?: string) {
  return status === 'completed' || status === 'failed';
}
function isRecentActiveJob(job: Record<string, unknown>) {
  if (!isActiveJob(String(job.status))) return false;
  const raw = String(job.started_at || job.created_at || '');
  const timestamp = new Date(raw).getTime();
  return Number.isFinite(timestamp) && Date.now() - timestamp < 2 * 60 * 60 * 1000;
}
