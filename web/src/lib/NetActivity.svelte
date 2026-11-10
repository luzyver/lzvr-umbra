<script lang="ts">
	import { onDestroy } from 'svelte';
	import { get, errMsg, type Client, type Conn, type ConnList } from '#lib/api.ts';
	import { session } from '#lib/session.svelte.ts';
	import { bytes, ago, date } from '#lib/format.ts';

	let { client }: { client: Client } = $props();

	type Entry = {
		time: string;
		question: { name: string; type: string };
		reason: string;
		answer?: { type: string; value: string }[];
	};

	const ports: Record<number, string> = {
		20: 'FTP', 21: 'FTP', 22: 'SSH', 25: 'SMTP', 53: 'DNS', 80: 'HTTP', 110: 'POP3', 123: 'NTP',
		143: 'IMAP', 443: 'HTTPS', 465: 'SMTPS', 587: 'SMTP', 853: 'DNS-over-TLS', 993: 'IMAPS',
		995: 'POP3S', 1194: 'OpenVPN', 3389: 'RDP', 3478: 'STUN', 5222: 'XMPP/WhatsApp', 5228: 'Google Play',
		8080: 'HTTP-alt', 8443: 'HTTPS-alt', 19302: 'Google Meet', 51820: 'WireGuard'
	};
	const service = (c: Conn) =>
		c.proto === 'udp' && c.dst_port === 443 ? 'QUIC (HTTPS)' : (ports[c.dst_port] ?? (c.dst_port ? String(c.dst_port) : ''));

	// --- Live connections (conntrack) ---
	let conns = $state<ConnList | null>(null);
	let connErr = $state('');
	let auto = $state(false);
	let timer: ReturnType<typeof setInterval> | undefined;

	async function loadConns() {
		try {
			conns = await get<ConnList>(`/clients/${client.id}/connections`);
			connErr = '';
		} catch (e) {
			connErr = errMsg(e);
		}
	}
	function setAuto(on: boolean) {
		auto = on;
		clearInterval(timer);
		if (on) timer = setInterval(loadConns, 5000);
	}
	onDestroy(() => clearInterval(timer));

	// --- DNS history (AdGuard query log for the device IP) ---
	let entries = $state<Entry[] | null>(null);
	let oldest = $state('');
	let dnsErr = $state('');
	let search = $state('');
	let busy = $state(false);
	const ip = $derived(client.address.split('/')[0]);

	async function loadDns(append = false) {
		if (!session.me?.adguard) return;
		busy = true;
		const params = new URLSearchParams({ limit: '200', search: ip });
		if (append && oldest) params.set('older_than', oldest);
		try {
			const r = await get<{ data: Entry[] | null; oldest: string }>(`/adguard/ctl/querylog?${params}`);
			const data = r.data ?? [];
			entries = append && entries ? [...entries, ...data] : data;
			oldest = data.length ? r.oldest : '';
			dnsErr = '';
		} catch (e) {
			dnsErr = errMsg(e);
		} finally {
			busy = false;
		}
	}

	$effect(() => {
		client.id;
		conns = null;
		entries = null;
		oldest = '';
		loadConns();
		loadDns();
	});

	const blocked = (e: Entry) => e.reason.startsWith('Filtered') && e.reason !== 'FilteredWhiteList';

	// IP -> domain map from DNS answers, so connections can be labelled with site names.
	const names = $derived.by(() => {
		const m = new Map<string, string>();
		for (const e of [...(entries ?? [])].reverse())
			for (const a of e.answer ?? []) if (a.type === 'A' || a.type === 'AAAA') m.set(a.value, e.question.name);
		return m;
	});

	const shown = $derived(
		(entries ?? []).filter((e) => !search || e.question.name.toLowerCase().includes(search.toLowerCase()))
	);

	const top = $derived.by(() => {
		const m = new Map<string, { n: number; blocked: number; last: string }>();
		for (const e of entries ?? []) {
			const d = e.question.name.replace(/\.$/, '');
			const v = m.get(d) ?? { n: 0, blocked: 0, last: e.time };
			v.n++;
			if (blocked(e)) v.blocked++;
			m.set(d, v);
		}
		return [...m].sort((a, b) => b[1].n - a[1].n).slice(0, 12);
	});

	const totals = $derived(
		(conns?.conns ?? []).reduce((t, c) => ({ sent: t.sent + c.sent, received: t.received + c.received }), {
			sent: 0,
			received: 0
		})
	);
</script>

<div class="space-y-4">
	<div class="card">
		<div class="mb-3 flex flex-wrap items-center gap-3">
			<div>
				<h2 class="font-semibold">Current connections</h2>
				<p class="hint">Destinations {client.name} ({ip}) is connecting to or just contacted.</p>
			</div>
			<label class="ml-auto flex items-center gap-2 text-sm">
				<input type="checkbox" checked={auto} onchange={(e) => setAuto(e.currentTarget.checked)} />
				Refresh every 5 seconds
			</label>
			<button class="btn-ghost" onclick={loadConns}>Reload</button>
		</div>

		{#if connErr}
			<p class="hint text-red-600">{connErr}</p>
		{:else if !conns}
			<p class="hint">Loading…</p>
		{:else if conns.unsupported}
			<p class="hint">{conns.unsupported}</p>
		{:else}
			{#if conns.conns.length}
				<p class="mb-2 text-xs text-zinc-500">
					{conns.conns.length} connections · ↑ {bytes(totals.sent)} sent · ↓ {bytes(totals.received)} received
				</p>
			{/if}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead class="text-zinc-500">
						<tr>
							<th class="py-1 pr-3 font-medium">Destination</th>
							<th class="py-1 pr-3 font-medium">Service</th>
							<th class="py-1 pr-3 font-medium">Status</th>
							<th class="py-1 pr-3 font-medium">Started</th>
							<th class="py-1 font-medium">Traffic</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-zinc-200 dark:divide-zinc-800">
						{#each conns.conns as c (`${c.proto}${c.src_port}${c.dst_ip}${c.dst_port}`)}
							<tr>
								<td class="py-1.5 pr-3">
									{#if names.get(c.dst_ip)}<div class="font-medium">{names.get(c.dst_ip)?.replace(/\.$/, '')}</div>{/if}
									<div class="font-mono text-zinc-500">{c.dst_ip}{c.dst_port ? `:${c.dst_port}` : ''}</div>
								</td>
								<td class="whitespace-nowrap py-1.5 pr-3">{service(c)} <span class="text-zinc-500">{c.proto.toUpperCase()}</span></td>
								<td class="whitespace-nowrap py-1.5 pr-3">{c.state ?? '–'}</td>
								<td class="whitespace-nowrap py-1.5 pr-3 tabular-nums">{c.started ? ago(c.started) : '–'}</td>
								<td class="whitespace-nowrap py-1.5 tabular-nums">↓ {bytes(c.received)} · ↑ {bytes(c.sent)}</td>
							</tr>
						{:else}
							<tr><td colspan="5" class="hint py-3">No active connections from this device.</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !conns.accounting}
				<p class="hint mt-2">
					The kernel is not counting bytes per connection yet. Re-run the installer/update.sh, or
					<code>sysctl -w net.netfilter.nf_conntrack_acct=1</code>.
				</p>
			{/if}
		{/if}
	</div>

	<div class="card">
		<div class="mb-3 flex flex-wrap items-center gap-3">
			<div>
				<h2 class="font-semibold">Sites visited (DNS history)</h2>
				<p class="hint">Domains this device requested through AdGuard.</p>
			</div>
			{#if session.me?.adguard}
				<input class="input ml-auto" placeholder="Search domains…" bind:value={search} />
				<button class="btn-ghost" disabled={busy} onclick={() => loadDns()}>Reload</button>
			{/if}
		</div>

		{#if !session.me?.adguard}
			<p class="hint">
				Site history needs AdGuard Home. Install or connect it in the <a class="underline" href="/adguard">AdGuard</a> menu
				and enable ad blocking (DNS through AdGuard) for this device.
			</p>
		{:else if dnsErr}
			<p class="hint text-red-600">{dnsErr}</p>
		{:else if !entries}
			<p class="hint">Loading…</p>
		{:else}
			{#if top.length}
				<div class="mb-4 flex flex-wrap gap-2">
					{#each top as [d, v] (d)}
						<button
							class="rounded-sm border border-zinc-200 px-2.5 py-0.5 text-xs dark:border-zinc-700"
							class:text-red-600={v.blocked === v.n}
							title="Last seen {date(v.last)}"
							onclick={() => (search = d)}>{d} <span class="text-zinc-500">{v.n}×</span></button
						>
					{/each}
				</div>
			{/if}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead class="text-zinc-500">
						<tr>
							<th class="py-1 pr-3 font-medium">Time</th>
							<th class="py-1 pr-3 font-medium">Domain</th>
							<th class="py-1 pr-3 font-medium">Type</th>
							<th class="py-1 font-medium">Result</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-zinc-200 dark:divide-zinc-800">
						{#each shown as e, i (i)}
							<tr>
								<td class="whitespace-nowrap py-1.5 pr-3 tabular-nums" title={ago(e.time)}>{date(e.time)}</td>
								<td class="break-all py-1.5 pr-3 font-mono">{e.question.name.replace(/\.$/, '')}</td>
								<td class="py-1.5 pr-3 text-zinc-500">{e.question.type}</td>
								<td class="whitespace-nowrap py-1.5">
									{#if blocked(e)}<span class="text-red-600">Blocked</span>{:else}<span class="text-zinc-500">Allowed</span>{/if}
								</td>
							</tr>
						{:else}
							<tr>
								<td colspan="4" class="hint py-3">
									No DNS queries from {ip} yet. Make sure the device uses AdGuard DNS (ad blocking enabled)
									and the AdGuard query log is turned on.
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if oldest}
				<button class="btn-ghost mt-3" disabled={busy} onclick={() => loadDns(true)}>Load older</button>
			{/if}
		{/if}
	</div>

	<p class="hint">
		Only the "envelope" is visible: domain, destination IP address, port and amount of data. Page contents, messages
		and passwords cannot be seen because almost all sites use HTTPS (encrypted). Current connections are only those
		the kernel still remembers (a few minutes); long-term history comes from the AdGuard DNS log.
	</p>
</div>
