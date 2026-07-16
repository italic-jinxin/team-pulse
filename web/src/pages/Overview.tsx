import type { DashboardData, Row } from '../lib/api';
import { ago, initials } from '../lib/format';
import { Badge, Empty, Stat } from '../components/ui';
import type { PageName } from '../app/navigation';

export function Overview({
  data,
  onNavigate,
}: {
  data: DashboardData;
  onNavigate: (page: PageName) => void;
}) {
  const { status = {}, repositories = [], activity = [], prs = [], risks = [], auth = {} } = data;
  const openPRs = status.open_pull_requests || prs.length || 0;
  const failing = prs.filter((pullRequest: Row) =>
    String(pullRequest.ci_state || '')
      .toLowerCase()
      .includes('fail'),
  ).length;
  const waiting = prs.filter((pullRequest: Row) =>
    String(pullRequest.review_state || '')
      .toLowerCase()
      .includes('waiting'),
  ).length;
  const highRisks = risks.filter((risk) => risk.severity === 'high').length;

  return (
    <>
      {(!auth.authenticated || repositories.length === 0 || activity.length === 0) && (
        <Onboarding
          connected={!!auth.authenticated}
          hasRepos={repositories.length > 0}
          hasActivity={activity.length > 0}
          onNavigate={onNavigate}
        />
      )}
      <section className="insight-card">
        <div>
          <span className="pill blue">Insight Summary</span>
          <h2>{insightHeadline(highRisks, waiting, failing)}</h2>
          <p>{insightCopy(repositories.length, risks.length, waiting, failing)}</p>
        </div>
        <div className="tag-row">
          <button className="secondary compactbtn" onClick={() => onNavigate('Pull Requests')}>
            Review PR queue
          </button>
          <button className="secondary compactbtn" onClick={() => onNavigate('Risks')}>
            Open risks
          </button>
        </div>
      </section>
      <section className="grid metrics">
        <Stat
          label="Active Members"
          value={status.members || 0}
          detail="Contributors in synced repos"
        />
        <Stat label="Open PRs" value={openPRs} detail={`${waiting} waiting for review`} />
        <Stat
          label="CI Failed"
          value={failing}
          detail="Pull requests with failing checks"
          tone={failing ? 'warn' : ''}
        />
        <Stat
          label="Delivery Risks"
          value={status.open_risks || risks.length || 0}
          detail="Rules-based open signals"
          tone={status.open_risks || risks.length ? 'warn' : ''}
        />
      </section>
      <div className="grid overview-layout">
        <section className="card">
          <div className="card-header">
            <h2>Recent Engineering Activity</h2>
            <span>{activity.length} events</span>
          </div>
          <Feed rows={activity.slice(0, 7)} />
        </section>
        <div className="grid">
          <section className="card">
            <div className="card-header">
              <h2>Risk Signals</h2>
              <span>{risks.length} open</span>
            </div>
            <RiskList rows={risks.slice(0, 5)} />
          </section>
          <section className="card">
            <div className="card-header">
              <h2>Current Focus</h2>
              <span>By activity share</span>
            </div>
            <Focus rows={activity} />
          </section>
        </div>
      </div>
    </>
  );
}

function Onboarding({
  connected,
  hasRepos,
  hasActivity,
  onNavigate,
}: {
  connected: boolean;
  hasRepos: boolean;
  hasActivity: boolean;
  onNavigate: (page: PageName) => void;
}) {
  const steps = [
    ['Connect GitHub', connected, 'Use GitHub CLI or a fine-grained PAT.'],
    ['Choose repositories', hasRepos, 'Pick the repos TeamPulse should track.'],
    ['Run first sync', hasActivity, 'Load the last 30 days of engineering activity.'],
    ['Review risks', hasActivity, 'Check stale reviews, CI failures, and large PRs.'],
    ['Generate report', hasActivity, 'Create a local Markdown summary.'],
  ] as const;
  return (
    <section className="onboarding card">
      <div className="card-header">
        <h2>Finish setup</h2>
        <span>
          {steps.filter((step) => step[1]).length}/{steps.length} complete
        </span>
      </div>
      <div className="checklist">
        {steps.map(([label, done, copy], index) => (
          <div className={'checkitem ' + (done ? 'done' : '')} key={label}>
            <span>{done ? '✓' : index + 1}</span>
            <div>
              <strong>{label}</strong>
              <p>{copy}</p>
            </div>
          </div>
        ))}
      </div>
      <div className="card-body onboarding-actions">
        <button onClick={() => onNavigate('Settings')}>
          {connected ? 'Choose repositories' : 'Connect GitHub'}
        </button>
        <button className="secondary" disabled={!hasActivity} onClick={() => onNavigate('Reports')}>
          Generate report
        </button>
      </div>
    </section>
  );
}

function insightHeadline(highRisks: number, waiting: number, failing: number) {
  if (highRisks) return `${highRisks} high-priority risk${highRisks > 1 ? 's' : ''} need attention`;
  if (failing) return `${failing} pull request${failing > 1 ? 's have' : ' has'} failing CI`;
  if (waiting) return `${waiting} pull request${waiting > 1 ? 's are' : ' is'} waiting for review`;
  return 'Engineering flow looks clear';
}

function insightCopy(repos: number, risks: number, waiting: number, failing: number) {
  if (!repos)
    return 'Connect GitHub and choose repositories to start generating useful delivery signals.';
  if (risks)
    return `There are ${risks} open risk signals. Start with review waits (${waiting}) and failing checks (${failing}).`;
  return 'No open risk signals were detected in the synced data. Keep the dashboard current with regular syncs.';
}

export function Feed({ rows }: { rows: Row[] }) {
  return rows.length ? (
    <ul className="activity-list">
      {rows.map((item, index) => (
        <li className="activity-item" key={item.id || index}>
          <div className="activity-row">
            <div className="avatar">{initials(item.actor)}</div>
            <div className="activity-copy">
              <strong>
                {item.actor || 'GitHub'} · {item.type}
              </strong>
              <p>{item.title || 'Repository activity'}</p>
              <div className="meta">
                {item.repository || 'Unknown repo'} · {ago(item.occurred_at)}
              </div>
            </div>
            <Badge value={item.type} />
          </div>
        </li>
      ))}
    </ul>
  ) : (
    <Empty
      title="No activity synced yet"
      text="Connect GitHub and sync repositories to fill this timeline."
    />
  );
}

export function RiskList({ rows }: { rows: Row[] }) {
  return rows.length ? (
    <ul className="risk-list">
      {rows.map((risk, index) => (
        <li className="risk-item" key={risk.id || index}>
          <div className="risk-top">
            <span className="risk-title">{risk.reason}</span>
            <Badge value={risk.severity || 'risk'} />
          </div>
          <div className="risk-desc">
            {risk.repository}
            {risk.pr_number ? ` #${risk.pr_number}` : ''} ·{' '}
            {risk.suggested_action || 'Review the signal'}
          </div>
        </li>
      ))}
    </ul>
  ) : (
    <Empty
      title="No blockers detected"
      text="No stale reviews, failing checks, or large PR signals are currently open."
      compact
    />
  );
}

function Focus({ rows }: { rows: Row[] }) {
  const counts = new Map<string, number>();
  rows.forEach((row) => {
    const repo = String(row.repository || 'Unknown');
    counts.set(repo, (counts.get(repo) || 0) + 1);
  });
  const max = Math.max(1, ...counts.values());
  const items = Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4);
  return (
    <div className="focus-list">
      {items.length ? (
        items.map(([label, count]) => (
          <div className="focus-row" key={label}>
            <span>{label}</span>
            <div className="bar">
              <span style={{ width: `${Math.round((count / max) * 100)}%` }} />
            </div>
            <strong>{count}</strong>
          </div>
        ))
      ) : (
        <Empty title="No focus data" text="Sync repositories to calculate focus areas." compact />
      )}
    </div>
  );
}
