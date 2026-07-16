export function ago(value?: string) {
  if (!value) return '—';
  const time = new Date(value).getTime();
  if (Number.isNaN(time)) return String(value);
  const ms = Date.now() - time;
  const hours = Math.floor(ms / 36e5);
  if (hours < 1) return 'just now';
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export function initials(value?: string) {
  return String(value || '?')
    .slice(0, 2)
    .toUpperCase();
}

export function label(value: string) {
  return value.replaceAll('_', ' ');
}

export function tone(value: string) {
  const normalized = String(value || '').toLowerCase();
  if (/high|fail|blocked|error|stale|changes requested|ci failed/.test(normalized)) return 'danger';
  if (/medium|pending|review|waiting|warn|large/.test(normalized)) return 'warning';
  if (/success|pass|approved|low|merged|clear|complete/.test(normalized)) return 'success';
  if (/commit|pull|pr|sync|running|open/.test(normalized)) return 'blue';
  return 'neutral';
}
