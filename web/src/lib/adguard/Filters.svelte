<script lang="ts">
	import { get, post, errMsg } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Toggle from '#lib/Toggle.svelte';

	type Filter = { id: number; name: string; url: string; enabled: boolean; rules_count: number; last_updated?: string };
	type Status = {
		enabled: boolean;
		interval: number;
		filters: Filter[] | null;
		whitelist_filters: Filter[] | null;
		user_rules: string[] | null;
	};

	let st = $state<Status | null>(null);
	let rules = $state('');
	let busy = $state(false);
	let add = $state({ name: '', url: '', whitelist: false });

	async function load() {
		try {
			st = await get<Status>('/adguard/ctl/filtering/status');
			rules = (st.user_rules ?? []).join('\n');
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}
	load();

	async function act(fn: () => Promise<unknown>, ok: string, reload = true) {
		busy = true;
		try {
			await fn();
			toast(ok);
			if (reload) await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	const setEnabled = (v: boolean) =>
		act(() => post('/adguard/ctl/filtering/config', { enabled: v, interval: st!.interval }), 'Filtering updated.');
	const setInterval_ = (h: number) =>
		act(() => post('/adguard/ctl/filtering/config', { enabled: st!.enabled, interval: h }), 'Update schedule saved.');
	const toggle = (f: Filter, whitelist: boolean, v: boolean) =>
		act(
			() => post('/adguard/ctl/filtering/set_url', { url: f.url, whitelist, data: { name: f.name, url: f.url, enabled: v } }),
			`${f.name} ${v ? 'enabled' : 'disabled'}.`
		);
	function remove(f: Filter, whitelist: boolean) {
		if (!confirm(`Delete list "${f.name}"?`)) return;
		act(() => post('/adguard/ctl/filtering/remove_url', { url: f.url, whitelist }), 'List deleted.');
	}
	const refresh = () =>
		act(async () => {
			await post('/adguard/ctl/filtering/refresh', { whitelist: false });
			await post('/adguard/ctl/filtering/refresh', { whitelist: true });
		}, 'Filter lists updated.');
	function addList(e: SubmitEvent) {
		e.preventDefault();
		act(async () => {
			await post('/adguard/ctl/filtering/add_url', add);
			add = { name: '', url: '', whitelist: false };
		}, 'List added.');
	}
	const saveRules = () =>
		act(
			() => post('/adguard/ctl/filtering/set_rules', { rules: rules.split('\n').map((r) => r.trim()).filter(Boolean) }),
			'Custom rules saved.'
		);

	const intervals = [
		[0, 'Never'],
		[1, 'Every hour'],
		[12, 'Every 12 hours'],
		[24, 'Every day'],
		[72, 'Every 3 days'],
		[168, 'Every week']
	] as const;
</script>

{#if st}
	<div class="space-y-4">
		<div class="card flex flex-wrap items-center justify-between gap-3">
			<div>
				<p class="font-medium">Filtering</p>
				<p class="hint">Turn off to disable all blocklists at once.</p>
			</div>
			<div class="flex flex-wrap items-center gap-3">
				<select
					class="input"
					value={st.interval}
					disabled={busy}
					onchange={(e) => setInterval_(Number(e.currentTarget.value))}
					aria-label="Update schedule"
				>
					{#each intervals as [h, l] (h)}
						<option value={h}>{l}</option>
					{/each}
				</select>
				<button class="btn-ghost" disabled={busy} onclick={refresh}>Update now</button>
				<Toggle checked={st.enabled} disabled={busy} label="Filtering" onchange={setEnabled} />
			</div>
		</div>

		{#each [['Blocklists', st.filters, false], ['Allowlists', st.whitelist_filters, true]] as const as [title, list, wl] (title)}
			<div class="card">
				<p class="mb-3 font-medium">{title}</p>
				<ul class="divide-y divide-zinc-200 text-sm dark:divide-zinc-800">
					{#each list ?? [] as f (f.url)}
						<li class="flex items-center gap-3 py-2">
							<Toggle checked={f.enabled} disabled={busy} label={f.name} onchange={(v) => toggle(f, wl, v)} />
							<div class="min-w-0 flex-1">
								<p class="truncate">{f.name}</p>
								<p class="hint truncate font-mono" title={f.url}>{f.url}</p>
							</div>
							<span class="hint tabular-nums">{f.rules_count.toLocaleString('en-GB')} rules</span>
							<button class="btn-danger text-xs" disabled={busy} onclick={() => remove(f, wl)}>Delete</button>
						</li>
					{:else}
						<li class="hint py-2">No lists yet.</li>
					{/each}
				</ul>
			</div>
		{/each}

		<form class="card space-y-3" onsubmit={addList}>
			<p class="font-medium">Add list</p>
			<div class="grid gap-3 sm:grid-cols-[1fr_2fr]">
				<input class="input" bind:value={add.name} required placeholder="Name, e.g. OISD" aria-label="List name" />
				<input class="input font-mono" bind:value={add.url} required placeholder="https://…/list.txt" aria-label="List URL" />
			</div>
			<div class="flex flex-wrap items-center justify-between gap-2">
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={add.whitelist} /> Allowlist (not a blocklist)
				</label>
				<button class="btn-primary" disabled={busy}>Add</button>
			</div>
		</form>

		<div class="card space-y-3">
			<div>
				<p class="font-medium">Custom rules</p>
				<p class="hint">
					One rule per line. Examples: <code>||ads.example.com^</code> to block,
					<code>@@||example.com^</code> to allow, <code>! note</code> for a comment.
				</p>
			</div>
			<textarea class="input h-48 w-full font-mono text-xs" bind:value={rules} spellcheck="false"></textarea>
			<div class="flex justify-end">
				<button class="btn-primary" disabled={busy} onclick={saveRules}>Save rules</button>
			</div>
		</div>
	</div>
{:else}
	<p class="hint">Loading…</p>
{/if}
