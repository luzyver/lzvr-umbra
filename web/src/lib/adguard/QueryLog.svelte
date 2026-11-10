<script lang="ts">
	import { get, post, put, errMsg } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Toggle from '#lib/Toggle.svelte';

	type Entry = {
		time: string;
		client: string;
		question: { name: string; type: string };
		reason: string;
		elapsedMs: string;
		upstream?: string;
		rules?: { text: string }[];
	};
	type Config = { enabled: boolean; interval: number; anonymize_client_ip: boolean; ignored: string[] | null };

	let entries = $state<Entry[] | null>(null);
	let cfg = $state<Config | null>(null);
	let search = $state('');
	let blockedOnly = $state(false);
	let busy = $state(false);

	async function load() {
		const params = new URLSearchParams({ limit: '100' });
		if (search) params.set('search', search);
		if (blockedOnly) params.set('response_status', 'blocked');
		try {
			const r = await get<{ data: Entry[] | null }>(`/adguard/ctl/querylog?${params}`);
			entries = r.data ?? [];
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}
	load();
	get<Config>('/adguard/ctl/querylog/config')
		.then((r) => (cfg = r))
		.catch(() => (cfg = null));

	async function saveCfg(patch: Partial<Config>) {
		if (!cfg) return;
		busy = true;
		try {
			const next = { ...cfg, ...patch };
			await put('/adguard/ctl/querylog/config/update', next);
			cfg = next;
			toast('Log settings saved.');
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function clear() {
		if (!confirm('Delete the entire query log?')) return;
		busy = true;
		try {
			await post('/adguard/ctl/querylog_clear');
			toast('Log deleted.');
			await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	const blocked = (e: Entry) => e.reason.startsWith('Filtered') && e.reason !== 'FilteredWhiteList';
	const day = 24 * 3600 * 1000;
	const retention = [
		[6 * 3600 * 1000, '6 hours'],
		[day, '1 day'],
		[7 * day, '7 days'],
		[30 * day, '30 days'],
		[90 * day, '90 days']
	] as const;
</script>

<div class="space-y-4">
	{#if cfg}
		<div class="card flex flex-wrap items-center gap-x-6 gap-y-3 text-sm">
			<div class="flex items-center gap-2">
				<Toggle checked={cfg.enabled} disabled={busy} label="Log queries" onchange={(v) => saveCfg({ enabled: v })} />
				Log queries
			</div>
			<div class="flex items-center gap-2">
				<Toggle
					checked={cfg.anonymize_client_ip}
					disabled={busy}
					label="Anonymize device IP"
					onchange={(v) => saveCfg({ anonymize_client_ip: v })}
				/>
				Anonymize device IP
			</div>
			<label class="flex items-center gap-2">
				Keep for
				<select class="input" value={cfg.interval} disabled={busy} onchange={(e) => saveCfg({ interval: Number(e.currentTarget.value) })}>
					{#each retention as [ms, l] (ms)}
						<option value={ms}>{l}</option>
					{/each}
				</select>
			</label>
			<button class="btn-danger ml-auto text-xs" disabled={busy} onclick={clear}>Delete log</button>
		</div>
	{/if}

	<div class="card">
		<form
			class="mb-3 flex flex-wrap items-center gap-2"
			onsubmit={(e) => {
				e.preventDefault();
				load();
			}}
		>
			<input class="input flex-1" bind:value={search} placeholder="Search domain or device IP…" aria-label="Search" />
			<label class="flex items-center gap-1 text-sm">
				<input type="checkbox" bind:checked={blockedOnly} onchange={load} /> Blocked only
			</label>
			<button class="btn-ghost">Reload</button>
		</form>
		{#if entries}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead class="text-zinc-500">
						<tr>
							<th class="py-1 pr-3 font-medium">Time</th>
							<th class="py-1 pr-3 font-medium">Domain</th>
							<th class="py-1 pr-3 font-medium">Device</th>
							<th class="py-1 font-medium">Result</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-zinc-200 dark:divide-zinc-800">
						{#each entries as e, i (i)}
							<tr>
								<td class="whitespace-nowrap py-1 pr-3 tabular-nums">{new Date(e.time).toLocaleTimeString('en-GB')}</td>
								<td class="max-w-xs truncate py-1 pr-3 font-mono" title={e.question.name}>
									{e.question.name} <span class="text-zinc-500">{e.question.type}</span>
								</td>
								<td class="py-1 pr-3 font-mono">{e.client}</td>
								<td class="py-1" title={e.rules?.map((r) => r.text).join('\n')}>
									{#if blocked(e)}
										<span class="text-red-600 dark:text-red-400">blocked</span>
									{:else}
										<span class="text-zinc-500">ok · {Number(e.elapsedMs).toFixed(0)} ms</span>
									{/if}
								</td>
							</tr>
						{:else}
							<tr><td colspan="4" class="hint py-2">No entries.</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<p class="hint">Loading…</p>
		{/if}
	</div>
</div>
