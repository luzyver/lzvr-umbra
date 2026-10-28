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
		act(() => post('/adguard/ctl/filtering/config', { enabled: v, interval: st!.interval }), 'Filter diperbarui.');
	const setInterval_ = (h: number) =>
		act(() => post('/adguard/ctl/filtering/config', { enabled: st!.enabled, interval: h }), 'Jadwal pembaruan disimpan.');
	const toggle = (f: Filter, whitelist: boolean, v: boolean) =>
		act(
			() => post('/adguard/ctl/filtering/set_url', { url: f.url, whitelist, data: { name: f.name, url: f.url, enabled: v } }),
			`${f.name} ${v ? 'diaktifkan' : 'dimatikan'}.`
		);
	function remove(f: Filter, whitelist: boolean) {
		if (!confirm(`Hapus daftar "${f.name}"?`)) return;
		act(() => post('/adguard/ctl/filtering/remove_url', { url: f.url, whitelist }), 'Daftar dihapus.');
	}
	const refresh = () =>
		act(async () => {
			await post('/adguard/ctl/filtering/refresh', { whitelist: false });
			await post('/adguard/ctl/filtering/refresh', { whitelist: true });
		}, 'Daftar filter diperbarui.');
	function addList(e: SubmitEvent) {
		e.preventDefault();
		act(async () => {
			await post('/adguard/ctl/filtering/add_url', add);
			add = { name: '', url: '', whitelist: false };
		}, 'Daftar ditambahkan.');
	}
	const saveRules = () =>
		act(
			() => post('/adguard/ctl/filtering/set_rules', { rules: rules.split('\n').map((r) => r.trim()).filter(Boolean) }),
			'Aturan kustom disimpan.'
		);

	const intervals = [
		[0, 'Tidak pernah'],
		[1, 'Tiap jam'],
		[12, 'Tiap 12 jam'],
		[24, 'Tiap hari'],
		[72, 'Tiap 3 hari'],
		[168, 'Tiap minggu']
	] as const;
</script>

{#if st}
	<div class="space-y-4">
		<div class="card flex flex-wrap items-center justify-between gap-3">
			<div>
				<p class="font-medium">Pemfilteran</p>
				<p class="hint">Matikan untuk menonaktifkan semua daftar blokir sekaligus.</p>
			</div>
			<div class="flex flex-wrap items-center gap-3">
				<select
					class="input"
					value={st.interval}
					disabled={busy}
					onchange={(e) => setInterval_(Number(e.currentTarget.value))}
					aria-label="Jadwal pembaruan"
				>
					{#each intervals as [h, l] (h)}
						<option value={h}>{l}</option>
					{/each}
				</select>
				<button class="btn-ghost" disabled={busy} onclick={refresh}>Perbarui sekarang</button>
				<Toggle checked={st.enabled} disabled={busy} label="Pemfilteran" onchange={setEnabled} />
			</div>
		</div>

		{#each [['Daftar blokir', st.filters, false], ['Daftar izin', st.whitelist_filters, true]] as const as [title, list, wl] (title)}
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
							<span class="hint tabular-nums">{f.rules_count.toLocaleString('id-ID')} aturan</span>
							<button class="btn-danger text-xs" disabled={busy} onclick={() => remove(f, wl)}>Hapus</button>
						</li>
					{:else}
						<li class="hint py-2">Belum ada daftar.</li>
					{/each}
				</ul>
			</div>
		{/each}

		<form class="card space-y-3" onsubmit={addList}>
			<p class="font-medium">Tambah daftar</p>
			<div class="grid gap-3 sm:grid-cols-[1fr_2fr]">
				<input class="input" bind:value={add.name} required placeholder="Nama, mis. OISD" aria-label="Nama daftar" />
				<input class="input font-mono" bind:value={add.url} required placeholder="https://…/list.txt" aria-label="URL daftar" />
			</div>
			<div class="flex flex-wrap items-center justify-between gap-2">
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={add.whitelist} /> Daftar izin (bukan blokir)
				</label>
				<button class="btn-primary" disabled={busy}>Tambah</button>
			</div>
		</form>

		<div class="card space-y-3">
			<div>
				<p class="font-medium">Aturan kustom</p>
				<p class="hint">
					Satu aturan per baris. Contoh: <code>||iklan.com^</code> untuk memblokir,
					<code>@@||situs.com^</code> untuk mengizinkan, <code>! catatan</code> untuk komentar.
				</p>
			</div>
			<textarea class="input h-48 w-full font-mono text-xs" bind:value={rules} spellcheck="false"></textarea>
			<div class="flex justify-end">
				<button class="btn-primary" disabled={busy} onclick={saveRules}>Simpan aturan</button>
			</div>
		</div>
	</div>
{:else}
	<p class="hint">Memuat…</p>
{/if}
