<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { ApiError, get, post, type Me, type SetupStatus } from '#lib/api.ts';
	import { session } from '#lib/session.svelte.ts';
	import Toasts from '#lib/Toasts.svelte';

	let { children } = $props();

	let ready = $state(false);
	let fatal = $state('');

	const publicPaths = ['/login', '/setup'];
	const nav = [
		{ href: '/', label: 'Dasbor' },
		{ href: '/clients', label: 'Klien' },
		{ href: '/adguard', label: 'AdGuard' },
		{ href: '/settings', label: 'Pengaturan' }
	];

	// Penjaga rute: admin belum ada -> setup; belum masuk -> login; WG belum aktif -> setup.
	async function guard(path: string) {
		try {
			const st = await get<SetupStatus>('/setup/status');
			session.initialized = st.initialized;
			if (st.needs_admin) {
				session.me = null;
				if (path !== '/setup') await goto('/setup', { replaceState: true });
				return;
			}
			try {
				session.me = await get<Me>('/auth/me', { noRedirect: true });
			} catch (e) {
				if (e instanceof ApiError && e.status === 401) {
					session.me = null;
					if (path !== '/login') await goto('/login', { replaceState: true });
					return;
				}
				throw e;
			}
			if (path === '/login') await goto('/', { replaceState: true });
			else if (!st.initialized && path !== '/setup') await goto('/setup', { replaceState: true });
		} catch (e) {
			fatal = e instanceof Error ? e.message : String(e);
		} finally {
			ready = true;
		}
	}

	let last = '';
	$effect(() => {
		const p = page.url.pathname;
		if (p === last) return;
		last = p;
		// Halaman publik tidak perlu dicek ulang setiap pindah, cukup sekali di awal.
		if (!ready || !publicPaths.includes(p)) guard(p);
	});

	async function logout() {
		try {
			await post('/auth/logout');
		} finally {
			session.me = null;
			goto('/login');
		}
	}

	const bare = $derived(publicPaths.includes(page.url.pathname) || !session.me);
</script>

{#if fatal}
	<div class="mx-auto mt-24 max-w-md card text-center">
		<p class="font-semibold text-red-600">Panel tidak bisa dimuat</p>
		<p class="hint mt-2">{fatal}</p>
		<button class="btn-ghost mt-4" onclick={() => location.reload()}>Coba lagi</button>
	</div>
{:else if !ready}
	<div class="mt-32 text-center text-sm text-zinc-500">Memuat…</div>
{:else if bare}
	{@render children()}
{:else}
	<div class="min-h-screen">
		<header class="border-b border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
			<div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
				<a href="/" class="flex items-center gap-2 font-semibold">
					<img src="/favicon.svg" alt="" class="h-7 w-7" />
					Umbra
				</a>
				<nav class="flex flex-wrap gap-1 text-sm">
					{#each nav as n (n.href)}
						{@const active = n.href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(n.href)}
						<a
							href={n.href}
							class="rounded-lg px-3 py-1.5 {active
								? 'bg-teal-50 font-medium text-teal-800 dark:bg-teal-950 dark:text-teal-200'
								: 'text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800'}">{n.label}</a
						>
					{/each}
				</nav>
				<div class="ml-auto flex items-center gap-3 text-sm">
					{#if session.me?.fake_wg}
						<span class="badge bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200" title="UMBRA_FAKE_WG aktif, tidak ada perubahan ke sistem">mode uji</span>
					{/if}
					<span class="text-zinc-500">{session.me?.username}</span>
					<button class="btn-ghost py-1" onclick={logout}>Keluar</button>
				</div>
			</div>
		</header>
		<main class="mx-auto max-w-6xl px-4 py-6">
			{@render children()}
		</main>
	</div>
{/if}

<Toasts />
