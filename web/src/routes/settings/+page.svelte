<script lang="ts">
	import { get, put, post, errMsg, type Defaults, type ServerConfig, type Status } from '#lib/api.ts';
	import { splitList } from '#lib/format.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';
	import ServerForm from '#lib/ServerForm.svelte';
	import FirewallPanel from '#lib/FirewallPanel.svelte';
	import TwoFactorSettings from '#lib/TwoFactorSettings.svelte';

	let server = $state<ServerConfig | null>(null);
	let error = $state('');
	let wgBackend = $state<Status['wg_backend']>('pending');
	const backendLabels = { kernel: 'Kernel', userspace: 'Userspace (wireguard-go)', fake: 'Simulated', pending: 'Not enabled' };

	// New device defaults
	let d = $state({ dns: '', allowed_ips: '', keepalive: '', mtu: '' });
	let dBusy = $state(false);
	let dError = $state('');

	// Change password
	let pw = $state({ current: '', next: '', repeat: '' });
	let pwBusy = $state(false);
	let pwError = $state('');

	function fillDefaults(x: Defaults) {
		d = {
			dns: x.dns.join(', '),
			allowed_ips: x.allowed_ips.join(', '),
			keepalive: String(x.keepalive ?? ''),
			mtu: x.mtu ? String(x.mtu) : ''
		};
	}

	async function load() {
		try {
			const [s, x, status] = await Promise.all([get<ServerConfig>('/server'), get<Defaults>('/defaults'), get<Status>('/status')]);
			server = s;
			wgBackend = status.wg_backend;
			fillDefaults(x);
		} catch (e) {
			error = errMsg(e);
		}
	}
	load();

	function serverSaved(s: ServerConfig, profileChanged: boolean) {
		server = s;
		get<Status>('/status').then((status) => { wgBackend = status.wg_backend; }).catch(() => {});
		if (profileChanged) toast('All device profiles need to be downloaded again.', 'warn', 8000);
	}

	async function saveDefaults(e: SubmitEvent) {
		e.preventDefault();
		dBusy = true;
		dError = '';
		try {
			const r = await put<Defaults & { warnings?: string[] }>('/defaults', {
				dns: splitList(d.dns),
				allowed_ips: splitList(d.allowed_ips),
				keepalive: Number(d.keepalive) || 0,
				mtu: Number(d.mtu) || 0
			});
			warnAll(r?.warnings);
			if (r?.dns) fillDefaults(r);
			toast('Defaults saved. Devices using the defaults need to download their profile again.');
		} catch (e) {
			dError = errMsg(e);
		} finally {
			dBusy = false;
		}
	}

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		pwError = '';
		if (pw.next !== pw.repeat) {
			pwError = 'The new password and its confirmation do not match.';
			return;
		}
		pwBusy = true;
		try {
			await post('/auth/password', { current: pw.current, new: pw.next });
			pw = { current: '', next: '', repeat: '' };
			toast('Password changed.');
		} catch (e) {
			pwError = errMsg(e);
		} finally {
			pwBusy = false;
		}
	}
</script>

<svelte:head><title>Settings · Umbra</title></svelte:head>

<div class="page-heading"><h1>Settings</h1></div>

{#if error}
	<div class="card text-sm text-red-600">{error}</div>
{:else if !server}
	<p class="text-sm text-zinc-500">Loading…</p>
{:else}
	<div class="space-y-6">
		<section class="card">
			<h2 class="font-semibold">WireGuard server</h2>
			<p class="hint mt-1">Backend: {backendLabels[wgBackend]}</p>
			<p class="hint mb-4 mt-1">
				Changes apply immediately. Changing the address, port or endpoint means device profiles
				need to be downloaded again.
			</p>
			{#if server.public_key}
				<p class="mb-4 text-xs text-zinc-500">
					Server public key: <span class="break-all font-mono">{server.public_key}</span>
				</p>
			{/if}
			<ServerForm {server} onsaved={serverSaved} />
		</section>

		<section class="card">
			<h2 class="font-semibold">New device defaults</h2>
			<p class="hint mb-4 mt-1">Used when a field is left empty on a device.</p>
			<form class="grid gap-4 sm:grid-cols-2" onsubmit={saveDefaults}>
				<div>
					<label class="label" for="d-dns">DNS</label>
					<input id="d-dns" class="input font-mono" bind:value={d.dns} placeholder="10.8.0.1" />
					<p class="hint">Filled in automatically when AdGuard is installed from the panel; enter the WireGuard server IP manually if you installed AdGuard yourself.</p>
				</div>
				<div>
					<label class="label" for="d-aips">Allowed IPs</label>
					<input id="d-aips" class="input font-mono" bind:value={d.allowed_ips} placeholder="0.0.0.0/0, ::/0" />
					<p class="hint">0.0.0.0/0 = all internet traffic through the VPN.</p>
				</div>
				<div>
					<label class="label" for="d-ka">Keepalive (seconds)</label>
					<input id="d-ka" class="input" type="number" min="0" max="3600" bind:value={d.keepalive} />
					<p class="hint">25 is recommended so phones stay connected behind NAT. 0 = off.</p>
				</div>
				<div>
					<label class="label" for="d-mtu">MTU</label>
					<input id="d-mtu" class="input" type="number" min="0" max="1500" bind:value={d.mtu} placeholder="auto" />
				</div>
				{#if dError}<p class="text-sm text-red-600 sm:col-span-2">{dError}</p>{/if}
				<div class="sm:col-span-2">
					<button class="btn-primary" disabled={dBusy}>{dBusy ? 'Saving…' : 'Save defaults'}</button>
				</div>
			</form>
		</section>

		<section class="card">
			<h2 class="mb-4 font-semibold">Firewall & NAT</h2>
			<FirewallPanel />
		</section>

		<section class="card">
			<h2 class="font-semibold">Security</h2>
			<TwoFactorSettings />
		</section>

		<section class="card">
			<h2 class="mb-4 font-semibold">Change password</h2>
			<form class="grid max-w-md gap-4" onsubmit={changePassword}>
				<input type="text" autocomplete="username" class="hidden" aria-hidden="true" tabindex="-1" />
				<div>
					<label class="label" for="pw-cur">Current password</label>
					<input id="pw-cur" class="input" type="password" bind:value={pw.current} autocomplete="current-password" required />
				</div>
				<div>
					<label class="label" for="pw-new">New password</label>
					<input id="pw-new" class="input" type="password" minlength="10" bind:value={pw.next} autocomplete="new-password" required />
					<p class="hint">At least 10 characters.</p>
				</div>
				<div>
					<label class="label" for="pw-rep">Repeat new password</label>
					<input id="pw-rep" class="input" type="password" bind:value={pw.repeat} autocomplete="new-password" required />
				</div>
				{#if pwError}<p class="text-sm text-red-600">{pwError}</p>{/if}
				<div>
					<button class="btn-primary" disabled={pwBusy}>{pwBusy ? 'Saving…' : 'Change password'}</button>
				</div>
			</form>
		</section>
	</div>
{/if}
