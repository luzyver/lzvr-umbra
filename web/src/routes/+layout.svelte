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
	let fatal = $state('');

	const publicPaths = ['/login', '/setup'];
	const docsUrl = 'https://www.wireguard.com/quickstart/';
	const repoUrl = 'https://github.com/luzyver/lzvr-umbra';
	const nav: { href: string; label: string; icon: string }[] = [
		{ href: '/', label: 'Overview', icon: 'overview' },
		{ href: '/clients', label: 'Devices', icon: 'laptop' },
		{ href: '/activity', label: 'Activity', icon: 'activity' },
		{ href: '/adguard', label: 'AdGuard', icon: 'shield' },
		{ href: '/settings', label: 'Settings', icon: 'settings' }
	];

	// Route guard: no admin yet -> setup; not signed in -> login; WG not enabled -> setup.
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
		// Public pages don't need re-checking on every navigation, once at startup is enough.
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

	// The sidebar can collapse to a row of icons; the choice is saved in this browser.
	const railKey = 'umbra-sidebar';
	let collapsed = $state(readRail());
	function readRail() {
		try { return localStorage.getItem(railKey) === 'collapsed'; } catch { return false; }
	}
	function toggleRail() {
		collapsed = !collapsed;
		try { localStorage.setItem(railKey, collapsed ? 'collapsed' : 'open'); } catch { /* Without storage, the choice only lasts until the page reloads. */ }
	}

	const isActive = (href: string) => (href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href));
	const bare = $derived(publicPaths.includes(page.url.pathname) || !session.me);
</script>

<a class="skip-link" href="#main-content">Skip to content</a>

{#snippet copyright()}
	<footer class="app-footer"><span>© 2026 LUZYVER</span><span>MIT License</span></footer>
{/snippet}

{#if fatal}
	<div class="mx-auto mt-24 max-w-md card text-center">
		<p class="font-semibold text-red-600">The panel could not be loaded</p>
		<p class="hint mt-2">{fatal}</p>
		<button class="btn-ghost mt-4" onclick={() => location.reload()}>Try again</button>
	</div>
{:else if !ready}
	<div class="mt-32 text-center text-sm text-zinc-500">Loading…</div>
{:else if bare}
	<div class="public-shell">
		<div class="public-theme"><ThemeToggle /></div>
		<main id="main-content">{@render children()}</main>
		{@render copyright()}
	</div>
{:else}
	<div class="app-shell" class:collapsed>
		<aside class="app-sidebar" id="app-sidebar">
			<div class="sidebar-top">
				<a href="/" aria-label="Umbra overview"><Logo wordmark={!collapsed} /></a>
				<button class="icon-btn" onclick={toggleRail} aria-expanded={!collapsed} aria-controls="app-sidebar" aria-label={collapsed ? 'Expand menu' : 'Collapse menu'} title={collapsed ? 'Expand menu' : 'Collapse menu'}><Icon name="panel" size={16} /></button>
			</div>
			<nav class="main-nav" aria-label="Main navigation">
				{#each nav as n (n.href)}
					<a href={n.href} class:active={isActive(n.href)} aria-current={isActive(n.href) ? 'page' : undefined} title={collapsed ? n.label : undefined}><Icon name={n.icon} size={18} /><span>{n.label}</span></a>
				{/each}
			</nav>
			<div class="sidebar-foot">
				<div class="sidebar-links">
					<a href={docsUrl} target="_blank" rel="noopener noreferrer" title={collapsed ? 'Docs' : undefined}><Icon name="book" size={14} /><span>Docs</span><span class="ext"><Icon name="external" size={12} /></span></a>
					<a href={repoUrl} target="_blank" rel="noopener noreferrer" title={collapsed ? 'GitHub' : undefined}><Icon name="github" size={14} /><span>GitHub</span><span class="ext"><Icon name="external" size={12} /></span></a>
				</div>
				{#if session.me?.fake_wg}<span class="badge test-badge" title="UMBRA_FAKE_WG is on, no changes are made to the system">{collapsed ? 'Test' : 'Test mode'}</span>{/if}
				<div class="sidebar-user">
					<span>{session.me?.username}</span>
					<ThemeToggle />
					<button class="icon-btn" onclick={logout} aria-label="Sign out" title="Sign out"><Icon name="logout" size={16} /></button>
				</div>
			</div>
		</aside>
		<header class="mobile-bar">
			<a href="/" aria-label="Umbra overview"><Logo /></a>
			{#if session.me?.fake_wg}<span class="badge test-badge" title="UMBRA_FAKE_WG is on, no changes are made to the system">Test mode</span>{/if}
			<a class="icon-btn" href={docsUrl} target="_blank" rel="noopener noreferrer" aria-label="Docs" title="Docs"><Icon name="book" size={17} /></a>
			<a class="icon-btn" href={repoUrl} target="_blank" rel="noopener noreferrer" aria-label="GitHub" title="GitHub"><Icon name="github" size={17} /></a>
			<ThemeToggle />
			<button class="icon-btn" onclick={logout} aria-label="Sign out" title="Sign out"><Icon name="logout" size={17} /></button>
		</header>
		<div class="app-content">
			<main id="main-content" class="page-content">{@render children()}</main>
			{@render copyright()}
		</div>
		<nav class="tab-bar" aria-label="Main navigation">
			{#each nav as n (n.href)}
				<a href={n.href} class:active={isActive(n.href)} aria-current={isActive(n.href) ? 'page' : undefined}><Icon name={n.icon} size={20} /><span>{n.label}</span></a>
			{/each}
		</nav>
	</div>
{/if}
<Toasts />
