<script lang="ts">
	import { get, post, errMsg } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';

	type Info = {
		upstream_dns: string[] | null;
		bootstrap_dns: string[] | null;
		upstream_mode: string;
		cache_size?: number;
	};

	let info = $state<Info | null>(null);
	let upstream = $state('');
	let bootstrap = $state('');
	let mode = $state('load_balance');
	let busy = $state(false);

	const lines = (s: string) => s.split('\n').map((l) => l.trim()).filter(Boolean);

	async function load() {
		try {
			info = await get<Info>('/adguard/ctl/dns_info');
			upstream = (info.upstream_dns ?? []).join('\n');
			bootstrap = (info.bootstrap_dns ?? []).join('\n');
			mode = info.upstream_mode || 'load_balance';
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}
	load();

	async function test() {
		busy = true;
		try {
			const r = await post<Record<string, string>>('/adguard/ctl/test_upstream_dns', {
				upstream_dns: lines(upstream),
				bootstrap_dns: lines(bootstrap)
			});
			const bad = Object.entries(r ?? {}).filter(([, v]) => v !== 'OK');
			if (bad.length === 0) toast('All upstreams responded.');
			else for (const [k, v] of bad) toast(`${k}: ${v}`, 'error');
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			await post('/adguard/ctl/dns_config', {
				upstream_dns: lines(upstream),
				bootstrap_dns: lines(bootstrap),
				upstream_mode: mode
			});
			toast('DNS settings saved.');
			await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	const modes = [
		['load_balance', 'Load balancing', 'Rotate between upstreams, favouring the fastest.'],
		['parallel', 'Parallel', 'Query all upstreams at once and use the fastest answer.'],
		['fastest_addr', 'Fastest address', 'Pick the IP with the fastest response (slower to answer).']
	] as const;
</script>

{#if info}
	<form class="space-y-4" onsubmit={save}>
		<div class="card space-y-3">
			<div>
				<p class="font-medium">Upstream DNS</p>
				<p class="hint">
					DNS servers AdGuard forwards to. One per line, e.g. <code>https://dns.cloudflare.com/dns-query</code>,
					<code>tls://dns.google</code> or <code>1.1.1.1</code>.
				</p>
			</div>
			<textarea class="input h-36 w-full font-mono text-xs" bind:value={upstream} spellcheck="false"></textarea>
			<div class="space-y-1">
				{#each modes as [v, l, d] (v)}
					<label class="flex items-start gap-2 text-sm">
						<input type="radio" name="mode" value={v} bind:group={mode} class="mt-1" />
						<span>{l} <span class="hint block">{d}</span></span>
					</label>
				{/each}
			</div>
		</div>

		<div class="card space-y-3">
			<div>
				<p class="font-medium">Bootstrap DNS</p>
				<p class="hint">Used to resolve the names of DoH/DoT upstreams. Must be IP addresses.</p>
			</div>
			<textarea class="input h-24 w-full font-mono text-xs" bind:value={bootstrap} spellcheck="false"></textarea>
		</div>

		<div class="flex justify-end gap-2">
			<button type="button" class="btn-ghost" disabled={busy} onclick={test}>Test upstreams</button>
			<button class="btn-primary" disabled={busy}>Save</button>
		</div>
	</form>
{:else}
	<p class="hint">Loading…</p>
{/if}
