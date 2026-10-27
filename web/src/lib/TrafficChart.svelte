<script lang="ts">
 import type { TrafficSample } from './traffic';
 import { rate } from './format';
 let { samples, live }: { samples: TrafficSample[]; live: boolean } = $props();
 const latest = $derived(samples.at(-1));
 const peak = $derived(Math.max(1, ...samples.flatMap(s => [s.rx,s.tx])));
 function path(key: 'rx'|'tx') {
  const end = latest?.t ?? 0;
  return samples.map((s,i) => `${i ? 'L':'M'}${500-(end-s.t)/60000*500},${120-s[key]/peak*110}`).join(' ');
 }
</script>
<div class="panel-heading"><div><h2>Aktivitas jaringan</h2><p>Aliran data melalui server WireGuard.</p></div><span class="quiet-badge">60 detik terakhir</span></div>
<div class="chart-summary"><span><i class="legend"></i>Masuk <strong>{live && latest ? rate(latest.rx) : '—'}</strong></span><span><i class="legend purple"></i>Keluar <strong>{live && latest ? rate(latest.tx) : '—'}</strong></span></div>
{#if samples.length > 1}
 <div class="traffic-chart"><div class="chart-y"><span>{rate(peak)}</span><span>{rate(peak/2)}</span><span>0 B/s</span></div><svg viewBox="0 0 500 130" preserveAspectRatio="none" role="img" aria-label="Grafik kecepatan data masuk dan keluar server selama 60 detik"><path d="M0 10H500 M0 65H500 M0 120H500" stroke="var(--border)" stroke-dasharray="3 5" fill="none"/><path d={path('rx')} stroke="var(--accent)" stroke-width="2" fill="none" vector-effect="non-scaling-stroke"/><path d={path('tx')} stroke="var(--violet)" stroke-width="2" fill="none" vector-effect="non-scaling-stroke"/></svg><div class="chart-x"><span>−60 dtk</span><span>−30 dtk</span><span>Sampel terakhir</span></div></div>
{:else}<div class="chart-empty">{live ? 'Mengumpulkan sampel aktivitas jaringan…' : 'Menunggu koneksi pembaruan langsung…'}</div>{/if}
<p class="chart-footnote">{live ? 'Dikumpulkan sejak halaman dibuka. Arah trafik dari sisi server.' : 'Pembaruan terputus. Data terakhir tetap ditampilkan.'}</p>
