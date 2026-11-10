import type { Status } from './api';

export type TrafficSample = { t: number; rx: number; tx: number };

/** Rates use server direction, only peers observed in both snapshots. */
export function sampleTraffic(previous: Status | null, current: Status): TrafficSample | null {
 if (!previous) return null;
 const t = Date.parse(current.time);
 const dt = (t - Date.parse(previous.time)) / 1000;
 if (!Number.isFinite(dt) || dt <= 0) return null;
 const peers = new Map(previous.clients.map(c => [c.id, c]));
 let rx = 0, tx = 0;
 for (const c of current.clients) {
  const p = peers.get(c.id);
  if (!p) continue;
  rx += Math.max(0, c.rx_bytes - p.rx_bytes) / dt;
  tx += Math.max(0, c.tx_bytes - p.tx_bytes) / dt;
 }
 return { t, rx, tx };
}

/** A device counts as carrying traffic from this combined rate (bytes per second). */
export const BUSY_RATE = 1024;

/** Combined rx + tx rate per device, only for devices seen in both snapshots. */
export function clientRates(previous: Status | null, current: Status): Record<number, number> {
 const rates: Record<number, number> = {};
 if (!previous) return rates;
 const dt = (Date.parse(current.time) - Date.parse(previous.time)) / 1000;
 if (!Number.isFinite(dt) || dt <= 0) return rates;
 const peers = new Map(previous.clients.map(c => [c.id, c]));
 for (const c of current.clients) {
  const p = peers.get(c.id);
  if (p) rates[c.id] = (Math.max(0, c.rx_bytes - p.rx_bytes) + Math.max(0, c.tx_bytes - p.tx_bytes)) / dt;
 }
 return rates;
}

export function appendSample(history: TrafficSample[], sample: TrafficSample): TrafficSample[] {
 return [...history.filter(p => p.t >= sample.t - 60_000), sample].slice(-120);
}
