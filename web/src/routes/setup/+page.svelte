<script lang="ts">
	import Logo from '#lib/Logo.svelte';
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

	const steps = ['Admin', 'Cek sistem', 'Server', 'Firewall', 'Perangkat pertama'];
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

	const titles = ['Buat akun admin', 'Cek sistem', 'Pengaturan server', 'Firewall dan NAT', 'Perangkat pertama'];
	const checkTag = { ok: '', warn: 'Peringatan', fail: 'Gagal' } as const;
	// Pelat: satu port per langkah. Hijau sudah lewat, amber sedang dikerjakan, merah bila langkah ini gagal.
	const finished = $derived(done || !!client);
	const waiting = $derived(busy || (step === 1 && !check && !error));
	const plateLabel = $derived(finished ? 'Selesai' : step < 0 ? (error ? 'Gagal memuat' : 'Memuat') : `Langkah ${step + 1} dari ${steps.length}`);
</script>

<svelte:head><title>Penyiapan · Umbra</title></svelte:head>

<main class="login-screen setup-screen">
	<div class="login-unit setup-unit">
		<div class="ports login-plate setup-plate" data-busy={waiting}>
			<div class="ports-caption"><Logo /><span>{plateLabel}</span></div>
			<ol class="setup-steps" aria-label="Langkah penyiapan">
				{#each steps as s, i (s)}
					<li
						class="port"
						class:online={finished || i < step}
						class:busy={!finished && i === step && !error}
						class:disabled={!finished && i === step && !!error}
						aria-current={!finished && i === step ? 'step' : undefined}
					>
						<span aria-hidden="true">{i + 1}</span><span class="sr-only">{s}</span><i></i>
					</li>
				{/each}
			</ol>
		</div>

		<div class="setup-body">
			{#if done}
				<h1>WireGuard sudah disiapkan</h1>
				<p class="hint">Pengaturan server bisa diubah di halaman Pengaturan.</p>
				<div><a class="btn-primary" href="/">Ke Ringkasan</a></div>
			{:else if step === -1}
				<h1>Penyiapan</h1>
				{#if error}<p class="login-error" role="alert">{error}</p>{:else}<p class="hint">Memuat…</p>{/if}
			{:else if step === 0}
				<form class="setup-form" onsubmit={createAdmin} oninput={() => (error = '')}>
					<h1>{titles[0]}</h1>
					<div>
						<label class="label" for="t">Token setup</label>
						<input id="t" class="input font-mono" bind:value={token} autocomplete="off" autocapitalize="none" spellcheck="false" aria-describedby="t-hint" required />
						<p class="hint" id="t-hint">
							Ada di file <span class="font-mono">setup-token</span> pada folder data panel, dan tercetak di log
							(<span class="font-mono">journalctl -u umbra</span>).
						</p>
					</div>
					<div>
						<label class="label" for="u">Nama pengguna</label>
						<input id="u" class="input" bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" required />
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<div>
							<label class="label" for="p">Kata sandi</label>
							<input id="p" class="input" type="password" minlength="10" bind:value={password} autocomplete="new-password" aria-describedby="p-hint" required />
							<p class="hint" id="p-hint">Minimal 10 karakter.</p>
						</div>
						<div>
							<label class="label" for="p2">Ulangi kata sandi</label>
							<input id="p2" class="input" type="password" bind:value={password2} autocomplete="new-password" required />
						</div>
					</div>
					{#if error}<p class="login-error" role="alert">{error}</p>{/if}
					<button class="btn-primary" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat admin'}</button>
				</form>
			{:else if step === 1}
				<h1>{titles[1]}</h1>
				{#if !check}
					{#if error}
						<p class="login-error" role="alert">{error}</p>
						<div><button class="btn-ghost" onclick={gotoCheck}>Cek ulang</button></div>
					{:else}<p class="hint">Memeriksa…</p>{/if}
				{:else}
					<ul class="setup-checks">
						{#each check.items as it (it.id)}
							<li>
								<i class="check-led {it.status}" aria-hidden="true"></i>
								<div>
									<p>{it.label}{#if checkTag[it.status]}<span class="check-tag {it.status}">{checkTag[it.status]}</span>{/if}</p>
									{#if it.detail}<p class="hint">{it.detail}</p>{/if}
								</div>
							</li>
						{/each}
					</ul>
					<div class="setup-note">
						<p>IP publik <span class="font-mono">{check.public_ip || 'tidak terdeteksi'}</span></p>
						{#if check.behind_nat}
							<p class="hint">
								Server berada di balik NAT (IP lokal {check.local_ips.join(', ')}). Pastikan port UDP
								WireGuard diteruskan (port forwarding) dari router/penyedia ke server ini.
							</p>
						{/if}
					</div>
					{#if hasFail}<p class="login-error">Perbaiki butir bertanda Gagal dulu, lalu cek ulang.</p>{/if}
					<div class="setup-actions">
						<button class="btn-primary" disabled={hasFail} onclick={() => (step = 2)}>Lanjut</button>
						<button class="btn-ghost" onclick={gotoCheck}>Cek ulang</button>
					</div>
				{/if}
			{:else if step === 2 && server}
				<h1>{titles[2]}</h1>
				<ServerForm {server} {check} submitLabel="Aktifkan WireGuard" onsaved={serverSaved} />
			{:else if step === 3}
				<h1>{titles[3]}</h1>
				<FirewallPanel onconfirmed={() => (step = 4)} />
				<div><button class="btn-ghost" onclick={() => (step = 4)}>Lewati, atur firewall sendiri</button></div>
			{:else if step === 4}
				{#if !client}
					<form class="setup-form" onsubmit={createClient} oninput={() => (error = '')}>
						<h1>{titles[4]}</h1>
						<div>
							<label class="label" for="cn">Nama perangkat</label>
							<input id="cn" class="input" bind:value={clientName} placeholder="HP Andi" required />
						</div>
						{#if error}<p class="login-error" role="alert">{error}</p>{/if}
						<div class="setup-actions">
							<button class="btn-primary" disabled={busy}>{busy ? 'Menambahkan…' : 'Tambah perangkat'}</button>
							<a class="btn-ghost" href="/">Lewati</a>
						</div>
					</form>
				{:else}
					<h1>Profil {client.name}</h1>
					<ClientProfile {client} showActivity={false} />
					<p class="hint">Setelah tersambung, perangkat ini tampil tersambung di Ringkasan.</p>
					<div><a class="btn-primary" href="/">Selesai</a></div>
				{/if}
			{/if}
		</div>
	</div>
</main>
