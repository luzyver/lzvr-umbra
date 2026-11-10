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
{#if samples.length > 1}
 <div class="traffic-chart">
  <span class="chart-peak">{rate(peak)}</span>
  <svg viewBox="0 0 500 130" preserveAspectRatio="none" role="img" aria-label="Chart of server inbound and outbound data rate over the last 60 seconds">
   <path d="M0 10H500 M0 65H500" stroke="var(--border)" stroke-dasharray="2 6" fill="none" vector-effect="non-scaling-stroke"/>
   <path d="M0 120H500" stroke="var(--border)" fill="none" vector-effect="non-scaling-stroke"/>
   <path d="{path('rx')} L500,120 L{500-((latest?.t ?? 0)-(samples[0]?.t ?? 0))/60000*500},120Z" style="fill: color-mix(in srgb, var(--chart-rx) 9%, transparent)"/>
   <path d={path('rx')} stroke="var(--chart-rx)" stroke-width="2" stroke-linejoin="round" fill="none" vector-effect="non-scaling-stroke"/>
   <path d={path('tx')} stroke="var(--chart-tx)" stroke-width="2" stroke-linejoin="round" fill="none" vector-effect="non-scaling-stroke"/>
  </svg>
 </div>
{:else}<div class="chart-empty">{live ? 'Collecting network activity samples…' : 'Waiting for the live update connection…'}</div>{/if}
<div class="chart-axis"><span>60 seconds ago</span><span class="chart-legend"><span><i class="legend"></i>In</span><span><i class="legend tx"></i>Out</span></span><span>{live ? 'Now' : 'Updates stopped'}</span></div>
