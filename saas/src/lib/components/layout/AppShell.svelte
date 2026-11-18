<script lang="ts" module>
	import {
		INITIAL_DUMMY_NOTIFICATIONS,
		type DummyNotification
	} from '$lib/data/dummyNotifications';

	const STORAGE_KEY = 'rira_sidebar_collapsed';

	let pendingFocusHamburger = false;
	let notificationsState = $state<DummyNotification[]>(
		INITIAL_DUMMY_NOTIFICATIONS.map((item) => ({ ...item }))
	);

	export function computeInitials(name?: string | null, email?: string | null): string {
		const trimmedName = (name ?? '').trim();
		if (trimmedName.length > 0) {
			const words = trimmedName.split(/\s+/).filter(Boolean);
			if (words.length >= 2) {
				return `${words[0][0]}${words[1][0]}`.toUpperCase();
			}
			return words[0][0].toUpperCase();
		}
		const trimmedEmail = (email ?? '').trim();
		if (trimmedEmail.length > 0) {
			return trimmedEmail[0].toUpperCase();
		}
		return '?';
	}
</script>

<script lang="ts">
	import { tick, type Snippet } from 'svelte';
	import {
		KeyRound,
		LayoutDashboard,
		LogOut,
		Menu,
		X,
		PanelLeftClose,
		PanelLeftOpen,
		Bell,
		Settings,
		Users,
		TriangleAlert,
		Laptop,
		Globe,
		BarChart3,
		CheckCheck,
		Building2,
		Plus,
		Shield
	} from '@lucide/svelte';
	import { ApiError, resolveAvatarUrl } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import type { DummyNotificationKind } from '$lib/data/dummyNotifications';

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
	let bellOpen = $state<boolean>(false);
	let avatarOpen = $state<boolean>(false);
	let avatarLoadError = $state<boolean>(false);
	let loggingOut = $state<boolean>(false);
	let switchingWorkspace = $state<boolean>(false);
	let createWsOpen = $state<boolean>(false);
	let newWsName = $state<string>('');
	let newWsSlug = $state<string>('');
	let createWsError = $state<string>('');
	let creatingWs = $state<boolean>(false);

	let hamburgerBtn = $state<HTMLButtonElement | null>(null);
	let drawerEl = $state<HTMLElement | null>(null);
	let drawerFirstFocusEl = $state<HTMLButtonElement | null>(null);
	let headerActionsEl = $state<HTMLDivElement | null>(null);
	let bellBtn = $state<HTMLButtonElement | null>(null);
	let avatarBtn = $state<HTMLButtonElement | null>(null);

	const isCollapsed = $derived(viewport === 'tablet' ? tabletCollapsed : desktopCollapsed);

	const navItems = $derived([
		{
			label: 'Overview',
			href: '/dashboard',
			icon: LayoutDashboard
		},
		...(authStore.user?.is_superadmin
			? [
					{
						label: 'Platform',
						href: '/platform',
						icon: Shield
					}
				]
			: [])
	]);

	const currentPageTitle = $derived(
		router.pathname === '/settings/profile'
			? 'Profile Settings'
			: router.pathname === '/settings/security'
				? 'Security Settings'
				: router.pathname === '/workspace/users'
					? 'Workspace Users'
					: router.pathname === '/platform'
						? 'Platform Console'
						: (navItems.find((item) => item.href === router.pathname)?.label ?? 'Overview')
	);

	const userInitials = $derived(
		computeInitials(authStore.user?.name, authStore.user?.email)
	);

	const resolvedHeaderAvatar = $derived(
		resolveAvatarUrl(authStore.user?.avatar_url)
	);

	$effect(() => {
		void resolvedHeaderAvatar;
		avatarLoadError = false;
	});

	const unreadCount = $derived(
		notificationsState.filter((item) => item.unread).length
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

	function handleWindowClick(e: MouseEvent) {
		if (!bellOpen && !avatarOpen) return;
		const target = e.target as Node | null;
		if (headerActionsEl && target && !headerActionsEl.contains(target)) {
			bellOpen = false;
			avatarOpen = false;
		}
	}

	function handleWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape') return;

		if (createWsOpen) {
			e.preventDefault();
			closeCreateWorkspaceModal();
			return;
		}

		if (bellOpen) {
			e.preventDefault();
			bellOpen = false;
			bellBtn?.focus();
			return;
		}

		if (avatarOpen) {
			e.preventDefault();
			avatarOpen = false;
			avatarBtn?.focus();
			return;
		}

		if (drawerOpen) {
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

	function toggleBell() {
		const next = !bellOpen;
		bellOpen = next;
		if (next) {
			avatarOpen = false;
		}
	}

	function toggleAvatar() {
		const next = !avatarOpen;
		avatarOpen = next;
		if (next) {
			bellOpen = false;
		}
	}

	function markAllRead() {
		notificationsState = notificationsState.map((item) => ({
			...item,
			unread: false
		}));
	}

	function notificationIcon(kind: DummyNotificationKind) {
		switch (kind) {
			case 'warning':
				return TriangleAlert;
			case 'activation':
				return Laptop;
			case 'signin':
				return Globe;
			case 'security':
				return KeyRound;
			case 'summary':
				return BarChart3;
		}
	}

	async function openDrawer() {
		bellOpen = false;
		avatarOpen = false;
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

	function handleNavigateSettings() {
		avatarOpen = false;
		router.navigate('/settings/profile');
	}

	async function handleLogout() {
		avatarOpen = false;
		loggingOut = true;
		try {
			await authStore.logout();
			router.navigate('/login', true);
		} finally {
			loggingOut = false;
		}
	}

	async function handleSwitchWorkspace(workspaceId: string) {
		if (switchingWorkspace) return;
		if (!workspaceId) {
			if (!authStore.workspace) return;
			switchingWorkspace = true;
			try {
				await authStore.exitWorkspace();
				if (router.pathname === '/workspace/users') {
					router.navigate('/platform', true);
				}
			} finally {
				switchingWorkspace = false;
			}
			return;
		}
		if (workspaceId === authStore.workspace?.id) return;
		switchingWorkspace = true;
		try {
			await authStore.switchWorkspace(workspaceId);
		} finally {
			switchingWorkspace = false;
		}
	}

	function openCreateWorkspaceModal() {
		newWsName = '';
		newWsSlug = '';
		createWsError = '';
		createWsOpen = true;
	}

	function closeCreateWorkspaceModal() {
		if (creatingWs) return;
		createWsOpen = false;
		createWsError = '';
	}

	async function handleCreateWorkspace(e: SubmitEvent) {
		e.preventDefault();
		const trimmed = newWsName.trim();
		if (trimmed.length < 2) {
			createWsError = 'Workspace name must be at least 2 characters.';
			return;
		}
		createWsError = '';
		creatingWs = true;
		try {
			await authStore.createWorkspace(trimmed, newWsSlug);
			createWsOpen = false;
			newWsName = '';
			newWsSlug = '';
		} catch (err) {
			createWsError = err instanceof ApiError ? err.message : 'Failed to create workspace.';
		} finally {
			creatingWs = false;
		}
	}
</script>

<svelte:window
	onresize={handleWindowResize}
	onclick={handleWindowClick}
	onkeydown={handleWindowKeydown}
/>

{#snippet workspaceSection(isMobile: boolean)}
	{#if authStore.user?.is_superadmin}
		{#if authStore.workspace}
			<div class="shrink-0 border-b-2 border-ink bg-cream/60 px-3 py-2.5 space-y-2">
				<div class="flex items-center justify-between gap-2">
					<span class="flex items-center gap-1.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink/70">
						<Shield class="h-3.5 w-3.5 text-cobalt" aria-hidden="true" />
						<span>Inspecting</span>
					</span>
					<button
						type="button"
						disabled={switchingWorkspace}
						onclick={() => handleSwitchWorkspace('')}
						title="Exit workspace inspection"
						class="inline-flex items-center gap-1 rounded border border-ink bg-surface px-1.5 py-0.5 font-mono text-[10px] font-bold uppercase text-ink shadow-brutal-xs cursor-pointer hover:bg-coral disabled:opacity-60"
					>
						<X class="h-3 w-3" aria-hidden="true" />
						<span>Exit</span>
					</button>
				</div>
				<div class="rounded border-2 border-ink bg-yellow/35 px-2 py-1.5">
					<p class="font-heading text-xs font-extrabold text-ink truncate">
						{authStore.workspace.name}
					</p>
					<p class="font-mono text-[10px] font-bold uppercase text-ink/70 truncate">
						{authStore.workspace.plan} · {authStore.workspace.slug}
					</p>
				</div>
				<button
					type="button"
					onclick={() =>
						isMobile
							? void handleDrawerNav('/workspace/users')
							: router.navigate('/workspace/users')}
					aria-current={router.pathname === '/workspace/users' ? 'page' : undefined}
					class="flex w-full items-center justify-between rounded border-2 border-ink px-2 py-1 font-mono text-[11px] font-bold uppercase text-ink shadow-brutal-xs cursor-pointer transition-colors {router.pathname ===
					'/workspace/users'
						? 'bg-yellow'
						: 'bg-surface hover:bg-yellow'}"
				>
					<span class="flex items-center gap-1.5">
						<Users class="h-3.5 w-3.5 text-cobalt" aria-hidden="true" />
						<span>Members</span>
					</span>
					<span aria-hidden="true">&rarr;</span>
				</button>
			</div>
		{/if}
	{:else if authStore.user}
		<div class="shrink-0 border-b-2 border-ink bg-cream/60 px-3 py-2.5 space-y-2">
			<div class="flex items-center justify-between gap-2">
				<span class="flex items-center gap-1.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink/70">
					<Building2 class="h-3.5 w-3.5 text-cobalt" aria-hidden="true" />
					<span>Workspace</span>
				</span>
				<button
					type="button"
					onclick={() => {
						if (isMobile) void closeDrawer(false);
						openCreateWorkspaceModal();
					}}
					title="Create workspace"
					class="inline-flex items-center gap-1 rounded border border-ink bg-surface px-1.5 py-0.5 font-mono text-[10px] font-bold uppercase text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
				>
					<Plus class="h-3 w-3" aria-hidden="true" />
					<span>New</span>
				</button>
			</div>
			{#if authStore.workspaces.length > 0}
				<select
					aria-label="Active workspace"
					value={authStore.workspace?.id ?? ''}
					disabled={switchingWorkspace}
					onchange={(e) => handleSwitchWorkspace((e.currentTarget as HTMLSelectElement).value)}
					class="w-full rounded border-2 border-ink bg-surface px-2 py-1.5 font-heading text-xs font-bold text-ink shadow-brutal-xs cursor-pointer focus:outline-none focus:ring-2 focus:ring-cobalt disabled:opacity-60"
				>
					{#each authStore.workspaces as ws (ws.id)}
						<option value={ws.id} selected={authStore.workspace?.id === ws.id}>
							{ws.name} ({ws.plan.toUpperCase()})
						</option>
					{/each}
				</select>
			{:else if authStore.workspace}
				<div class="rounded border-2 border-ink bg-surface px-2 py-1.5 font-heading text-xs font-bold text-ink truncate">
					{authStore.workspace.name}
				</div>
			{/if}
			{#if authStore.canViewUsers}
				<button
					type="button"
					onclick={() =>
						isMobile
							? void handleDrawerNav('/workspace/users')
							: router.navigate('/workspace/users')}
					aria-current={router.pathname === '/workspace/users' ? 'page' : undefined}
					class="flex w-full items-center justify-between rounded border-2 border-ink px-2 py-1 font-mono text-[11px] font-bold uppercase text-ink shadow-brutal-xs cursor-pointer transition-colors {router.pathname ===
					'/workspace/users'
						? 'bg-yellow'
						: 'bg-surface hover:bg-yellow'}"
				>
					<span class="flex items-center gap-1.5">
						<Users class="h-3.5 w-3.5 text-cobalt" aria-hidden="true" />
						<span>Members</span>
					</span>
					<span aria-hidden="true">&rarr;</span>
				</button>
			{/if}
		</div>
	{/if}
{/snippet}

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

		{#if !isCollapsed}
			{@render workspaceSection(false)}
		{/if}

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

		{@render workspaceSection(true)}

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

			<!-- Right Side: Bell Icon Button, then Avatar Button -->
			<div bind:this={headerActionsEl} class="relative flex items-center gap-2.5 shrink-0">
				<!-- Bell Button -->
				<button
					bind:this={bellBtn}
					type="button"
					onclick={toggleBell}
					aria-label={unreadCount > 0
						? `Notifications (${unreadCount} unread)`
						: 'Notifications'}
					aria-expanded={bellOpen}
					aria-haspopup="dialog"
					class="relative flex h-9 w-9 items-center justify-center rounded-md border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer transition-colors hover:bg-yellow focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt"
				>
					<Bell class="h-4 w-4 stroke-[2.25]" aria-hidden="true" />
					{#if unreadCount > 0}
						<span
							data-unread-badge
							class=" -top-1.5 -right-1.5 absolute flex h-5 min-w-5 items-center justify-center rounded-sm border-2 border-ink bg-coral px-1 font-mono text-[10px] font-extrabold leading-none text-ink shadow-brutal-xs"
						>
							{unreadCount}
						</span>
					{/if}
				</button>

				<!-- Avatar Button -->
				<button
					bind:this={avatarBtn}
					type="button"
					onclick={toggleAvatar}
					aria-label="Account menu"
					aria-expanded={avatarOpen}
					aria-haspopup="menu"
					class="flex h-9 w-9 items-center justify-center overflow-hidden rounded-md border-2 border-ink bg-yellow font-heading text-xs font-extrabold uppercase tracking-tight text-ink shadow-brutal-xs cursor-pointer transition-colors hover:bg-yellow-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt"
				>
					{#if resolvedHeaderAvatar && !avatarLoadError}
						<img
							src={resolvedHeaderAvatar}
							alt={authStore.user?.name || 'User avatar'}
							class="h-full w-full object-cover"
							onerror={() => (avatarLoadError = true)}
						/>
					{:else}
						<span>{userInitials}</span>
					{/if}
				</button>

				<!-- Notifications Panel -->
				{#if bellOpen}
					<div
						role="dialog"
						aria-label="Notifications"
						class="absolute right-0 top-full z-50 mt-2 w-[calc(100vw-2rem)] max-w-sm sm:w-96 rounded-md border-2 border-ink bg-surface shadow-brutal-md overflow-hidden"
					>
						<div class="flex items-center justify-between gap-2 border-b-2 border-ink bg-cream px-4 py-3">
							<div class="flex items-center gap-2 min-w-0">
								<span class="font-heading text-sm font-extrabold text-ink">
									Notifications
								</span>
								{#if unreadCount > 0}
									<span
										class="rounded-sm border-2 border-ink bg-yellow px-1.5 py-0.5 font-mono text-[10px] font-extrabold uppercase text-ink"
									>
										{unreadCount} Unread
									</span>
								{/if}
							</div>

							<button
								type="button"
								onclick={markAllRead}
								disabled={unreadCount === 0}
								class="inline-flex items-center gap-1 rounded border-2 border-ink bg-surface px-2 py-1 font-mono text-[11px] font-bold text-ink shadow-brutal-xs cursor-pointer transition-colors hover:bg-yellow disabled:opacity-50 disabled:pointer-events-none"
							>
								<CheckCheck class="h-3.5 w-3.5" aria-hidden="true" />
								<span>Mark all read</span>
							</button>
						</div>

						{#if unreadCount === 0}
							<div
								role="status"
								data-notifications-empty
								class="flex items-center gap-2.5 border-b-2 border-ink bg-yellow/35 px-4 py-2.5 text-ink"
							>
								<CheckCheck class="h-4 w-4 shrink-0 text-ink" aria-hidden="true" />
								<p class="font-heading text-xs font-bold">
									All caught up — no unread notifications.
								</p>
							</div>
						{/if}

						<ul class="max-h-80 divide-y-2 divide-ink/15 overflow-y-auto">
							{#each notificationsState as item (item.id)}
								{@const ItemIcon = notificationIcon(item.kind)}
								<li
									class="flex items-start gap-3 px-4 py-3 transition-colors {item.unread
										? 'bg-cream/65'
										: 'bg-surface opacity-75'}"
								>
									<div
										class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md border-2 border-ink shadow-brutal-xs {item.kind ===
										'warning'
											? 'bg-coral text-ink'
											: item.unread
												? 'bg-yellow text-ink'
												: 'bg-cream text-ink/80'}"
									>
										<ItemIcon class="h-4 w-4" aria-hidden="true" />
									</div>

									<div class="min-w-0 flex-1">
										<p
											class="text-xs leading-snug text-ink {item.unread
												? 'font-bold'
												: 'font-medium text-ink/80'}"
										>
											{item.text}
										</p>
										<span class="mt-1 block font-mono text-[11px] font-bold text-ink/60">
											{item.time}
										</span>
									</div>

									{#if item.unread}
										<span
											aria-label="Unread"
											class="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full border border-ink bg-cobalt"
										></span>
									{/if}
								</li>
							{/each}
						</ul>
					</div>
				{/if}

				<!-- Avatar Dropdown -->
				{#if avatarOpen}
					<div
						role="menu"
						aria-label="Account menu"
						class="absolute right-0 top-full z-50 mt-2 w-64 max-w-[calc(100vw-2rem)] rounded-md border-2 border-ink bg-surface shadow-brutal-md overflow-hidden"
					>
						<div class="border-b-2 border-ink bg-cream px-4 py-3">
							<p class="font-heading text-sm font-extrabold text-ink truncate">
								{authStore.user?.name || 'User'}
							</p>
							<p class="mt-0.5 font-mono text-xs font-bold text-ink/75 truncate">
								{authStore.user?.email || ''}
							</p>
						</div>

						<div class="p-1.5">
							<button
								type="button"
								role="menuitem"
								onclick={handleNavigateSettings}
								class="flex w-full items-center gap-2.5 rounded-md border-2 border-transparent px-3 py-2 text-left font-heading text-sm font-bold text-ink cursor-pointer transition-colors hover:border-ink hover:bg-yellow"
							>
								<Settings class="h-4 w-4 shrink-0" aria-hidden="true" />
								<span>Settings</span>
							</button>
						</div>

						<div class="border-t-2 border-ink p-1.5">
							<button
								type="button"
								role="menuitem"
								disabled={loggingOut}
								onclick={handleLogout}
								class="flex w-full items-center gap-2.5 rounded-md border-2 border-transparent px-3 py-2 text-left font-heading text-sm font-bold text-ink cursor-pointer transition-colors hover:border-ink hover:bg-coral disabled:opacity-50"
							>
								<LogOut class="h-4 w-4 shrink-0" aria-hidden="true" />
								<span>{loggingOut ? 'Signing out...' : 'Logout'}</span>
							</button>
						</div>
					</div>
				{/if}
			</div>
		</header>

		<main class="flex-1 min-w-0 p-4 lg:p-6">
			<div class="mx-auto w-full max-w-6xl min-w-0">
				{@render children?.()}
			</div>
		</main>
	</div>
</div>

{#if createWsOpen}
	<div
		role="dialog"
		aria-modal="true"
		aria-labelledby="create-workspace-dialog-title"
		class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
	>
		<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 id="create-workspace-dialog-title" class="font-heading text-lg font-extrabold text-ink">
						Create Workspace
					</h2>
					<p class="text-xs text-ink/70">Create an isolated tenant workspace on the Free plan.</p>
				</div>
				<button
					type="button"
					onclick={closeCreateWorkspaceModal}
					aria-label="Close create workspace dialog"
					class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
				>
					<X class="h-4 w-4" />
				</button>
			</div>

			<form onsubmit={handleCreateWorkspace} class="space-y-4">
				<div>
					<label for="new-workspace-name" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
						Workspace Name
					</label>
					<input
						id="new-workspace-name"
						type="text"
						required
						minlength={2}
						maxlength={120}
						bind:value={newWsName}
						placeholder="Acme Corp"
						class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
					/>
				</div>

				<div>
					<label for="new-workspace-slug" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
						Slug (optional)
					</label>
					<input
						id="new-workspace-slug"
						type="text"
						maxlength={64}
						bind:value={newWsSlug}
						placeholder="acme-corp"
						class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 font-mono text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
					/>
				</div>

				{#if createWsError}
					<p role="alert" class="text-xs font-semibold text-red-600">{createWsError}</p>
				{/if}

				<div class="flex justify-end gap-3">
					<button
						type="button"
						onclick={closeCreateWorkspaceModal}
						disabled={creatingWs}
						class="h-10 rounded-md border-2 border-ink bg-surface px-4 font-heading text-sm font-bold text-ink shadow-brutal-xs cursor-pointer hover:bg-cream disabled:opacity-50"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={creatingWs}
						class="h-10 rounded-md border-2 border-ink bg-yellow px-4 font-heading text-sm font-bold text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow-hover disabled:opacity-50"
					>
						{creatingWs ? 'Creating...' : 'Create Workspace'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	:global(body:has([data-app-shell])) {
		overflow: hidden;
		height: 100dvh;
	}

	@layer base {
		:global([data-app-shell] main div.border-2.border-ink.rounded-md:not([role='dialog'], [role='dialog'] *)),
		:global([data-app-shell] main table) {
			min-width: 0;
			max-width: 100%;
			overflow-x: auto;
		}
	}
</style>
