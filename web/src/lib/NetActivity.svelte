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

	// --- Koneksi live (conntrack) ---
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

	// --- Riwayat DNS (log query AdGuard untuk IP klien) ---
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

	// Peta IP -> domain dari jawaban DNS, agar koneksi bisa diberi nama situs.
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
				<h2 class="font-semibold">Koneksi saat ini</h2>
				<p class="hint">Tujuan yang sedang/baru saja dihubungi {client.name} ({ip}).</p>
			</div>
			<label class="ml-auto flex items-center gap-2 text-sm">
				<input type="checkbox" checked={auto} onchange={(e) => setAuto(e.currentTarget.checked)} />
				Segarkan tiap 5 detik
			</label>
			<button class="btn-ghost" onclick={loadConns}>Muat ulang</button>
		</div>

		{#if connErr}
			<p class="hint text-red-600">{connErr}</p>
		{:else if !conns}
			<p class="hint">Memuat…</p>
		{:else if conns.unsupported}
			<p class="hint">{conns.unsupported}</p>
		{:else}
			{#if conns.conns.length}
				<p class="mb-2 text-xs text-zinc-500">
					{conns.conns.length} koneksi · ↑ {bytes(totals.sent)} dikirim · ↓ {bytes(totals.received)} diterima
				</p>
			{/if}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead class="text-zinc-500">
						<tr>
							<th class="py-1 pr-3 font-medium">Tujuan</th>
							<th class="py-1 pr-3 font-medium">Layanan</th>
							<th class="py-1 pr-3 font-medium">Status</th>
							<th class="py-1 pr-3 font-medium">Mulai</th>
							<th class="py-1 font-medium">Trafik</th>
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
							<tr><td colspan="5" class="hint py-3">Tidak ada koneksi aktif dari perangkat ini.</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !conns.accounting}
				<p class="hint mt-2">
					Kernel belum menghitung byte per koneksi. Jalankan ulang installer/update.sh, atau
					<code>sysctl -w net.netfilter.nf_conntrack_acct=1</code>.
				</p>
			{/if}
		{/if}
	</div>

	<div class="card">
		<div class="mb-3 flex flex-wrap items-center gap-3">
			<div>
				<h2 class="font-semibold">Situs yang dibuka (riwayat DNS)</h2>
				<p class="hint">Domain yang diminta perangkat ini lewat AdGuard.</p>
			</div>
			{#if session.me?.adguard}
				<input class="input ml-auto" placeholder="Cari domain…" bind:value={search} />
				<button class="btn-ghost" disabled={busy} onclick={() => loadDns()}>Muat ulang</button>
			{/if}
		</div>

		{#if !session.me?.adguard}
			<p class="hint">
				Riwayat situs butuh AdGuard Home. Pasang/hubungkan di menu <a class="underline" href="/adguard">AdGuard</a>
				dan aktifkan blokir iklan (DNS lewat AdGuard) untuk perangkat ini.
			</p>
		{:else if dnsErr}
			<p class="hint text-red-600">{dnsErr}</p>
		{:else if !entries}
			<p class="hint">Memuat…</p>
		{:else}
			{#if top.length}
				<div class="mb-4 flex flex-wrap gap-2">
					{#each top as [d, v] (d)}
						<button
							class="rounded-sm border border-zinc-200 px-2.5 py-0.5 text-xs dark:border-zinc-700"
							class:text-red-600={v.blocked === v.n}
							title="Terakhir {date(v.last)}"
							onclick={() => (search = d)}>{d} <span class="text-zinc-500">{v.n}×</span></button
						>
					{/each}
				</div>
			{/if}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead class="text-zinc-500">
						<tr>
							<th class="py-1 pr-3 font-medium">Waktu</th>
							<th class="py-1 pr-3 font-medium">Domain</th>
							<th class="py-1 pr-3 font-medium">Tipe</th>
							<th class="py-1 font-medium">Hasil</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-zinc-200 dark:divide-zinc-800">
						{#each shown as e, i (i)}
							<tr>
								<td class="whitespace-nowrap py-1.5 pr-3 tabular-nums" title={ago(e.time)}>{date(e.time)}</td>
								<td class="break-all py-1.5 pr-3 font-mono">{e.question.name.replace(/\.$/, '')}</td>
								<td class="py-1.5 pr-3 text-zinc-500">{e.question.type}</td>
								<td class="whitespace-nowrap py-1.5">
									{#if blocked(e)}<span class="text-red-600">Diblokir</span>{:else}<span class="text-zinc-500">Diizinkan</span>{/if}
								</td>
							</tr>
						{:else}
							<tr>
								<td colspan="4" class="hint py-3">
									Belum ada query DNS dari {ip}. Pastikan perangkat memakai DNS AdGuard (opsi blokir iklan aktif)
									dan log query AdGuard menyala.
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if oldest}
				<button class="btn-ghost mt-3" disabled={busy} onclick={() => loadDns(true)}>Muat lebih lama</button>
			{/if}
		{/if}
	</div>

	<p class="hint">
		Yang terlihat hanya "amplop"-nya: domain, alamat IP tujuan, port, dan jumlah data. Isi halaman, pesan, atau
		password tidak bisa dilihat karena hampir semua situs memakai HTTPS (terenkripsi). Koneksi saat ini hanya yang masih
		diingat kernel (beberapa menit); riwayat jangka panjang berasal dari log DNS AdGuard.
	</p>
</div>
