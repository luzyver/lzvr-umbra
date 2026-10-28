<script lang="ts">
	import { get } from '#lib/api.ts';

	// Daftar centang layanan yang bisa diblokir (YouTube, TikTok, …); dipakai global dan per klien.
	type Service = { id: string; name: string; icon_svg: string };

	let { selected = $bindable([]), disabled = false }: { selected?: string[]; disabled?: boolean } = $props();

	let all = $state<Service[]>([]);
	let q = $state('');

	get<{ blocked_services: Service[] }>('/adguard/ctl/blocked_services/all')
		.then((r) => (all = r.blocked_services ?? []))
		.catch(() => (all = []));

	const shown = $derived(q ? all.filter((s) => s.name.toLowerCase().includes(q.toLowerCase())) : all);

	function set(id: string, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((x) => x !== id);
	}
</script>

<div class="space-y-2">
	<div class="flex flex-wrap items-center gap-2">
		<input class="input flex-1" bind:value={q} placeholder="Cari layanan…" aria-label="Cari layanan" />
		<span class="hint">{selected.length} diblokir</span>
		<button type="button" class="btn-ghost text-xs" {disabled} onclick={() => (selected = all.map((s) => s.id))}>Semua</button>
		<button type="button" class="btn-ghost text-xs" {disabled} onclick={() => (selected = [])}>Kosongkan</button>
	</div>
	<div class="grid max-h-80 gap-1 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3">
		{#each shown as s (s.id)}
			<label class="flex items-center gap-2 rounded-lg px-2 py-1 text-sm hover:bg-zinc-100 dark:hover:bg-zinc-800">
				<input type="checkbox" checked={selected.includes(s.id)} {disabled} onchange={(e) => set(s.id, e.currentTarget.checked)} />
				{#if s.icon_svg}
					<img src="data:image/svg+xml;base64,{s.icon_svg}" alt="" class="h-4 w-4 dark:invert" />
				{/if}
				<span class="truncate">{s.name}</span>
			</label>
		{:else}
			<p class="hint">{all.length ? 'Tidak ada yang cocok.' : 'Memuat…'}</p>
		{/each}
	</div>
</div>
