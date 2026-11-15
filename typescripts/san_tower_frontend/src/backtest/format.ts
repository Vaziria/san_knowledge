import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt';

// Formatting for the Backtest Analysis page. Times show in WIB (UTC+7),
// San's time, with UTC in tooltips where it matters.

export const WIB_OFFSET = 7 * 3600;

const pad = (n: number) => String(n).padStart(2, '0');

// wib formats Unix seconds as "2026-05-14 10:45" in WIB.
export function wib(sec: number, withTime = true) {
  const d = new Date((sec + WIB_OFFSET) * 1000);
  const day = `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
  return withTime ? `${day} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}` : day;
}

// utc formats Unix seconds as "2026-05-14 03:45 UTC".
export function utc(sec: number) {
  const d = new Date(sec * 1000);
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())} UTC`;
}

// sec is a Timestamp as Unix seconds (0 when unset).
export const sec = (t?: Timestamp) => (t ? Math.floor(timestampDate(t).getTime() / 1000) : 0);

export const money = (v: number) => v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export const pct = (v: number, digits = 1) => `${v.toFixed(digits)}%`;

// pf is a profit factor; "inf" with no losing trade.
export const pf = (v: number) => (Number.isFinite(v) ? v.toFixed(2) : 'inf');

export const signed = (v: number) => (v > 0 ? '+' : '') + money(v);

// price keeps up to 5 decimals without trailing zeros.
export const price = (v: number) => (v ? String(Number(v.toFixed(5))) : '-');

// range is a run's period in WIB days.
export const rangeText = (from?: Timestamp, to?: Timestamp) =>
  from && to ? `${wib(sec(from), false)} to ${wib(sec(to), false)}` : '-';

// chartColors are the charts' colours for the page's light or dark mode.
export function chartColors() {
  const dark = document.documentElement.classList.contains('dark');
  return dark
    ? { dark, bg: '#111111', text: '#d4d4d8', grid: '#27272a', up: '#22c55e', down: '#ef4444', win: '#4ade80', loss: '#f87171', stop: '#f87171', target: '#4ade80', entry: '#a1a1aa', equity: '#60a5fa', balance: '#a1a1aa', dd: '#f87171' }
    : { dark, bg: '#ffffff', text: '#27272a', grid: '#f4f4f5', up: '#16a34a', down: '#dc2626', win: '#15803d', loss: '#b91c1c', stop: '#dc2626', target: '#16a34a', entry: '#71717a', equity: '#1f6fd0', balance: '#a1a1aa', dd: '#dc2626' };
}

// compareColors tell the compared runs apart.
export const compareColors = ['#1f6fd0', '#e8590c', '#2f9e44', '#9c36b5', '#c2255c', '#0c8599', '#f08c00', '#5c7cfa'];

// runLink is a run page's link.
export const runLink = (path: string, folder?: string) =>
  `/backtest-analysis/run?path=${encodeURIComponent(path)}${folder ? `&folder=${encodeURIComponent(folder)}` : ''}`;
