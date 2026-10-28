<script lang="ts">
	import '../app.css';
	import '../design.css';
	import Logo from '#lib/Logo.svelte';
	import Icon from '#lib/Icon.svelte';
	import ThemeToggle from '#lib/ThemeToggle.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { ApiError, get, post, type Me, type SetupStatus } from '#lib/api.ts';
	import { session } from '#lib/session.svelte.ts';
	import Toasts from '#lib/Toasts.svelte';

	let { children } = $props();

	let ready = $state(false);
	let menuOpen = $state(false);
	let fatal = $state('');

	const publicPaths = ['/login', '/setup'];
	const nav = [
		{ href: '/', label: 'Ringkasan', icon: 'overview' },
		{ href: '/clients', label: 'Perangkat', icon: 'laptop' },
		{ href: '/activity', label: 'Aktivitas', icon: 'activity' },
		{ href: '/adguard', label: 'AdGuard', icon: 'shield' },
		{ href: '/settings', label: 'Pengaturan', icon: 'settings' }
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
		menuOpen = false;
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

<a class="skip-link" href="#main-content">Lewati ke konten</a>

{#if fatal}
	<div class="mx-auto mt-24 max-w-md card text-center">
		<p class="font-semibold text-red-600">Panel tidak bisa dimuat</p>
		<p class="hint mt-2">{fatal}</p>
		<button class="btn-ghost mt-4" onclick={() => location.reload()}>Coba lagi</button>
	</div>
{:else if !ready}
	<div class="mt-32 text-center text-sm text-zinc-500">Memuat…</div>
{:else if bare}
	<div class="public-shell">
		<div class="public-theme"><ThemeToggle /></div>
		<main id="main-content">{@render children()}</main>
	</div>
{:else}
	<div class="app-shell">
		<aside class="app-sidebar" class:open={menuOpen}>
			<div class="sidebar-top">
				<a href="/" aria-label="Umbra — ringkasan"><Logo /></a>
				<button class="icon-btn mobile-menu" aria-label={menuOpen ? 'Tutup navigasi' : 'Buka navigasi'} aria-expanded={menuOpen} aria-controls="main-nav" onclick={() => (menuOpen = !menuOpen)}>
					<Icon name={menuOpen ? 'close' : 'menu'} size={18} />
				</button>
			</div>
			<nav id="main-nav" class="main-nav" aria-label="Navigasi utama">
				{#each nav as n (n.href)}
					{@const active = n.href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(n.href)}
					<a href={n.href} class:active aria-current={active ? 'page' : undefined}><Icon name={n.icon} size={16} /><span>{n.label}</span></a>
				{/each}
			</nav>
			<div class="sidebar-foot">
				{#if session.me?.fake_wg}<span class="badge test-badge" title="UMBRA_FAKE_WG aktif, tidak ada perubahan ke sistem">Mode uji</span>{/if}
				<div class="sidebar-user">
					<span>{session.me?.username}</span>
					<ThemeToggle />
					<button class="icon-btn" onclick={logout} aria-label="Keluar" title="Keluar"><Icon name="logout" size={16} /></button>
				</div>
			</div>
		</aside>
		<main id="main-content" class="page-content">{@render children()}</main>
	</div>
{/if}
<Toasts />
