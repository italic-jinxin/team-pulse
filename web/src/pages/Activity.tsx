import { useMemo, useState } from 'react';
import type { Row } from '../lib/api';
import { ago, initials } from '../lib/format';
import { Badge, Empty } from '../components/ui';
import { Dropdown } from '../components/Dropdown';

export function Activity({ rows, prs }: { rows: Row[]; prs: Row[] }) {
  const [query, setQuery] = useState('');
  const [repo, setRepo] = useState('All repositories');
  const [type, setType] = useState('All activity');
  const [actor, setActor] = useState('All people');
  const [range, setRange] = useState('30');
  const repositories = useMemo(
    () =>
      Array.from(new Set(rows.map((row) => String(row.repository || '')).filter(Boolean))).sort(),
    [rows],
  );
  const actors = useMemo(
    () => Array.from(new Set(rows.map((row) => String(row.actor || '')).filter(Boolean))).sort(),
    [rows],
  );
  const filtered = rows.filter((row) => {
    const matchesQuery = !query || JSON.stringify(row).toLowerCase().includes(query.toLowerCase());
    const matchesRepo = repo === 'All repositories' || row.repository === repo;
    const matchesType = type === 'All activity' || String(row.type || '').includes(type);
    const matchesActor = actor === 'All people' || row.actor === actor;
    const occurred = new Date(row.occurred_at).getTime();
    const matchesRange =
      range === 'all' ||
      (Number.isFinite(occurred) && Date.now() - occurred <= Number(range) * 864e5);
    return matchesQuery && matchesRepo && matchesType && matchesActor && matchesRange;
  });
  const commitCount = rows.filter((row) => String(row.type).includes('commit')).length;
  const reviewCount = rows.filter((row) => String(row.type).includes('review')).length;
  return (
    <>
      <div className="filters">
        <input
          placeholder="Search commits, PRs, members..."
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <Dropdown
          value={repo}
          onChange={setRepo}
          ariaLabel="Repository filter"
          options={[
            { value: 'All repositories', label: 'All repositories' },
            ...repositories.map((name) => ({ value: name, label: name })),
          ]}
        />
        <Dropdown
          value={actor}
          onChange={setActor}
          ariaLabel="Actor filter"
          options={[
            { value: 'All people', label: 'All people' },
            ...actors.map((name) => ({ value: name, label: name })),
          ]}
        />
        <Dropdown
          value={type}
          onChange={setType}
          ariaLabel="Activity type filter"
          options={[
            { value: 'All activity', label: 'All activity' },
            { value: 'commit', label: 'Commits' },
            { value: 'pr.', label: 'Pull requests' },
            { value: 'review', label: 'Reviews' },
          ]}
        />
        <Dropdown
          value={range}
          onChange={setRange}
          ariaLabel="Activity time range"
          options={[
            { value: '1', label: 'Today' },
            { value: '7', label: '7 days' },
            { value: '30', label: '30 days' },
            { value: 'all', label: 'All time' },
          ]}
        />
      </div>
      <div className="split">
        <section className="card">
          <div className="card-header">
            <h2>Activity Timeline</h2>
            <span>{filtered.length} events</span>
          </div>
          {filtered.length ? (
            <ul className="activity-list">
              {filtered.map((item, index) => (
                <li className="activity-item" key={item.id || index}>
                  <div className="activity-row">
                    <div className="avatar">{initials(item.actor)}</div>
                    <div className="activity-copy">
                      <strong>
                        {item.actor || 'GitHub'} · {activityLabel(item.type)}
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
              title="No matching activity"
              text="Try a different search, repository, or activity type."
            />
          )}
        </section>
        <div className="grid">
          <section className="card">
            <div className="card-header">
              <h2>Activity Breakdown</h2>
              <span>This sync window</span>
            </div>
            <div className="card-body">
              <div className="kpi-strip">
                <div className="mini-kpi">
                  <strong>{commitCount}</strong>
                  <span>Commits</span>
                </div>
                <div className="mini-kpi">
                  <strong>{prs.length}</strong>
                  <span>PRs loaded</span>
                </div>
                <div className="mini-kpi">
                  <strong>{reviewCount}</strong>
                  <span>Reviews</span>
                </div>
              </div>
            </div>
          </section>
          <section className="card">
            <div className="card-header">
              <h2>Top Repositories</h2>
              <span>By event volume</span>
            </div>
            <FocusList items={topCounts(rows, 'repository')} />
          </section>
        </div>
      </div>
    </>
  );
}

function activityLabel(type: string) {
  if (String(type).includes('commit')) return 'Commit';
  if (String(type).includes('review')) return 'Review';
  if (String(type).includes('pr.')) return 'Pull Request';
  return String(type || 'Activity');
}

function topCounts(rows: Row[], field: string) {
  const counts = new Map<string, number>();
  rows.forEach((row) => {
    const value = String(row[field] || 'Unknown');
    counts.set(value, (counts.get(value) || 0) + 1);
  });
  const max = Math.max(1, ...counts.values());
  return Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4)
    .map(([label, count]) => ({ label, count, percent: Math.round((count / max) * 100) }));
}

function FocusList({ items }: { items: { label: string; count: number; percent: number }[] }) {
  return (
    <div className="focus-list">
      {items.length ? (
        items.map((item) => (
          <div className="focus-row" key={item.label}>
            <span>{item.label}</span>
            <div className="bar">
              <span style={{ width: `${item.percent}%` }} />
            </div>
            <strong>{item.count}</strong>
          </div>
        ))
      ) : (
        <Empty
          title="No activity yet"
          text="Sync repositories to see activity distribution."
          compact
        />
      )}
    </div>
  );
}
