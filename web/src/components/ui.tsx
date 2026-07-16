import type { ReactNode } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { tone } from '../lib/format';

export function Panel({
  title,
  meta,
  children,
}: {
  title: string;
  meta?: string;
  children: ReactNode;
}) {
  return (
    <section className="panel">
      <div className="panelhead">
        <h2>{title}</h2>
        {meta && <span>{meta}</span>}
      </div>
      {children}
    </section>
  );
}

export function Stat({
  label,
  value,
  detail,
  tone: statTone = '',
}: {
  label: string;
  value: ReactNode;
  detail?: string;
  tone?: string;
}) {
  return (
    <article className={'stat ' + statTone}>
      <strong>{value}</strong>
      <span>{label}</span>
      {detail && <small>{detail}</small>}
    </article>
  );
}

export function StatusChip({
  icon,
  label,
  tone,
}: {
  icon: ReactNode;
  label: string;
  tone: string;
}) {
  return (
    <span className={'statuschip ' + tone}>
      {icon}
      {label}
    </span>
  );
}

export function HealthItem({
  label,
  value,
  tone: healthTone = '',
}: {
  label: string;
  value: ReactNode;
  tone?: string;
}) {
  return (
    <div className={'healthitem ' + healthTone}>
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}

export function Badge({ value }: { value: unknown }) {
  const text = String(value || 'unknown')
    .toLowerCase()
    .replaceAll('_', ' ');
  return <span className={'pill ' + tone(text)}>{text}</span>;
}

export function Empty({
  title = 'Nothing here yet',
  text = 'Connect GitHub and sync a repository.',
  compact = false,
}: {
  title?: string;
  text?: string;
  compact?: boolean;
}) {
  return (
    <div className={'empty ' + (compact ? 'compact' : '')}>
      <CheckCircle2 size={compact ? 18 : 24} />
      <b>{title}</b>
      <span>{text}</span>
    </div>
  );
}

export function Loading() {
  return (
    <div className="loadinggrid">
      <div className="skeleton hero" />
      {[0, 1, 2, 3].map((i) => (
        <div className="skeleton stat" key={i} />
      ))}
      <div className="skeleton panel" />
      <div className="skeleton panel" />
    </div>
  );
}
