import { useMemo, useState } from 'react';
import type { Row } from '../lib/api';
import { ago } from '../lib/format';
import { Badge, Empty, Stat } from '../components/ui';
import { Dropdown } from '../components/Dropdown';

export function PullRequests({ rows }: { rows: Row[] }) {
  const [actionFilter, setActionFilter] = useState('All actions');
  const open = rows.filter((row) => row.state === 'open');
  const failing = rows.filter((row) => String(row.ci_state || '').includes('fail'));
  const waiting = rows.filter((row) => String(row.review_state || '').includes('waiting'));
  const blocked = rows.filter(
    (row) =>
      String(row.ci_state || '').includes('fail') ||
      String(row.review_state || '').includes('changes'),
  );
  const [selectedId, setSelectedId] = useState(String((open[0] || rows[0])?.id || ''));
  const selected = useMemo(
    () => rows.find((row) => String(row.id) === selectedId) || open[0] || rows[0],
    [rows, selectedId, open],
  );
  const queue = useMemo(
    () =>
      rows
        .filter((row) => actionFilter === 'All actions' || nextAction(row) === actionFilter)
        .sort((a, b) => actionRank(nextAction(a)) - actionRank(nextAction(b))),
    [rows, actionFilter],
  );
  return (
    <div className="pr-page">
      <section className="grid metrics pr-metrics">
        <Stat
          label="Open PRs"
          value={open.length}
          detail={`${waiting.length} waiting for review`}
        />
        <Stat label="Median Age" value={medianAge(open)} detail="Based on open PR creation time" />
        <Stat
          label="CI Failed"
          value={failing.length}
          detail="Pull requests with failing checks"
          tone={failing.length ? 'warn' : ''}
        />
        <Stat
          label="Merge Blocked"
          value={blocked.length}
          detail="Review or CI blockers"
          tone={blocked.length ? 'warn' : ''}
        />
      </section>
      <div className="filters">
        <Dropdown
          value={actionFilter}
          onChange={setActionFilter}
          ariaLabel="Pull request action filter"
          options={[
            { value: 'All actions', label: 'All actions' },
            { value: 'Fix CI', label: 'Fix CI' },
            { value: 'Review needed', label: 'Review needed' },
            { value: 'Ready to merge', label: 'Ready to merge' },
            { value: 'Reduce scope', label: 'Reduce scope' },
            { value: 'Monitor', label: 'Monitor' },
          ]}
        />
      </div>
      <div className="split">
        <section className="card tablewrap">
          <div className="card-header">
            <h2>Pull Request Queue</h2>
            <span>{queue.length} in view</span>
          </div>
          {queue.length ? (
            <table className="table">
              <thead>
                <tr>
                  <th>Pull Request</th>
                  <th>Owner</th>
                  <th>Next Action</th>
                  <th>Status</th>
                  <th>Age</th>
                </tr>
              </thead>
              <tbody>
                {queue.map((row) => (
                  <tr
                    className={String(row.id) === String(selected?.id) ? 'selected' : ''}
                    key={row.id}
                    onClick={() => setSelectedId(String(row.id))}
                  >
                    <td>
                      <strong>
                        #{row.number} {row.title}
                      </strong>
                      <div className="meta">{row.repository}</div>
                    </td>
                    <td>{row.author || '—'}</td>
                    <td>
                      <Badge value={nextAction(row)} />
                    </td>
                    <td>
                      <Badge value={status(row)} />
                    </td>
                    <td>{age(row.created_at || row.updated_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <Empty title="No pull requests match" text="Try another next-action filter." />
          )}
        </section>
        <section className="card pr-detail-card">
          <div className="card-header">
            <h2>Selected PR</h2>
            <span>{selected ? `#${selected.number}` : '—'}</span>
          </div>
          {selected ? (
            <SelectedPR row={selected} />
          ) : (
            <Empty title="Select a PR" text="Choose a pull request to see details." compact />
          )}
        </section>
      </div>
    </div>
  );
}

function SelectedPR({ row }: { row: Row }) {
  const additions = Number(row.additions || 0),
    deletions = Number(row.deletions || 0),
    changed = additions + deletions;
  const action = nextAction(row);
  const hints = riskHints(row);
  return (
    <div className="detail-panel pr-detail">
      <div className="pr-action-summary">
        <Badge value={action} />
        <div>
          <strong>{actionSummary(action)}</strong>
          <p>{actionCopy(action, row)}</p>
        </div>
      </div>
      <h3>{row.title}</h3>
      <p>
        {row.repository} · opened by {row.author || 'unknown'} · updated {ago(row.updated_at)}
      </p>
      <div className="pr-detail-grid">
        <Detail label="Review state" value={row.review_state || 'unknown'} />
        <Detail label="Checks" value={row.ci_state || 'unknown'} />
        <Detail label="State" value={row.state || 'unknown'} />
        <Detail label="Draft" value={row.draft ? 'Yes' : 'No'} />
        <Detail label="Created" value={ago(row.created_at)} />
        <Detail label="Updated" value={ago(row.updated_at)} />
      </div>
      <div className="change-box">
        <div>
          <strong>{changed.toLocaleString()}</strong>
          <span>changed lines</span>
        </div>
        <div>
          <strong className="plus">+{additions.toLocaleString()}</strong>
          <span>additions</span>
        </div>
        <div>
          <strong className="minus">-{deletions.toLocaleString()}</strong>
          <span>deletions</span>
        </div>
      </div>
      <div className="stack">
        <div>
          <strong>Risk hints</strong>
          {hints.length ? (
            <ul className="hint-list">
              {hints.map((hint) => (
                <li key={hint}>{hint}</li>
              ))}
            </ul>
          ) : (
            <div className="meta">No obvious blocker from synced metadata.</div>
          )}
        </div>
        <div>
          <strong>Suggested next step</strong>
          <div className="meta">{suggestion(action, row)}</div>
        </div>
        <div className="tag-row">
          <Badge value={row.state} />
          <Badge value={row.review_state || 'review unknown'} />
          <Badge value={row.ci_state || 'ci unknown'} />
          {row.draft ? <span className="pill neutral">Draft</span> : null}
        </div>
        {row.url ? (
          <a className="open-link" href={String(row.url)} target="_blank" rel="noreferrer">
            Open pull request ↗
          </a>
        ) : null}
      </div>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="mini-kpi">
      <strong>{String(value || '—')}</strong>
      <span>{label}</span>
    </div>
  );
}

function status(row: Row) {
  if (String(row.ci_state || '').includes('fail')) return 'CI failed';
  if (String(row.review_state || '').includes('waiting')) return 'Waiting review';
  if (String(row.review_state || '').includes('approved')) return 'Approved';
  return row.review_state || row.state || 'unknown';
}

function nextAction(row: Row) {
  if (String(row.ci_state || '').includes('fail')) return 'Fix CI';
  if (Number(row.additions || 0) + Number(row.deletions || 0) > 800) return 'Reduce scope';
  if (String(row.review_state || '').includes('waiting')) return 'Review needed';
  if (String(row.review_state || '').includes('approved') && row.state === 'open')
    return 'Ready to merge';
  return 'Monitor';
}

function actionSummary(action: string) {
  return (
    {
      'Fix CI': 'CI is blocking merge',
      'Review needed': 'Reviewer attention needed',
      'Reduce scope': 'PR may be too large',
      'Ready to merge': 'Approved and ready',
      Monitor: 'No immediate blocker',
    }[action] || 'Review PR'
  );
}

function actionCopy(action: string, row: Row) {
  if (action === 'Fix CI')
    return `Checks are ${row.ci_state || 'failing'}. Inspect the workflow before review/merge.`;
  if (action === 'Review needed')
    return `Review state is ${row.review_state || 'waiting'}. Assign or remind reviewers.`;
  if (action === 'Reduce scope')
    return 'This PR exceeds the large-change threshold. Consider splitting or adding review guidance.';
  if (action === 'Ready to merge')
    return 'Review is approved and no failing CI was detected in the synced data.';
  return 'Keep this PR visible and refresh after the next GitHub sync.';
}

function riskHints(row: Row) {
  const hints: string[] = [];
  const changed = Number(row.additions || 0) + Number(row.deletions || 0);
  if (String(row.ci_state || '').includes('fail')) hints.push('Failing CI detected.');
  if (String(row.review_state || '').includes('waiting')) hints.push('Waiting for review.');
  if (changed > 800) hints.push(`Large change size: ${changed.toLocaleString()} lines.`);
  if (row.draft) hints.push('Draft PR; may not be ready for full review.');
  return hints;
}

function suggestion(action: string, row: Row) {
  if (action === 'Fix CI')
    return 'Open the failed workflow, fix or rerun CI, then refresh TeamPulse.';
  if (action === 'Review needed')
    return `Ask for review on #${row.number}, or assign an explicit reviewer.`;
  if (action === 'Reduce scope')
    return 'Add a review guide or split the change before requesting broad review.';
  if (action === 'Ready to merge')
    return 'Confirm branch policy and merge when product/engineering owner is ready.';
  return 'Monitor for updates during the next sync window.';
}

function actionRank(action: string) {
  const ranks: Record<string, number> = {
    'Fix CI': 1,
    'Review needed': 2,
    'Reduce scope': 3,
    'Ready to merge': 4,
    Monitor: 5,
  };
  return ranks[action] || 9;
}

function age(value: string) {
  return ago(value).replace(' ago', '');
}

function medianAge(rows: Row[]) {
  if (!rows.length) return '—';
  const ages = rows
    .map((row) => Date.now() - new Date(row.created_at || row.updated_at).getTime())
    .filter(Number.isFinite)
    .sort((a, b) => a - b);
  const median = ages[Math.floor(ages.length / 2)] || 0;
  const days = median / 864e5;
  return days < 1 ? `${Math.max(1, Math.round(days * 24))}h` : `${days.toFixed(1)}d`;
}
