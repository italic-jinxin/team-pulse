import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Download, FileText } from 'lucide-react';
import type { Row } from '../lib/api';
import { api, queryKeys } from '../lib/api';
import { ago } from '../lib/format';
import { Empty, Stat } from '../components/ui';
import { Dropdown } from '../components/Dropdown';
import { usePreference, type ReportTemplate } from '../lib/preferences';

export function Reports({ rows }: { rows: Row[] }) {
  const queryClient = useQueryClient();
  const [report, setReport] = useState<Row | null>(null);
  const [reportType, setReportType] = useState('weekly');
  const [scope, setScope] = useState('all');
  const [length, setLength] = useState('standard');
  const [template, setTemplate] = usePreference<ReportTemplate>(
    'teampulse.reportTemplate',
    'executive',
  );
  const generateReport = useMutation({
    mutationFn: () =>
      api<Row>('/reports/generate', {
        method: 'POST',
        json: { type: reportType, scope, length, template },
      }),
    onSuccess: (newReport) => {
      setReport(newReport);
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
    },
  });
  const loadReport = useMutation({
    mutationFn: async (id: string) => (await api<Row[]>(`/reports/${id}`))[0],
    onSuccess: setReport,
  });

  const content = String(report?.markdown || 'Generate or select a report to preview it here.');
  return (
    <>
      <section className="grid metrics">
        <Stat label="Reports Generated" value={rows.length} detail="Stored locally" />
        <Stat
          label="Last Generated"
          value={rows[0]?.created_at ? ago(rows[0].created_at) : '—'}
          detail="Latest local report"
        />
        <Stat label="Export Format" value="MD" detail="Markdown is default" />
        <Stat label="Local Only" value="Yes" detail="No report data uploaded" />
      </section>
      <div className="split">
        <div>
          <section className="card report-options">
            <div className="card-header">
              <h2>Report Builder</h2>
              <span>{templateLabel(template)}</span>
            </div>
            <div className="card-body report-controls">
              <Dropdown
                label="Type"
                value={reportType}
                onChange={setReportType}
                options={[
                  { value: 'daily', label: 'Daily' },
                  { value: 'weekly', label: 'Weekly' },
                  { value: 'risk', label: 'Risk Summary' },
                ]}
              />
              <Dropdown
                label="Template"
                value={template}
                onChange={(value) => setTemplate(value as ReportTemplate)}
                options={[
                  { value: 'executive', label: 'Executive summary' },
                  { value: 'engineering', label: 'Engineering detail' },
                  { value: 'risk', label: 'Risk-focused' },
                  { value: 'standup', label: 'Standup-ready' },
                ]}
              />
              <Dropdown
                label="Scope"
                value={scope}
                onChange={setScope}
                options={[
                  { value: 'all', label: 'All repositories' },
                  { value: 'risks', label: 'Risk-heavy work' },
                ]}
              />
              <Dropdown
                label="Length"
                value={length}
                onChange={setLength}
                options={[
                  { value: 'brief', label: 'Brief' },
                  { value: 'standard', label: 'Standard' },
                  { value: 'detailed', label: 'Detailed' },
                ]}
              />
            </div>
          </section>
          <div className="actions report-actions">
            <button disabled={generateReport.isPending} onClick={() => generateReport.mutate()}>
              <FileText size={16} />
              {generateReport.isPending ? 'Generating' : `Generate ${label(reportType)} Report`}
            </button>
            <button
              className="secondary"
              disabled={!report}
              onClick={() => navigator.clipboard.writeText(content)}
            >
              Copy Markdown
            </button>
            <button
              className="secondary"
              disabled={!report}
              onClick={() => downloadMarkdown(content, report?.title)}
            >
              <Download size={16} />
              Download .md
            </button>
          </div>
          <section className="card report">
            <pre>{content}</pre>
          </section>
        </div>
        <section className="card">
          <div className="card-header">
            <h2>Report History</h2>
            <span>{rows.length} local files</span>
          </div>
          {rows.length ? (
            <ul className="member-list">
              {rows.map((row) => (
                <li
                  className="member-item member-select"
                  key={row.id}
                  onClick={() => loadReport.mutate(String(row.id))}
                >
                  <div className="member-top">
                    <span className="member-title">{row.title}</span>
                    <span className="pill success">Ready</span>
                  </div>
                  <div className="member-desc">{ago(row.created_at)} · Markdown</div>
                </li>
              ))}
            </ul>
          ) : (
            <Empty
              title="No reports yet"
              text="Generate a weekly report after syncing repositories."
              compact
            />
          )}
        </section>
      </div>
    </>
  );
}

function label(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
function templateLabel(value: string) {
  return (
    {
      executive: 'Executive summary',
      engineering: 'Engineering detail',
      risk: 'Risk-focused',
      standup: 'Standup-ready',
    }[value] || 'Template'
  );
}

function downloadMarkdown(markdown: string, title?: unknown) {
  const blob = new Blob([markdown], { type: 'text/markdown' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `${String(title || 'teampulse-report')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')}.md`;
  anchor.click();
  URL.revokeObjectURL(url);
}
