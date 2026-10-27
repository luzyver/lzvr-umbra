<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		get,
		post,
		errMsg,
		type Client,
		type Me,
		type ServerConfig,
		type SetupStatus,
		type SystemCheck
	} from '#lib/api.ts';
	import { session } from '#lib/session.svelte.ts';
	import { warnAll } from '#lib/toast.svelte.ts';
	import ServerForm from '#lib/ServerForm.svelte';
	import FirewallPanel from '#lib/FirewallPanel.svelte';
	import ClientProfile from '#lib/ClientProfile.svelte';

	const steps = ['Admin', 'Cek sistem', 'Server', 'Firewall', 'Klien pertama'];
	let step = $state(-1); // -1 = memuat
	let done = $state(false); // sudah disetup sebelumnya

	let error = $state('');
	let busy = $state(false);

	// Langkah 0: admin
	let token = $state('');
	let username = $state('admin');
	let password = $state('');
	let password2 = $state('');

	// Langkah 1-2
	let check = $state<SystemCheck | null>(null);
	let server = $state<ServerConfig | null>(null);

	// Langkah 4
	let clientName = $state('');
	let client = $state<Client | null>(null);

	async function init() {
		try {
			const st = await get<SetupStatus>('/setup/status');
			if (st.needs_admin) {
				step = 0;
				return;
			}
			if (!session.me) {
				await goto('/login', { replaceState: true });
				return;
			}
			if (st.initialized) {
				done = true;
				return;
			}
			await gotoCheck();
		} catch (e) {
			error = errMsg(e);
		}
	}
	init();

	async function createAdmin(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (password !== password2) {
			error = 'Kedua kata sandi tidak sama.';
			return;
		}
		busy = true;
		try {
			await post('/setup/admin', { token: token.trim(), username: username.trim(), password });
			// Pembuatan admin tidak membuat sesi; masuk otomatis.
			await post('/auth/login', { username: username.trim(), password });
			session.me = await get<Me>('/auth/me');
			password = password2 = token = '';
			await gotoCheck();
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}

	async function gotoCheck() {
		step = 1;
		check = null;
		error = '';
		try {
			const [c, s] = await Promise.all([get<SystemCheck>('/system/check'), get<ServerConfig>('/server')]);
			check = c;
			server = s;
		} catch (e) {
			error = errMsg(e);
		}
	}

	const hasFail = $derived(check?.items.some((i) => i.status === 'fail') ?? false);

	function serverSaved(s: ServerConfig) {
		server = s;
		session.initialized = true;
		step = 3;
	}

	async function createClient(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const r = await post<{ client: Client; warnings: string[] }>('/clients', { name: clientName.trim() });
			warnAll(r.warnings);
			client = r.client;
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}

	const icon = { ok: '✓', warn: '!', fail: '✕' } as const;
	const iconCls = {
		ok: 'bg-teal-100 text-teal-700 dark:bg-teal-950 dark:text-teal-300',
		warn: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
		fail: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'
	} as const;
</script>

<svelte:head><title>Setup · Umbra</title></svelte:head>

<div class="mx-auto max-w-2xl px-4 py-10">
	<div class="mb-6 flex items-center gap-2">
		<img src="/favicon.svg" alt="" class="h-8 w-8" />
		<h1 class="text-xl font-semibold">Setup Umbra</h1>
	</div>

	{#if done}
		<div class="card space-y-3">
			<p class="font-medium">WireGuard sudah disetup.</p>
			<p class="hint">Pengaturan server bisa diubah di halaman Pengaturan.</p>
			<a class="btn-primary inline-block" href="/">Ke dasbor</a>
		</div>
	{:else if step === -1}
		{#if error}<p class="text-sm text-red-600">{error}</p>{:else}<p class="text-sm text-zinc-500">Memuat…</p>{/if}
	{:else}
		<ol class="mb-6 flex flex-wrap gap-2 text-xs">
			{#each steps as s, i (s)}
				<li
					class="rounded-full px-3 py-1 {i === step
						? 'bg-teal-600 text-white'
						: i < step
							? 'bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-200'
							: 'bg-zinc-200 text-zinc-500 dark:bg-zinc-800'}"
				>
					{i + 1}. {s}
				</li>
			{/each}
		</ol>

		<div class="card">
			{#if step === 0}
				<form class="space-y-4" onsubmit={createAdmin}>
					<div>
						<h2 class="font-semibold">Buat akun admin</h2>
						<p class="hint mt-1">
							Token setup ada di file <span class="font-mono">setup-token</span> pada folder data panel, dan
							juga tercetak di log (<span class="font-mono">journalctl -u umbra</span>).
						</p>
					</div>
					<div>
						<label class="label" for="t">Token setup</label>
						<input id="t" class="input font-mono" bind:value={token} autocomplete="off" required />
					</div>
					<div>
						<label class="label" for="u">Nama pengguna</label>
						<input id="u" class="input" bind:value={username} autocomplete="username" required />
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<div>
							<label class="label" for="p">Kata sandi</label>
							<input id="p" class="input" type="password" minlength="10" bind:value={password} autocomplete="new-password" required />
							<p class="hint">Minimal 10 karakter.</p>
						</div>
						<div>
							<label class="label" for="p2">Ulangi kata sandi</label>
							<input id="p2" class="input" type="password" bind:value={password2} autocomplete="new-password" required />
						</div>
					</div>
					{#if error}<p class="text-sm text-red-600">{error}</p>{/if}
					<button class="btn-primary" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat admin'}</button>
				</form>
			{:else if step === 1}
				<h2 class="font-semibold">Cek sistem</h2>
				<p class="hint mt-1">Memastikan server siap menjalankan WireGuard.</p>
				{#if !check}
					{#if error}<p class="mt-4 text-sm text-red-600">{error}</p>{:else}<p class="mt-4 text-sm text-zinc-500">Memeriksa…</p>{/if}
				{:else}
					<ul class="mt-4 space-y-2">
						{#each check.items as it (it.id)}
							<li class="flex gap-3">
								<span class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-xs font-bold {iconCls[it.status]}">{icon[it.status]}</span>
								<div>
									<p class="text-sm font-medium">{it.label}</p>
									{#if it.detail}<p class="hint">{it.detail}</p>{/if}
								</div>
							</li>
						{/each}
					</ul>
					<div class="mt-4 rounded-lg bg-zinc-100 p-3 text-sm dark:bg-zinc-950">
						<p>IP publik: <span class="font-mono">{check.public_ip || 'tidak terdeteksi'}</span></p>
						{#if check.behind_nat}
							<p class="hint mt-1">
								Server berada di balik NAT (IP lokal {check.local_ips.join(', ')}). Pastikan port UDP
								WireGuard diteruskan (port forwarding) dari router/penyedia ke server ini.
							</p>
						{/if}
					</div>
					<div class="mt-4 flex gap-2">
						<button class="btn-primary" disabled={hasFail} onclick={() => (step = 2)}>Lanjut</button>
						<button class="btn-ghost" onclick={gotoCheck}>Cek ulang</button>
					</div>
					{#if hasFail}<p class="hint mt-2 text-red-600">Perbaiki item bertanda ✕ dulu, lalu cek ulang.</p>{/if}
				{/if}
			{:else if step === 2 && server}
				<h2 class="font-semibold">Pengaturan server</h2>
				<p class="hint mb-4 mt-1">Nilai sudah diisi otomatis. Biasanya cukup langsung diterapkan.</p>
				<ServerForm {server} {check} submitLabel="Aktifkan WireGuard" onsaved={serverSaved} />
			{:else if step === 3}
				<h2 class="mb-1 font-semibold">Firewall & NAT</h2>
				<p class="hint mb-4">Supaya klien bisa mengakses internet lewat server ini.</p>
				<FirewallPanel onconfirmed={() => (step = 4)} />
				<button class="btn-ghost mt-4 text-xs" onclick={() => (step = 4)}>Lewati (atur firewall sendiri)</button>
			{:else if step === 4}
				{#if !client}
					<form class="space-y-4" onsubmit={createClient}>
						<h2 class="font-semibold">Klien pertama</h2>
						<div>
							<label class="label" for="cn">Nama perangkat</label>
							<input id="cn" class="input" bind:value={clientName} placeholder="HP Andi" required />
						</div>
						{#if error}<p class="text-sm text-red-600">{error}</p>{/if}
						<div class="flex gap-2">
							<button class="btn-primary" disabled={busy}>{busy ? 'Membuat…' : 'Buat klien'}</button>
							<a class="btn-ghost" href="/">Lewati</a>
						</div>
					</form>
				{:else}
					<h2 class="mb-4 font-semibold">Profil {client.name}</h2>
					<ClientProfile {client} />
					<p class="hint mt-4 text-center">
						Setelah tersambung, klien akan tampil “online” di dasbor.
					</p>
					<div class="mt-4 text-center"><a class="btn-primary" href="/">Selesai</a></div>
				{/if}
			{/if}
		</div>
	{/if}
</div>
