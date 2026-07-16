import { useMemo, useState } from 'react';
import { Search } from 'lucide-react';
import type { Row } from '../lib/api';
import { ago, label } from '../lib/format';
import { Badge, Empty } from './ui';

export function DataTable({
  rows = [],
  cols,
  searchable = false,
  searchLabel = 'Filter',
}: {
  rows: Row[];
  cols: string[];
  searchable?: boolean;
  searchLabel?: string;
}) {
  const [query, setQuery] = useState('');
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return rows;
    return rows.filter((row) => JSON.stringify(row).toLowerCase().includes(needle));
  }, [rows, query]);

  return (
    <section className="panel tablewrap">
      <div className="tabletools">
        <div>
          <h2>{rows.length ? `${rows.length} records` : 'No records yet'}</h2>
          <p>
            {filtered.length !== rows.length
              ? `${filtered.length} match your filter`
              : 'Showing synced data'}
          </p>
        </div>
        {searchable && (
          <label className="searchbox">
            <Search size={16} />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={searchLabel}
            />
          </label>
        )}
      </div>
      {filtered.length ? (
        <table>
          <thead>
            <tr>
              {cols.map((col) => (
                <th key={col}>{label(col)}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((row, index) => (
              <tr key={row.id || row.login || index}>
                {cols.map((col) => (
                  <td key={col}>{cell(col, row[col], row)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <Empty
          title={rows.length ? 'No matches' : 'Nothing here yet'}
          text={
            rows.length
              ? 'Try a different filter.'
              : 'Connect GitHub and sync repositories to populate this view.'
          }
        />
      )}
    </section>
  );
}

function cell(column: string, value: unknown, row: Row) {
  if (column.endsWith('_at')) return <time>{ago(String(value || ''))}</time>;
  if (column === 'number') return value ? <span className="mono">#{String(value)}</span> : '—';
  if (column === 'repository') return <span className="repochip">{String(value || '—')}</span>;
  if (column === 'review_state' || column === 'ci_state' || column === 'type')
    return <Badge value={value || 'unknown'} />;
  if (column === 'title')
    return (
      <span className="titlecell">
        {String(value || '—')}
        {row.draft ? <small>Draft</small> : null}
      </span>
    );
  return String(value ?? '—');
}
