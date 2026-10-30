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
<div class="chart-head">
 <span class="font-semibold text-[var(--text)]">Kecepatan, 60 detik terakhir</span>
 <span class="status ml-auto" class:online={live}><i class="status-dot"></i>{live ? 'Langsung' : 'Terputus'}</span>
</div>
{#if samples.length > 1}
 <div class="traffic-chart">
  <span class="chart-peak">{rate(peak)}</span>
  <svg viewBox="0 0 500 130" preserveAspectRatio="none" role="img" aria-label="Grafik kecepatan data masuk dan keluar server selama 60 detik terakhir">
   <path d="M0 120H500" stroke="var(--border)" fill="none" vector-effect="non-scaling-stroke"/>
   <path d="{path('rx')} L500,120 L{500-((latest?.t ?? 0)-(samples[0]?.t ?? 0))/60000*500},120Z" style="fill: color-mix(in srgb, var(--chart-rx) 14%, transparent)"/>
   <path d={path('rx')} stroke="var(--chart-rx)" stroke-width="1.5" fill="none" vector-effect="non-scaling-stroke"/>
   <path d={path('tx')} stroke="var(--chart-tx)" stroke-width="1.5" fill="none" vector-effect="non-scaling-stroke"/>
  </svg>
 </div>
{:else}<div class="chart-empty">{live ? 'Mengumpulkan sampel aktivitas jaringan…' : 'Menunggu koneksi pembaruan langsung…'}</div>{/if}
