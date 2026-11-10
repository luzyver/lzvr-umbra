<script lang="ts">
	import { post, errMsg, type AdGuardJob } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import ConnForm from './ConnForm.svelte';

	// Tampilan saat AdGuard belum terhubung: pasang otomatis atau hubungkan manual.
	let { job, ondone }: { job: AdGuardJob; ondone: () => void } = $props();

	let manual = $state(false);
	const running = $derived(job.state === 'running');

	async function install() {
		try {
			await post('/adguard/install');
			ondone();
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}
</script>

<div class="grid gap-4 md:grid-cols-2">
	<div class="card space-y-3 text-sm">
		<p class="font-medium">Pasang AdGuard Home</p>
		<p class="hint">
			Panel mengunduh AdGuard Home, menjalankan setup awal, membuat akun admin, lalu mengarahkan
			DNS klien ke IP WireGuard server. Web AdGuard hanya terbuka di localhost; semua pengaturan
			lewat panel ini.
		</p>
		{#if running}
			<p class="flex items-center gap-2">
				<span class="inline-block h-3 w-3 animate-spin rounded-full border-2 border-zinc-400 border-t-transparent"></span>
				{job.message || 'Sedang memasang…'}
			</p>
		{:else if job.state === 'error' && job.action === 'install'}
			<p class="text-red-600">Gagal: {job.message}</p>
		{/if}
		{#if job.detected}
			<p class="rounded bg-amber-50 p-2 text-amber-800 dark:bg-amber-950 dark:text-amber-200">
				AdGuard Home terdeteksi di 127.0.0.1:3000. Bila belum disiapkan, tombol di bawah
				menjalankan setup awalnya; bila sudah, hubungkan manual.
			</p>
		{/if}
		{#if job.helper_available}
			<button class="btn-primary" disabled={running} onclick={install}>
				{running ? 'Memasang…' : 'Pasang AdGuard'}
			</button>
		{:else}
			<p class="hint">
				Pemasangan otomatis butuh helper root yang dipasang installer versi terbaru. Perbarui Umbra
				(jalankan update.sh) atau jalankan ulang installer dari folder rilis:
			</p>
			<pre class="overflow-x-auto rounded bg-zinc-100 p-3 font-mono text-xs dark:bg-zinc-950">sudo sh umbra-linux-amd64/install.sh --with-adguard umbra-linux-amd64/umbra</pre>
		{/if}
	</div>

	<div class="card space-y-3 text-sm">
		<p class="font-medium">Sudah punya AdGuard Home?</p>
		<p class="hint">Hubungkan instalasi yang ada dengan alamat web dan akun adminnya.</p>
		{#if manual}
			<ConnForm onsaved={ondone} />
		{:else}
			<button class="btn-ghost" disabled={running} onclick={() => (manual = true)}>Hubungkan manual</button>
		{/if}
	</div>
</div>
