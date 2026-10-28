<script lang="ts">
	import { get, put, errMsg, type Client } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Modal from '#lib/Modal.svelte';
	import Toggle from '#lib/Toggle.svelte';
	import ServicePicker from './ServicePicker.svelte';

	type AgClient = {
		use_global_settings?: boolean;
		safebrowsing_enabled?: boolean;
		parental_enabled?: boolean;
		safesearch_enabled?: boolean;
		safe_search?: { enabled: boolean };
		use_global_blocked_services?: boolean;
		blocked_services?: string[] | null;
		upstreams?: string[] | null;
	};

	let clients = $state<Client[] | null>(null);
	let open = $state(false);
	let cur = $state<Client | null>(null);
	let missing = $state(false);
	let busy = $state(false);
	let loading = $state(false);
	let form = $state({
		safebrowsing: false,
		parental: false,
		safesearch: false,
		globalServices: true,
		services: [] as string[],
		upstreams: ''
	});

	get<Client[]>('/clients')
		.then((r) => (clients = r))
		.catch((e) => toast(errMsg(e), 'error'));

	async function edit(c: Client) {
		cur = c;
		missing = false;
		loading = true;
		open = true;
		try {
			const r = await get<{ client: AgClient | null }>(`/adguard/clients/${c.id}`);
			const a = r.client;
			missing = !a;
			form = {
				safebrowsing: !!a?.safebrowsing_enabled,
				parental: !!a?.parental_enabled,
				safesearch: !!(a?.safe_search?.enabled ?? a?.safesearch_enabled),
				globalServices: a?.use_global_blocked_services ?? true,
				services: a?.blocked_services ?? [],
				upstreams: (a?.upstreams ?? []).join('\n')
			};
		} catch (e) {
			toast(errMsg(e), 'error');
			open = false;
		} finally {
			loading = false;
		}
	}

	async function save() {
		if (!cur) return;
		busy = true;
		try {
			await put(`/adguard/clients/${cur.id}`, {
				safebrowsing_enabled: form.safebrowsing,
				parental_enabled: form.parental,
				safe_search_enabled: form.safesearch,
				use_global_blocked_services: form.globalServices,
				blocked_services: form.services,
				upstreams: form.upstreams.split('\n').map((l) => l.trim()).filter(Boolean)
			});
			toast(`Pengaturan AdGuard ${cur.name} disimpan.`);
			open = false;
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

<div class="card">
	<p class="font-medium">Pengaturan per klien</p>
	<p class="hint mb-3">
		Klien dengan "Blokir iklan" aktif didaftarkan ke AdGuard memakai IP WireGuard-nya. Di sini kamu
		bisa memberi aturan khusus, mis. kontrol orang tua hanya untuk HP anak.
	</p>
	{#if clients}
		<ul class="divide-y divide-zinc-200 text-sm dark:divide-zinc-800">
			{#each clients as c (c.id)}
				<li class="flex items-center gap-3 py-2">
					<div class="min-w-0 flex-1">
						<p class="truncate">{c.name}</p>
						<p class="hint font-mono">{c.address}</p>
					</div>
					{#if c.adblock}
						<button class="btn-ghost text-xs" onclick={() => edit(c)}>Atur</button>
					{:else}
						<span class="hint">blokir iklan mati</span>
					{/if}
				</li>
			{:else}
				<li class="hint py-2">Belum ada klien.</li>
			{/each}
		</ul>
	{:else}
		<p class="hint">Memuat…</p>
	{/if}
</div>

<Modal bind:open title="AdGuard · {cur?.name ?? ''}" wide>
	{#if loading}
		<p class="hint">Memuat…</p>
	{:else if missing}
		<p class="text-sm">
			Klien ini belum terdaftar di AdGuard. Tekan "Sinkron klien" di atas halaman, lalu coba lagi.
		</p>
	{:else}
		<div class="space-y-4 text-sm">
			<p class="hint">Menyimpan akan membuat klien ini memakai pengaturannya sendiri, bukan pengaturan global.</p>
			<div class="grid gap-3 sm:grid-cols-3">
				<div class="flex items-center justify-between gap-2">
					<span>Safe browsing</span>
					<Toggle checked={form.safebrowsing} disabled={busy} label="Safe browsing" onchange={(v) => (form.safebrowsing = v)} />
				</div>
				<div class="flex items-center justify-between gap-2">
					<span>Kontrol orang tua</span>
					<Toggle checked={form.parental} disabled={busy} label="Kontrol orang tua" onchange={(v) => (form.parental = v)} />
				</div>
				<div class="flex items-center justify-between gap-2">
					<span>Safe search</span>
					<Toggle checked={form.safesearch} disabled={busy} label="Safe search" onchange={(v) => (form.safesearch = v)} />
				</div>
			</div>

			<div class="space-y-2">
				<label class="flex items-center gap-2">
					<input type="checkbox" bind:checked={form.globalServices} disabled={busy} />
					Pakai daftar layanan diblokir global
				</label>
				{#if !form.globalServices}
					<ServicePicker bind:selected={form.services} disabled={busy} />
				{/if}
			</div>

			<div>
				<label class="label" for="ag-cl-up">Upstream DNS khusus</label>
				<textarea id="ag-cl-up" class="input h-20 w-full font-mono text-xs" bind:value={form.upstreams} spellcheck="false" placeholder="Kosongkan untuk memakai upstream global"></textarea>
			</div>
		</div>
	{/if}
	{#snippet footer()}
		<button class="btn-ghost" onclick={() => (open = false)}>Batal</button>
		{#if !missing}
			<button class="btn-primary" disabled={busy || loading} onclick={save}>Simpan</button>
		{/if}
	{/snippet}
</Modal>
