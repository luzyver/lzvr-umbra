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

	const steps = ['Admin', 'System check', 'Server', 'Firewall', 'First device'];
	let step = $state(-1); // -1 = loading
	let done = $state(false); // already set up before

	let error = $state('');
	let busy = $state(false);

	// Step 0: admin
	let token = $state('');
	let username = $state('admin');
	let password = $state('');
	let password2 = $state('');

	// Steps 1-2
	let check = $state<SystemCheck | null>(null);
	let server = $state<ServerConfig | null>(null);

	// Step 4
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
			error = 'The two passwords do not match.';
			return;
		}
		busy = true;
		try {
			await post('/setup/admin', { token: token.trim(), username: username.trim(), password });
			// Creating the admin does not create a session; sign in automatically.
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

	const titles = ['Create admin account', 'System check', 'Server settings', 'Firewall and NAT', 'First device'];
	const checkTag = { ok: '', warn: 'Warning', fail: 'Failed' } as const;
	// Plate: one port per step. Green = done, amber = in progress, red = this step failed.
	const finished = $derived(done || !!client);
	const waiting = $derived(busy || (step === 1 && !check && !error));
	const plateLabel = $derived(finished ? 'Done' : step < 0 ? (error ? 'Failed to load' : 'Loading') : `Step ${step + 1} of ${steps.length}`);
</script>

<svelte:head><title>Setup · Umbra</title></svelte:head>

<main class="login-screen setup-screen">
	<div class="login-unit setup-unit">
		<div class="ports login-plate setup-plate" data-busy={waiting}>
			<div class="ports-caption"><Logo /><span>{plateLabel}</span></div>
			<ol class="setup-steps" aria-label="Setup steps">
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
				<h1>WireGuard is already set up</h1>
				<p class="hint">Server settings can be changed on the Settings page.</p>
				<div><a class="btn-primary" href="/">Go to Overview</a></div>
			{:else if step === -1}
				<h1>Setup</h1>
				{#if error}<p class="login-error" role="alert">{error}</p>{:else}<p class="hint">Loading…</p>{/if}
			{:else if step === 0}
				<form class="setup-form" onsubmit={createAdmin} oninput={() => (error = '')}>
					<h1>{titles[0]}</h1>
					<div>
						<label class="label" for="t">Setup token</label>
						<input id="t" class="input font-mono" bind:value={token} autocomplete="off" autocapitalize="none" spellcheck="false" aria-describedby="t-hint" required />
						<p class="hint" id="t-hint">
							Found in the <span class="font-mono">setup-token</span> file in the panel data folder, and printed in the log
							(<span class="font-mono">journalctl -u umbra</span>).
						</p>
					</div>
					<div>
						<label class="label" for="u">Username</label>
						<input id="u" class="input" bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" required />
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<div>
							<label class="label" for="p">Password</label>
							<input id="p" class="input" type="password" minlength="10" bind:value={password} autocomplete="new-password" aria-describedby="p-hint" required />
							<p class="hint" id="p-hint">At least 10 characters.</p>
						</div>
						<div>
							<label class="label" for="p2">Repeat password</label>
							<input id="p2" class="input" type="password" bind:value={password2} autocomplete="new-password" required />
						</div>
					</div>
					{#if error}<p class="login-error" role="alert">{error}</p>{/if}
					<button class="btn-primary" disabled={busy}>{busy ? 'Saving…' : 'Create admin'}</button>
				</form>
			{:else if step === 1}
				<h1>{titles[1]}</h1>
				{#if !check}
					{#if error}
						<p class="login-error" role="alert">{error}</p>
						<div><button class="btn-ghost" onclick={gotoCheck}>Check again</button></div>
					{:else}<p class="hint">Checking…</p>{/if}
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
						<p>Public IP <span class="font-mono">{check.public_ip || 'not detected'}</span></p>
						{#if check.behind_nat}
							<p class="hint">
								The server is behind NAT (local IP {check.local_ips.join(', ')}). Make sure the WireGuard UDP
								port is forwarded (port forwarding) from the router or provider to this server.
							</p>
						{/if}
					</div>
					{#if hasFail}<p class="login-error">Fix the items marked Failed first, then check again.</p>{/if}
					<div class="setup-actions">
						<button class="btn-primary" disabled={hasFail} onclick={() => (step = 2)}>Continue</button>
						<button class="btn-ghost" onclick={gotoCheck}>Check again</button>
					</div>
				{/if}
			{:else if step === 2 && server}
				<h1>{titles[2]}</h1>
				<ServerForm {server} {check} submitLabel="Enable WireGuard" onsaved={serverSaved} />
			{:else if step === 3}
				<h1>{titles[3]}</h1>
				<FirewallPanel onconfirmed={() => (step = 4)} />
				<div><button class="btn-ghost" onclick={() => (step = 4)}>Skip, I'll set up the firewall myself</button></div>
			{:else if step === 4}
				{#if !client}
					<form class="setup-form" onsubmit={createClient} oninput={() => (error = '')}>
						<h1>{titles[4]}</h1>
						<div>
							<label class="label" for="cn">Device name</label>
							<input id="cn" class="input" bind:value={clientName} placeholder="Andi's phone" required />
						</div>
						{#if error}<p class="login-error" role="alert">{error}</p>{/if}
						<div class="setup-actions">
							<button class="btn-primary" disabled={busy}>{busy ? 'Adding…' : 'Add device'}</button>
							<a class="btn-ghost" href="/">Skip</a>
						</div>
					</form>
				{:else}
					<h1>{client.name} profile</h1>
					<ClientProfile {client} showActivity={false} />
					<p class="hint">Once connected, this device shows as connected in Overview.</p>
					<div><a class="btn-primary" href="/">Done</a></div>
				{/if}
			{/if}
		</div>
	</div>
</main>
