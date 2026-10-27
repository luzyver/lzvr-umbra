<script lang="ts">
	import { onMount } from 'svelte';
	onMount(() => { if (new URL(location.href).searchParams.get('new') === '1') openNew(); });
	import { get, post, put, del, errMsg, type Client, type Defaults } from '#lib/api.ts';
	import { splitList, date } from '#lib/format.ts';
	import { session } from '#lib/session.svelte.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';
	import Modal from '#lib/Modal.svelte';
	import Toggle from '#lib/Toggle.svelte';
	import ClientProfile from '#lib/ClientProfile.svelte';

	type Result = { client: Client; profile_changed: boolean; warnings: string[] };

	let clients = $state<Client[] | null>(null);
	let defaults = $state<Defaults | null>(null);
	let error = $state('');
	let q = $state('');

	async function load() {
		try {
			const [c, d] = await Promise.all([get<Client[]>('/clients'), get<Defaults>('/defaults')]);
			clients = c;
			defaults = d;
		} catch (e) {
			error = errMsg(e);
		}
	}
	load();

	const shown = $derived(
		(clients ?? []).filter(
			(c) => !q || c.name.toLowerCase().includes(q.toLowerCase()) || c.address.includes(q)
		)
	);

	function replace(c: Client) {
		if (!clients) return;
		const i = clients.findIndex((x) => x.id === c.id);
		if (i >= 0) clients[i] = c;
		else clients.push(c);
	}

	function afterSave(r: Result, msg: string) {
		warnAll(r.warnings);
		replace(r.client);
		if (r.profile_changed) toast(`${msg} Profil di perangkat perlu diunduh ulang.`, 'warn', 7000);
		else toast(msg);
	}

	// ---- Form tambah / ubah ----
	let formOpen = $state(false);
	let editing = $state<Client | null>(null);
	let busy = $state(false);
	let formError = $state('');
	let f = $state({ name: '', allowed_ips: '', dns: '', mtu: '', keepalive: '', adblock: true });

	function openNew() {
		editing = null;
		f = { name: '', allowed_ips: '', dns: '', mtu: '', keepalive: '', adblock: true };
		formError = '';
		formOpen = true;
	}

	function openEdit(c: Client) {
		editing = c;
		f = {
			name: c.name,
			allowed_ips: c.allowed_ips.join(', '),
			dns: c.dns.join(', '),
			mtu: c.mtu ? String(c.mtu) : '',
			keepalive: c.keepalive ? String(c.keepalive) : '',
			adblock: c.adblock
		};
		formError = '';
		formOpen = true;
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		formError = '';
		// Kolom kosong = pakai bawaan server. Server menyimpan apa adanya, jadi kolom kosong
		// diisi nilai bawaan (atau tidak dikirim bila bawaan belum termuat) agar tidak tersimpan kosong.
		const body: Record<string, unknown> = { name: f.name.trim() };
		const list = (key: 'allowed_ips' | 'dns') => {
			const v = splitList(f[key]);
			if (v.length) body[key] = v;
			else if (defaults) body[key] = defaults[key];
		};
		const num = (key: 'mtu' | 'keepalive') => {
			const v = Number(f[key]);
			if (v) body[key] = v;
			else if (defaults) body[key] = defaults[key];
		};
		list('allowed_ips');
		list('dns');
		num('mtu');
		num('keepalive');
		if (session.me?.adguard) body.adblock = f.adblock;
		try {
			if (editing) {
				afterSave(await put<Result>(`/clients/${editing.id}`, body), 'Klien disimpan.');
				formOpen = false;
			} else {
				const r = await post<Result>('/clients', body);
				warnAll(r.warnings);
				replace(r.client);
				formOpen = false;
				profile = r.client; // langsung tampilkan QR
			}
		} catch (e) {
			formError = errMsg(e);
		} finally {
			busy = false;
		}
	}

	// ---- Toggle cepat ----
	async function toggle(c: Client, field: 'enabled' | 'adblock', v: boolean) {
		try {
			const r = await put<Result>(`/clients/${c.id}`, { [field]: v });
			afterSave(
				r,
				field === 'enabled' ? (v ? `${c.name} diaktifkan.` : `${c.name} dinonaktifkan.`) : `Blokir iklan ${v ? 'aktif' : 'mati'} untuk ${c.name}.`
			);
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}

	// ---- Hapus ----
	let deleting = $state<Client | null>(null);
	let deleteOpen = $state(false);
	async function confirmDelete() {
		if (!deleting) return;
		const c = deleting;
		busy = true;
		try {
			const r = await del<{ warnings: string[] }>(`/clients/${c.id}`);
			warnAll(r?.warnings);
			clients = (clients ?? []).filter((x) => x.id !== c.id);
			toast(`${c.name} dihapus.`);
			deleteOpen = false;
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	// ---- QR ----
	let profile = $state<Client | null>(null);
	let profileOpen = $state(false);
	$effect(() => {
		if (profile) profileOpen = true;
	});
	$effect(() => {
		if (!profileOpen) profile = null;
	});
</script>

<svelte:head><title>Klien · Umbra</title></svelte:head>

<div class="page-heading flex-wrap">
	<div><div class="overline">YOUR DEVICES</div><h1>Perangkat</h1><p>Atur akses dan profil koneksi perangkatmu.</p></div>
	<input class="input ml-auto w-56" aria-label="Cari perangkat" placeholder="Cari nama / IP" bind:value={q} />
	<button class="btn-primary" onclick={openNew}>+ Tambah klien</button>
</div>

{#if error}
	<div class="card text-sm text-red-600">{error}</div>
{:else if !clients}
	<p class="text-sm text-zinc-500">Memuat…</p>
{:else if clients.length === 0}
	<div class="card text-center">
		<p class="text-sm text-zinc-500">Belum ada klien.</p>
		<button class="btn-primary mt-3" onclick={openNew}>Tambah klien pertama</button>
	</div>
{:else}
	<div class="card client-management overflow-x-auto p-0">
		<table class="w-full text-sm">
			<thead class="text-left text-xs text-zinc-500">
				<tr class="border-b border-zinc-200 dark:border-zinc-800">
					<th class="px-4 py-2 font-medium">Nama</th>
					<th class="px-4 py-2 font-medium">Alamat</th>
					<th class="px-4 py-2 font-medium">Aktif</th>
					{#if session.me?.adguard}<th class="px-4 py-2 font-medium">Blokir iklan</th>{/if}
					<th class="px-4 py-2 font-medium">Dibuat</th>
					<th class="px-4 py-2"></th>
				</tr>
			</thead>
			<tbody>
				{#each shown as c (c.id)}
					<tr class="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60">
						<td class="px-4 py-2 font-medium">{c.name}</td>
						<td class="px-4 py-2 font-mono text-xs">{c.address}</td>
						<td class="px-4 py-2">
							<Toggle checked={c.enabled} label="Aktif" onchange={(v) => toggle(c, 'enabled', v)} />
						</td>
						{#if session.me?.adguard}
							<td class="px-4 py-2">
								<Toggle checked={c.adblock} label="Blokir iklan" onchange={(v) => toggle(c, 'adblock', v)} />
							</td>
						{/if}
						<td class="px-4 py-2 text-xs text-zinc-500">{date(c.created_at)}</td>
						<td class="px-4 py-2">
							<div class="flex justify-end gap-1">
								<button class="btn-ghost px-2 py-1 text-xs" onclick={() => (profile = c)}>QR / unduh</button>
								<button class="btn-ghost px-2 py-1 text-xs" onclick={() => openEdit(c)}>Ubah</button>
								<button
									class="btn-ghost px-2 py-1 text-xs text-red-600"
									onclick={() => {
										deleting = c;
										deleteOpen = true;
									}}>Hapus</button
								>
							</div>
						</td>
					</tr>
				{:else}
					<tr><td colspan="6" class="px-4 py-6 text-center text-zinc-500">Tidak ada yang cocok.</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<Modal bind:open={formOpen} title={editing ? `Ubah ${editing.name}` : 'Tambah klien'}>
	<form id="client-form" class="space-y-4" onsubmit={save}>
		<div>
			<label class="label" for="c-name">Nama perangkat</label>
			<input id="c-name" class="input" bind:value={f.name} placeholder="Laptop kantor" required />
		</div>
		{#if session.me?.adguard}
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={f.adblock} /> Blokir iklan & pelacak (AdGuard)
			</label>
		{/if}
		<details>
			<summary class="cursor-pointer text-sm text-zinc-500">Pengaturan lanjutan</summary>
			<div class="mt-3 space-y-3">
				<div>
					<label class="label" for="c-aips">Allowed IPs</label>
					<input id="c-aips" class="input font-mono" bind:value={f.allowed_ips} placeholder={defaults?.allowed_ips.join(', ')} />
					<p class="hint">Kosong = bawaan. 0.0.0.0/0 berarti semua internet lewat VPN.</p>
				</div>
				<div>
					<label class="label" for="c-dns">DNS</label>
					<input id="c-dns" class="input font-mono" bind:value={f.dns} placeholder={defaults?.dns.join(', ')} />
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div>
						<label class="label" for="c-mtu">MTU</label>
						<input id="c-mtu" class="input" type="number" min="0" max="1500" bind:value={f.mtu} placeholder={String(defaults?.mtu || 'otomatis')} />
					</div>
					<div>
						<label class="label" for="c-ka">Keepalive (detik)</label>
						<input id="c-ka" class="input" type="number" min="0" max="3600" bind:value={f.keepalive} placeholder={String(defaults?.keepalive ?? '')} />
					</div>
				</div>
			</div>
		</details>
		{#if formError}<p class="text-sm text-red-600">{formError}</p>{/if}
	</form>
	{#snippet footer()}
		<button class="btn-ghost" onclick={() => (formOpen = false)}>Batal</button>
		<button class="btn-primary" form="client-form" disabled={busy}>{busy ? 'Menyimpan…' : 'Simpan'}</button>
	{/snippet}
</Modal>

<Modal bind:open={deleteOpen} title="Hapus klien">
	<p class="text-sm">
		Hapus <b>{deleting?.name}</b>? Perangkat ini langsung terputus dan profilnya tidak bisa dipakai lagi.
	</p>
	{#snippet footer()}
		<button class="btn-ghost" onclick={() => (deleteOpen = false)}>Batal</button>
		<button class="btn-danger" disabled={busy} onclick={confirmDelete}>Hapus</button>
	{/snippet}
</Modal>

<Modal bind:open={profileOpen} title={profile ? `Profil ${profile.name}` : 'Profil'}>
	{#if profile}<ClientProfile client={profile} />{/if}
</Modal>
