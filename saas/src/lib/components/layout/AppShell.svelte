<script lang="ts" module>
	const STORAGE_KEY = 'rira_sidebar_collapsed';
	let pendingFocusHamburger = false;
</script>

<script lang="ts">
	import { tick, type Snippet } from 'svelte';
	import {
		KeyRound,
		LayoutDashboard,
		ShieldCheck,
		LogOut,
		Menu,
		X,
		PanelLeftClose,
		PanelLeftOpen
	} from '@lucide/svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	interface Props {
		children?: Snippet;
	}

	let { children }: Props = $props();

	type ViewportMode = 'mobile' | 'tablet' | 'desktop';

	function detectViewport(): ViewportMode {
		if (typeof window === 'undefined') return 'desktop';
		if (window.innerWidth < 768) return 'mobile';
		if (window.innerWidth < 1024) return 'tablet';
		return 'desktop';
	}

	function readStoredDesktopCollapsed(): boolean {
		if (typeof window === 'undefined') return false;
		try {
			return window.localStorage.getItem(STORAGE_KEY) === 'true';
		} catch {
			return false;
		}
	}

	let viewport = $state<ViewportMode>(detectViewport());
	let desktopCollapsed = $state<boolean>(readStoredDesktopCollapsed());
	let tabletCollapsed = $state<boolean>(true);
	let drawerOpen = $state<boolean>(false);
	let loggingOut = $state<boolean>(false);

	let hamburgerBtn = $state<HTMLButtonElement | null>(null);
	let drawerEl = $state<HTMLElement | null>(null);
	let drawerFirstFocusEl = $state<HTMLButtonElement | null>(null);

	const isCollapsed = $derived(viewport === 'tablet' ? tabletCollapsed : desktopCollapsed);

	const navItems = [
		{
			label: 'Overview',
			href: '/dashboard',
			icon: LayoutDashboard
		},
		{
			label: 'Security',
			href: '/settings/security',
			icon: ShieldCheck
		}
	] as const;

	const currentPageTitle = $derived(
		navItems.find((item) => item.href === router.pathname)?.label ?? 'Dashboard'
	);

	$effect(() => {
		if (typeof document === 'undefined') return;
		const prevOverflow = document.body.style.overflow;
		document.body.classList.add('overflow-hidden');
		document.body.style.overflow = 'hidden';

		return () => {
			document.body.classList.remove('overflow-hidden');
			document.body.style.overflow = prevOverflow;
		};
	});

	$effect(() => {
		if (pendingFocusHamburger && hamburgerBtn) {
			pendingFocusHamburger = false;
			hamburgerBtn.focus();
		}
	});

	function handleWindowResize() {
		const next = detectViewport();
		if (next !== viewport) {
			if (next === 'tablet') {
				tabletCollapsed = true;
			}
			if (next !== 'mobile' && drawerOpen) {
				drawerOpen = false;
			}
			viewport = next;
		}
	}

	function handleWindowKeydown(e: KeyboardEvent) {
		if (drawerOpen && e.key === 'Escape') {
			e.preventDefault();
			void closeDrawer(true);
		}
	}

	function toggleSidebar() {
		if (viewport === 'tablet') {
			tabletCollapsed = !tabletCollapsed;
		} else {
			desktopCollapsed = !desktopCollapsed;
			try {
				window.localStorage.setItem(STORAGE_KEY, String(desktopCollapsed));
			} catch {
				// Ignore storage quota / privacy errors
			}
		}
	}

	async function openDrawer() {
		drawerOpen = true;
		await tick();
		drawerFirstFocusEl?.focus();
	}

	async function closeDrawer(restoreFocus = true) {
		if (!drawerOpen) return;
		drawerOpen = false;
		if (restoreFocus) {
			await tick();
			hamburgerBtn?.focus();
		}
	}

	function toggleDrawer() {
		if (drawerOpen) {
			void closeDrawer(true);
		} else {
			void openDrawer();
		}
	}

	async function handleDrawerNav(href: string) {
		pendingFocusHamburger = true;
		drawerOpen = false;
		router.navigate(href);
		await tick();
		if (pendingFocusHamburger && hamburgerBtn) {
			pendingFocusHamburger = false;
			hamburgerBtn.focus();
		}
	}

	async function handleLogout() {
		loggingOut = true;
		try {
			await authStore.logout();
			router.navigate('/login', true);
		} finally {
			loggingOut = false;
		}
	}
</script>

<svelte:window onresize={handleWindowResize} onkeydown={handleWindowKeydown} />

<div data-app-shell class="flex h-dvh w-full overflow-hidden bg-cream bg-brutal-grid">
	<!-- Desktop & Tablet Fixed Sidebar (>= 768px) -->
	<aside
		aria-label="Sidebar navigation"
		class="hidden md:flex h-dvh shrink-0 flex-col overflow-y-auto overflow-x-hidden border-r-2 border-ink bg-surface transition-[width] duration-200 ease-in-out {isCollapsed
			? 'w-[64px]'
			: 'w-[260px]'}"
	>
		<div
			class="flex h-16 shrink-0 items-center border-b-2 border-ink {isCollapsed
				? 'justify-center px-2'
				: 'justify-between px-4'}"
		>
			<button
				type="button"
				onclick={() => router.navigate('/dashboard')}
				title="RiraLicense — Overview"
				aria-label="RiraLicense Overview"
				class="flex items-center gap-2.5 cursor-pointer min-w-0 {isCollapsed
					? 'justify-center'
					: ''}"
			>
				<div
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-yellow shadow-brutal-xs"
				>
					<KeyRound class="h-4 w-4 text-ink stroke-[2.5]" aria-hidden="true" />
				</div>
				{#if !isCollapsed}
					<div class="flex flex-col items-start min-w-0">
						<span
							class="font-heading text-lg font-extrabold tracking-tight text-ink leading-none truncate"
						>
							Rira<span class="text-cobalt">License</span>
						</span>
						<span
							class="mt-0.5 font-mono text-[10px] font-bold uppercase tracking-widest text-ink/65 truncate"
						>
							Control Plane
						</span>
					</div>
				{/if}
			</button>
		</div>

		<nav aria-label="Primary" class="flex-1 space-y-1.5 overflow-y-auto px-2.5 py-4">
			{#each navItems as item (item.href)}
				{@const active = router.pathname === item.href}
				{@const Icon = item.icon}
				<button
					type="button"
					onclick={() => router.navigate(item.href)}
					title={item.label}
					aria-label={item.label}
					aria-current={active ? 'page' : undefined}
					class="flex h-10 w-full items-center rounded-md border-2 font-heading text-sm font-bold cursor-pointer transition-colors {isCollapsed
						? 'justify-center px-0'
						: 'gap-3 px-3'} {active
						? 'border-ink bg-yellow text-ink shadow-brutal-xs'
						: 'border-transparent text-ink/80 hover:border-ink hover:bg-cream hover:text-ink'}"
				>
					<Icon class="h-4 w-4 shrink-0" aria-hidden="true" />
					{#if !isCollapsed}
						<span class="truncate">{item.label}</span>
					{/if}
				</button>
			{/each}
		</nav>

		<div class="shrink-0 border-t-2 border-ink p-2.5">
			<button
				type="button"
				onclick={toggleSidebar}
				title={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
				aria-label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
				aria-expanded={!isCollapsed}
				class="flex h-10 w-full items-center rounded-md border-2 border-ink bg-cream font-mono text-xs font-bold uppercase tracking-wider text-ink shadow-brutal-xs cursor-pointer transition-colors hover:bg-yellow {isCollapsed
					? 'justify-center px-0'
					: 'gap-2.5 px-3'}"
			>
				{#if isCollapsed}
					<PanelLeftOpen class="h-4 w-4 shrink-0" aria-hidden="true" />
				{:else}
					<PanelLeftClose class="h-4 w-4 shrink-0" aria-hidden="true" />
					<span class="truncate">Collapse</span>
				{/if}
			</button>
		</div>
	</aside>

	<!-- Mobile Backdrop (< 768px) -->
	{#if drawerOpen}
		<div
			role="presentation"
			aria-hidden="true"
			onclick={() => closeDrawer(true)}
			class="fixed inset-0 z-40 bg-ink/50 md:hidden"
		></div>
	{/if}

	<!-- Mobile Drawer (< 768px) -->
	<aside
		bind:this={drawerEl}
		id="mobile-navigation-drawer"
		aria-label="Mobile navigation"
		aria-hidden={!drawerOpen}
		inert={!drawerOpen ? true : undefined}
		class="fixed inset-y-0 left-0 z-50 flex h-dvh w-[260px] max-w-[85vw] flex-col overflow-y-auto overflow-x-hidden border-r-2 border-ink bg-surface shadow-brutal-lg transition-transform duration-200 ease-in-out md:hidden {drawerOpen
			? 'translate-x-0'
			: '-translate-x-full pointer-events-none invisible'}"
	>
		<div class="flex h-16 shrink-0 items-center justify-between border-b-2 border-ink px-4">
			<button
				type="button"
				onclick={() => handleDrawerNav('/dashboard')}
				title="RiraLicense — Overview"
				aria-label="RiraLicense Overview"
				class="flex items-center gap-2.5 cursor-pointer min-w-0"
			>
				<div
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-yellow shadow-brutal-xs"
				>
					<KeyRound class="h-4 w-4 text-ink stroke-[2.5]" aria-hidden="true" />
				</div>
				<div class="flex flex-col items-start min-w-0">
					<span
						class="font-heading text-lg font-extrabold tracking-tight text-ink leading-none truncate"
					>
						Rira<span class="text-cobalt">License</span>
					</span>
					<span
						class="mt-0.5 font-mono text-[10px] font-bold uppercase tracking-widest text-ink/65 truncate"
					>
						Control Plane
					</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => closeDrawer(true)}
				aria-label="Close navigation menu"
				title="Close navigation menu"
				class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
			>
				<X class="h-4 w-4" aria-hidden="true" />
			</button>
		</div>

		<nav aria-label="Mobile menu" class="flex-1 space-y-1.5 overflow-y-auto px-3 py-4">
			{#each navItems as item, index (item.href)}
				{@const active = router.pathname === item.href}
				{@const Icon = item.icon}
				{#if index === 0}
					<button
						bind:this={drawerFirstFocusEl}
						type="button"
						onclick={() => handleDrawerNav(item.href)}
						title={item.label}
						aria-label={item.label}
						aria-current={active ? 'page' : undefined}
						class="flex h-10 w-full items-center gap-3 rounded-md border-2 px-3 font-heading text-sm font-bold cursor-pointer transition-colors {active
							? 'border-ink bg-yellow text-ink shadow-brutal-xs'
							: 'border-transparent text-ink/80 hover:border-ink hover:bg-cream hover:text-ink'}"
					>
						<Icon class="h-4 w-4 shrink-0" aria-hidden="true" />
						<span class="truncate">{item.label}</span>
					</button>
				{:else}
					<button
						type="button"
						onclick={() => handleDrawerNav(item.href)}
						title={item.label}
						aria-label={item.label}
						aria-current={active ? 'page' : undefined}
						class="flex h-10 w-full items-center gap-3 rounded-md border-2 px-3 font-heading text-sm font-bold cursor-pointer transition-colors {active
							? 'border-ink bg-yellow text-ink shadow-brutal-xs'
							: 'border-transparent text-ink/80 hover:border-ink hover:bg-cream hover:text-ink'}"
					>
						<Icon class="h-4 w-4 shrink-0" aria-hidden="true" />
						<span class="truncate">{item.label}</span>
					</button>
				{/if}
			{/each}
		</nav>
	</aside>

	<!-- Scrollable Content Area -->
	<div
		data-content-scroll
		class="flex h-dvh flex-1 min-w-0 flex-col overflow-x-hidden {drawerOpen
			? 'overflow-y-hidden'
			: 'overflow-y-auto'}"
	>
		<header
			class="sticky top-0 z-30 flex h-16 shrink-0 items-center justify-between gap-4 border-b-2 border-ink bg-surface px-4 lg:px-6"
		>
			<div class="flex items-center gap-3 min-w-0">
				<button
					bind:this={hamburgerBtn}
					type="button"
					onclick={toggleDrawer}
					aria-expanded={drawerOpen}
					aria-controls="mobile-navigation-drawer"
					aria-label={drawerOpen ? 'Close navigation menu' : 'Open navigation menu'}
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow md:hidden"
				>
					<Menu class="h-4 w-4" aria-hidden="true" />
				</button>

				<span class="font-heading text-base font-extrabold tracking-tight text-ink truncate">
					{currentPageTitle}
				</span>
			</div>

			<div class="flex items-center gap-3 shrink-0">
				{#if authStore.user}
					<span
						class="hidden sm:inline-block max-w-[240px] truncate font-mono text-xs font-bold text-ink/75"
					>
						{authStore.user.email}
					</span>
				{/if}
				<Button variant="secondary" size="sm" disabled={loggingOut} onclick={handleLogout}>
					<LogOut class="h-3.5 w-3.5" />
					<span>{loggingOut ? 'Signing out...' : 'Logout'}</span>
				</Button>
			</div>
		</header>

		<main class="flex-1 min-w-0 p-4 lg:p-6">
			<div class="mx-auto w-full max-w-6xl min-w-0">
				{@render children?.()}
			</div>
		</main>
	</div>
</div>

<style>
	:global(body:has([data-app-shell])) {
		overflow: hidden;
		height: 100dvh;
	}

	:global([data-app-shell] main div.border-2.border-ink.rounded-md),
	:global([data-app-shell] main table) {
		min-width: 0;
		max-width: 100%;
		overflow-x: auto;
	}
</style>
