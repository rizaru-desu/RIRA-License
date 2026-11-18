<script lang="ts">
	import {
		Shield,
		Building2,
		Users,
		ArrowRight,
		SlidersHorizontal,
		Ban,
		CheckCircle2,
		X,
		Search,
		ArrowUpDown,
		ArrowUp,
		ArrowDown,
		Eye,
		TriangleAlert,
		ChevronRight
	} from '@lucide/svelte';
	import {
		ApiError,
		authApi,
		resolveAvatarUrl,
		type GlobalSuspendImpactDTO,
		type PlatformUserDTO,
		type WorkspaceDTO,
		type WorkspacePlan
	} from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AppShell, { computeInitials } from '$lib/components/layout/AppShell.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import TablePagination from '$lib/components/ui/TablePagination.svelte';

	type SortDir = 'asc' | 'desc';
	type WorkspaceSortKey = 'name' | 'plan' | 'members' | 'status';
	type UserSortKey = 'name' | 'role' | 'workspaces' | 'status';

	let workspaces = $state<WorkspaceDTO[]>([]);
	let platformUsers = $state<PlatformUserDTO[]>([]);
	let loading = $state(true);
	let errorMsg = $state('');
	let successMsg = $state('');

	// All Workspaces filters, sorting, pagination
	let wsSearch = $state('');
	let wsPlanFilter = $state<'all' | WorkspacePlan>('all');
	let wsStatusFilter = $state<'all' | 'active' | 'suspended'>('all');
	let wsSortKey = $state<WorkspaceSortKey>('name');
	let wsSortDir = $state<SortDir>('asc');
	let wsPage = $state(1);
	let wsPageSize = $state(10);

	// Global Users filters, sorting, pagination
	let userSearch = $state('');
	let userRoleFilter = $state<'all' | 'superadmin' | 'user'>('all');
	let userStatusFilter = $state<'all' | 'active' | 'suspended'>('all');
	let userSortKey = $state<UserSortKey>('name');
	let userSortDir = $state<SortDir>('asc');
	let userPage = $state(1);
	let userPageSize = $state(10);

	// Detail dialogs
	let inspectWorkspace = $state<WorkspaceDTO | null>(null);
	let detailUser = $state<PlatformUserDTO | null>(null);

	// Edit Plan / Overrides modal
	let editPlanTarget = $state<WorkspaceDTO | null>(null);
	let editPlanValue = $state<WorkspacePlan>('free');
	let editMaxUsers = $state<string>('');
	let editMaxProducts = $state<string>('');
	let editMaxLicenses = $state<string>('');
	let editOverrideNote = $state<string>('');
	let editPlanError = $state('');
	let savingPlan = $state(false);

	// Suspend confirmation dialog
	let suspendTarget = $state<{
		kind: 'workspace' | 'user';
		id: string;
		label: string;
		sublabel: string;
		meta: string;
	} | null>(null);
	let suspendReason = $state('');
	let suspendError = $state('');
	let suspending = $state(false);
	let suspendImpact = $state<GlobalSuspendImpactDTO | null>(null);
	let loadingImpact = $state(false);
	let impactError = $state('');

	let busyId = $state<string | null>(null);

	// Real KPI summary counts
	const activeWorkspaceCount = $derived(workspaces.filter((w) => w.status === 'active').length);
	const suspendedWorkspaceCount = $derived(
		workspaces.filter((w) => w.status === 'suspended').length
	);

	const hasActiveWsFilters = $derived(
		wsSearch.trim().length > 0 || wsPlanFilter !== 'all' || wsStatusFilter !== 'all'
	);
	const hasActiveUserFilters = $derived(
		userSearch.trim().length > 0 || userRoleFilter !== 'all' || userStatusFilter !== 'all'
	);

	const filteredWorkspaces = $derived.by(() => {
		const q = wsSearch.trim().toLowerCase();
		const list = workspaces.filter((ws) => {
			if (wsPlanFilter !== 'all' && ws.plan !== wsPlanFilter) return false;
			if (wsStatusFilter !== 'all' && ws.status !== wsStatusFilter) return false;
			if (q && !ws.name.toLowerCase().includes(q) && !ws.slug.toLowerCase().includes(q)) {
				return false;
			}
			return true;
		});

		const dir = wsSortDir === 'asc' ? 1 : -1;
		return [...list].sort((a, b) => {
			switch (wsSortKey) {
				case 'name':
					return a.name.localeCompare(b.name) * dir;
				case 'plan':
					return a.plan.localeCompare(b.plan) * dir;
				case 'members':
					return ((a.member_count ?? 0) - (b.member_count ?? 0)) * dir;
				case 'status':
					return a.status.localeCompare(b.status) * dir;
			}
		});
	});

	const wsTotalPages = $derived(Math.max(1, Math.ceil(filteredWorkspaces.length / wsPageSize)));
	const safeWsPage = $derived(Math.min(Math.max(1, wsPage), wsTotalPages));
	const pagedWorkspaces = $derived(
		filteredWorkspaces.slice((safeWsPage - 1) * wsPageSize, safeWsPage * wsPageSize)
	);

	const filteredPlatformUsers = $derived.by(() => {
		const q = userSearch.trim().toLowerCase();
		const list = platformUsers.filter((u) => {
			if (userRoleFilter === 'superadmin' && !u.is_superadmin) return false;
			if (userRoleFilter === 'user' && u.is_superadmin) return false;
			if (userStatusFilter !== 'all' && u.status !== userStatusFilter) return false;
			if (q && !u.name.toLowerCase().includes(q) && !u.email.toLowerCase().includes(q)) {
				return false;
			}
			return true;
		});

		const dir = userSortDir === 'asc' ? 1 : -1;
		return [...list].sort((a, b) => {
			switch (userSortKey) {
				case 'name':
					return a.name.localeCompare(b.name) * dir;
				case 'role':
					return (Number(b.is_superadmin) - Number(a.is_superadmin)) * dir;
				case 'workspaces':
					return (a.workspace_count - b.workspace_count) * dir;
				case 'status':
					return a.status.localeCompare(b.status) * dir;
			}
		});
	});

	const userTotalPages = $derived(Math.max(1, Math.ceil(filteredPlatformUsers.length / userPageSize)));
	const safeUserPage = $derived(Math.min(Math.max(1, userPage), userTotalPages));
	const pagedPlatformUsers = $derived(
		filteredPlatformUsers.slice((safeUserPage - 1) * userPageSize, safeUserPage * userPageSize)
	);

	$effect(() => {
		void wsSearch;
		void wsPlanFilter;
		void wsStatusFilter;
		wsPage = 1;
	});

	$effect(() => {
		if (wsPage > wsTotalPages) {
			wsPage = wsTotalPages;
		} else if (wsPage < 1) {
			wsPage = 1;
		}
	});

	$effect(() => {
		void userSearch;
		void userRoleFilter;
		void userStatusFilter;
		userPage = 1;
	});

	$effect(() => {
		if (userPage > userTotalPages) {
			userPage = userTotalPages;
		} else if (userPage < 1) {
			userPage = 1;
		}
	});

	$effect(() => {
		if (authStore.user?.is_superadmin) {
			void loadPlatformData();
		} else {
			loading = false;
		}
	});

	async function loadPlatformData() {
		loading = true;
		errorMsg = '';
		try {
			const [wsRes, usersRes] = await Promise.all([
				authApi.platformListWorkspaces(),
				authApi.platformListUsers()
			]);
			workspaces = wsRes.workspaces ?? [];
			platformUsers = usersRes.users ?? [];
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to load platform data.';
		} finally {
			loading = false;
		}
	}

	function toggleWorkspaceSort(key: WorkspaceSortKey) {
		if (wsSortKey === key) {
			wsSortDir = wsSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			wsSortKey = key;
			wsSortDir = 'asc';
		}
	}

	function toggleUserSort(key: UserSortKey) {
		if (userSortKey === key) {
			userSortDir = userSortDir === 'asc' ? 'desc' : 'asc';
		} else {
			userSortKey = key;
			userSortDir = 'asc';
		}
	}

	function resetWorkspaceFilters() {
		wsSearch = '';
		wsPlanFilter = 'all';
		wsStatusFilter = 'all';
	}

	function resetUserFilters() {
		userSearch = '';
		userRoleFilter = 'all';
		userStatusFilter = 'all';
	}

	function utilizationStats(ws: WorkspaceDTO) {
		const count = ws.member_count ?? 0;
		const max = ws.max_users;
		const unlimited = max <= 0;
		const overLimit = !unlimited && count > max;
		const atLimit = !unlimited && count === max;
		const pct = unlimited ? 0 : Math.min(100, Math.round((count / max) * 100));
		return { count, max, unlimited, overLimit, atLimit, pct };
	}

	function formatDate(iso: string | null | undefined): string {
		if (!iso) return '—';
		try {
			return new Date(iso).toLocaleDateString('en-US', {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			});
		} catch {
			return iso;
		}
	}

	async function handleSwitchInto(ws: WorkspaceDTO) {
		errorMsg = '';
		successMsg = '';
		busyId = ws.id;
		try {
			await authStore.switchWorkspace(ws.id);
			inspectWorkspace = null;
			router.navigate('/workspace/users');
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to switch workspace.';
		} finally {
			busyId = null;
		}
	}

	function openEditPlanModal(ws: WorkspaceDTO) {
		errorMsg = '';
		successMsg = '';
		inspectWorkspace = null;
		editPlanTarget = ws;
		editPlanValue = ws.plan;
		editMaxUsers = ws.max_users_override ? String(ws.max_users_override) : '';
		editMaxProducts = ws.max_products_override ? String(ws.max_products_override) : '';
		editMaxLicenses = ws.max_licenses_override ? String(ws.max_licenses_override) : '';
		editOverrideNote = ws.override_note ?? '';
		editPlanError = '';
	}

	function closeEditPlanModal() {
		if (savingPlan) return;
		editPlanTarget = null;
		editPlanError = '';
	}

	function parseOptionalInt(raw: string): number | null {
		const t = raw.trim();
		if (!t) return null;
		const parsed = Number.parseInt(t, 10);
		return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
	}

	async function handleSavePlan(e: SubmitEvent) {
		e.preventDefault();
		if (!editPlanTarget) return;
		editPlanError = '';
		savingPlan = true;
		const target = editPlanTarget;
		try {
			const res = await authApi.platformUpdateWorkspacePlan(target.id, {
				plan: editPlanValue,
				max_users_override: parseOptionalInt(editMaxUsers),
				max_products_override: parseOptionalInt(editMaxProducts),
				max_licenses_override: parseOptionalInt(editMaxLicenses),
				override_note: editOverrideNote.trim()
			});
			workspaces = workspaces.map((w) => (w.id === target.id ? res.workspace : w));
			await authStore.refreshUser();
			editPlanTarget = null;
			successMsg = `Updated plan and overrides for ${res.workspace.name}.`;
		} catch (err) {
			editPlanError = err instanceof ApiError ? err.message : 'Failed to update workspace plan.';
		} finally {
			savingPlan = false;
		}
	}

	async function openSuspendModal(
		kind: 'workspace' | 'user',
		id: string,
		label: string,
		sublabel: string,
		meta: string
	) {
		errorMsg = '';
		successMsg = '';
		inspectWorkspace = null;
		detailUser = null;
		suspendTarget = { kind, id, label, sublabel, meta };
		suspendReason = '';
		suspendError = '';
		suspendImpact = null;
		impactError = '';

		if (kind === 'user') {
			loadingImpact = true;
			try {
				suspendImpact = await authApi.platformGetUserSuspendImpact(id);
			} catch (err) {
				impactError =
					err instanceof ApiError ? err.message : 'Failed to evaluate workspace impact.';
			} finally {
				loadingImpact = false;
			}
		} else {
			loadingImpact = false;
		}
	}

	function closeSuspendModal() {
		if (suspending) return;
		suspendTarget = null;
		suspendReason = '';
		suspendError = '';
		suspendImpact = null;
		loadingImpact = false;
		impactError = '';
	}

	async function handleConfirmSuspend(e: SubmitEvent) {
		e.preventDefault();
		if (!suspendTarget || suspending) return;
		if (suspendTarget.kind === 'user' && suspendImpact && !suspendImpact.allowed) {
			return;
		}
		const trimmed = suspendReason.trim();
		if (trimmed.length < 3) {
			suspendError = 'Reason must be at least 3 characters.';
			return;
		}

		suspendError = '';
		suspending = true;
		const target = suspendTarget;
		try {
			if (target.kind === 'workspace') {
				const res = await authApi.platformSuspendWorkspace(target.id, trimmed);
				workspaces = workspaces.map((w) => (w.id === target.id ? res.workspace : w));
				successMsg = `Suspended workspace ${res.workspace.name}.`;
			} else {
				const res = await authApi.platformSuspendUser(target.id, trimmed);
				platformUsers = platformUsers.map((u) => (u.id === target.id ? res.user : u));
				successMsg = `Globally suspended ${res.user.email} and revoked all sessions.`;
			}
			suspendTarget = null;
			suspendReason = '';
			suspendImpact = null;
			await authStore.refreshUser();
			await loadPlatformData();
		} catch (err) {
			suspendError = err instanceof ApiError ? err.message : 'Failed to suspend.';
		} finally {
			suspending = false;
		}
	}

	async function handleActivateWorkspace(ws: WorkspaceDTO) {
		errorMsg = '';
		successMsg = '';
		busyId = ws.id;
		try {
			const res = await authApi.platformActivateWorkspace(ws.id);
			workspaces = workspaces.map((w) => (w.id === ws.id ? res.workspace : w));
			if (inspectWorkspace?.id === ws.id) {
				inspectWorkspace = res.workspace;
			}
			await authStore.refreshUser();
			successMsg = `Activated workspace ${res.workspace.name}.`;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to activate workspace.';
		} finally {
			busyId = null;
		}
	}

	async function handleActivateUser(u: PlatformUserDTO) {
		errorMsg = '';
		successMsg = '';
		busyId = u.id;
		try {
			const res = await authApi.platformActivateUser(u.id);
			platformUsers = platformUsers.map((item) => (item.id === u.id ? res.user : item));
			if (detailUser?.id === u.id) {
				detailUser = res.user;
			}
			successMsg = `Reactivated global account ${res.user.email}.`;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to activate user.';
		} finally {
			busyId = null;
		}
	}

	function handleWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape') return;
		if (suspendTarget) closeSuspendModal();
		else if (editPlanTarget) closeEditPlanModal();
		else if (inspectWorkspace) inspectWorkspace = null;
		else if (detailUser) detailUser = null;
	}
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<AppShell>
	{#if authStore.user?.is_superadmin}
		<div class="space-y-6">
			<!-- A. Page Header -->
			<div class="space-y-2">
				<nav aria-label="Breadcrumb" class="flex items-center gap-1.5 font-mono text-xs font-bold uppercase text-ink/65">
					<button
						type="button"
						onclick={() => router.navigate('/dashboard')}
						class="cursor-pointer hover:text-cobalt hover:underline"
					>
						Overview
					</button>
					<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
					<span class="text-ink">Platform Console</span>
				</nav>

				<div class="flex flex-wrap items-start justify-between gap-3">
					<div>
						<div class="flex items-center gap-2.5">
							<Shield class="h-6 w-6 text-cobalt" aria-hidden="true" />
							<h1 class="font-heading text-3xl font-extrabold text-ink">Platform Console</h1>
						</div>
						<p class="mt-1 text-sm text-ink/75">
							Platform-level management and oversight across all tenant workspaces and global accounts.
						</p>
					</div>
					<Badge variant="cobalt">Super Admin · Platform Scope</Badge>
				</div>
			</div>

			{#if errorMsg}
				<div role="alert" class="rounded-md border-2 border-ink bg-coral/25 px-4 py-3 text-sm font-semibold text-ink shadow-brutal-xs">
					{errorMsg}
				</div>
			{/if}

			{#if successMsg}
				<div role="status" class="rounded-md border-2 border-ink bg-yellow/40 px-4 py-3 text-sm font-semibold text-ink shadow-brutal-xs">
					{successMsg}
				</div>
			{/if}

			<!-- B. Platform Overview (4 Compact KPI Cards) -->
			<section aria-label="Platform overview" class="grid grid-cols-2 gap-4 lg:grid-cols-4">
				<Card padding="sm" shadow="sm">
					<div class="flex items-center justify-between gap-2">
						<span class="font-mono text-[11px] font-extrabold uppercase tracking-wider text-ink/65">
							Total Workspaces
						</span>
						<Building2 class="h-4 w-4 text-cobalt" aria-hidden="true" />
					</div>
					<p class="mt-2 font-heading text-2xl font-extrabold text-ink">
						{loading ? '—' : workspaces.length}
					</p>
				</Card>

				<Card padding="sm" shadow="sm">
					<div class="flex items-center justify-between gap-2">
						<span class="font-mono text-[11px] font-extrabold uppercase tracking-wider text-ink/65">
							Global Users
						</span>
						<Users class="h-4 w-4 text-cobalt" aria-hidden="true" />
					</div>
					<p class="mt-2 font-heading text-2xl font-extrabold text-ink">
						{loading ? '—' : platformUsers.length}
					</p>
				</Card>

				<Card padding="sm" shadow="sm">
					<div class="flex items-center justify-between gap-2">
						<span class="font-mono text-[11px] font-extrabold uppercase tracking-wider text-ink/65">
							Active Workspaces
						</span>
						<CheckCircle2 class="h-4 w-4 text-emerald-700" aria-hidden="true" />
					</div>
					<p class="mt-2 font-heading text-2xl font-extrabold text-ink">
						{loading ? '—' : activeWorkspaceCount}
					</p>
				</Card>

				<Card padding="sm" shadow="sm">
					<div class="flex items-center justify-between gap-2">
						<span class="font-mono text-[11px] font-extrabold uppercase tracking-wider text-ink/65">
							Suspended Workspaces
						</span>
						<Ban class="h-4 w-4 {suspendedWorkspaceCount > 0 ? 'text-red-600' : 'text-ink/55'}" aria-hidden="true" />
					</div>
					<p class="mt-2 font-heading text-2xl font-extrabold {suspendedWorkspaceCount > 0 ? 'text-red-600' : 'text-ink'}">
						{loading ? '—' : suspendedWorkspaceCount}
					</p>
				</Card>
			</section>

			<!-- C. All Workspaces -->
			<Card padding="sm">
				<div class="flex flex-wrap items-center justify-between gap-3 px-2 pb-3">
					<div class="flex items-center gap-2.5">
						<Building2 class="h-5 w-5 text-cobalt" aria-hidden="true" />
						<h2 class="font-heading text-xl font-extrabold text-ink">All Workspaces</h2>
					</div>
					<Badge variant="cream">
						{filteredWorkspaces.length === workspaces.length
							? `${workspaces.length} Total`
							: `${filteredWorkspaces.length} of ${workspaces.length}`}
					</Badge>
				</div>

				{#if !loading}
					<div class="mb-4 grid gap-2.5 px-2 sm:grid-cols-[1fr_140px_140px]">
						<div class="relative">
							<Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink/55" aria-hidden="true" />
							<input
								type="search"
								aria-label="Search workspaces"
								bind:value={wsSearch}
								placeholder="Search workspace name or slug..."
								class="h-9 w-full rounded-md border-2 border-ink bg-cream pl-9 pr-3 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
						<select
							aria-label="Filter by plan"
							bind:value={wsPlanFilter}
							class="h-9 rounded-md border-2 border-ink bg-cream px-2.5 font-heading text-xs font-bold uppercase text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						>
							<option value="all">All Plans</option>
							<option value="free">Free</option>
							<option value="pro">Pro</option>
							<option value="enterprise">Enterprise</option>
						</select>
						<select
							aria-label="Filter by workspace status"
							bind:value={wsStatusFilter}
							class="h-9 rounded-md border-2 border-ink bg-cream px-2.5 font-heading text-xs font-bold uppercase text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						>
							<option value="all">All Status</option>
							<option value="active">Active</option>
							<option value="suspended">Suspended</option>
						</select>
					</div>
				{/if}

				{#if loading}
					<div class="px-2 py-8 text-center font-mono text-sm font-bold text-ink/70">
						Loading workspaces...
					</div>
				{:else if filteredWorkspaces.length === 0}
					<div class="mx-2 rounded-md border-2 border-dashed border-ink/40 bg-cream/40 px-4 py-8 text-center space-y-3">
						<p class="font-mono text-sm font-bold text-ink/75">
							{hasActiveWsFilters ? 'No workspaces match your current search or filters.' : 'No tenant workspaces found.'}
						</p>
						{#if hasActiveWsFilters}
							<Button variant="secondary" size="sm" onclick={resetWorkspaceFilters}>
								Reset Filters
							</Button>
						{/if}
					</div>
				{:else}
					<!-- Desktop & Tablet Table -->
					<div class="hidden md:block overflow-x-auto rounded-md border-2 border-ink bg-surface">
						<table class="w-full border-collapse text-left text-sm">
							<thead>
								<tr class="border-b-2 border-ink bg-cream font-mono text-xs font-extrabold uppercase tracking-wider text-ink">
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleWorkspaceSort('name')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Workspace</span>
											{#if wsSortKey === 'name'}
												{#if wsSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleWorkspaceSort('plan')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Plan</span>
											{#if wsSortKey === 'plan'}
												{#if wsSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleWorkspaceSort('members')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Members</span>
											{#if wsSortKey === 'members'}
												{#if wsSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleWorkspaceSort('status')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Status</span>
											{#if wsSortKey === 'status'}
												{#if wsSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3 text-right">Actions</th>
								</tr>
							</thead>
							<tbody class="divide-y-2 divide-ink/15">
								{#each pagedWorkspaces as ws (ws.id)}
									{@const isInspecting = authStore.workspace?.id === ws.id}
									{@const util = utilizationStats(ws)}
									<tr class={ws.status === 'suspended' ? 'opacity-70 transition-colors hover:bg-cream/50' : 'transition-colors hover:bg-cream/50'}>
										<td class="px-3 lg:px-4 py-3.5">
											<div class="flex flex-wrap items-center gap-1.5">
												<button
													type="button"
													disabled={busyId === ws.id}
													onclick={() => handleSwitchInto(ws)}
													class="font-heading font-extrabold text-ink text-left cursor-pointer hover:text-cobalt hover:underline disabled:opacity-60"
												>
													{ws.name}
												</button>
												{#if isInspecting}
													<Badge variant="yellow">Inspecting</Badge>
												{/if}
											</div>
											<p class="font-mono text-xs text-ink/70">{ws.slug}</p>
										</td>

										<td class="px-3 lg:px-4 py-3.5">
											<div class="flex flex-wrap items-center gap-1.5">
												<Badge variant={ws.plan === 'free' ? 'yellow' : 'cobalt'}>{ws.plan}</Badge>
												{#if ws.max_users_override || ws.max_products_override || ws.max_licenses_override}
													<span class="rounded border border-ink bg-cream px-1.5 py-0.5 font-mono text-[10px] font-bold uppercase text-ink">
														Override
													</span>
												{/if}
											</div>
											{#if ws.override_note}
												<p class="mt-1 font-mono text-[11px] text-ink/70 truncate max-w-[180px]" title={ws.override_note}>
													{ws.override_note}
												</p>
											{/if}
										</td>

										<td class="px-3 lg:px-4 py-3.5">
											<div class="space-y-1.5 min-w-[130px]">
												<div class="flex items-center gap-2 font-mono text-xs font-bold {util.overLimit ? 'text-red-600' : 'text-ink'}">
													<span>
														{util.count} / {util.unlimited ? '∞' : util.max} Members
													</span>
													{#if util.overLimit}
														<span class="rounded border border-red-600 bg-red-50 px-1 py-0.5 text-[10px] font-extrabold uppercase text-red-700">
															Over Limit
														</span>
													{/if}
												</div>
												{#if !util.unlimited}
													<div
														aria-label={`Member utilization ${util.count} of ${util.max}`}
														class="h-2 w-32 overflow-hidden rounded-sm border border-ink bg-cream"
													>
														<div
															style="width: {util.pct}%"
															class="h-full {util.overLimit
																? 'bg-red-500'
																: util.atLimit
																	? 'bg-yellow'
																	: 'bg-cobalt'}"
														></div>
													</div>
												{:else}
													<span class="block font-mono text-[11px] text-ink/65">Unlimited seats</span>
												{/if}
											</div>
										</td>

										<td class="px-3 lg:px-4 py-3.5">
											<Badge variant={ws.status === 'active' ? 'green' : 'red'}>{ws.status}</Badge>
											{#if ws.status === 'suspended' && ws.suspended_reason}
												<p class="mt-1 font-mono text-xs text-ink/75 max-w-[180px] truncate" title={ws.suspended_reason}>
													{ws.suspended_reason}
												</p>
											{/if}
										</td>

										<td class="px-3 lg:px-4 py-3.5 text-right whitespace-nowrap">
											<div class="inline-flex items-center justify-end gap-2">
												<Button
													variant={isInspecting ? 'yellow' : 'primary'}
													size="sm"
													disabled={busyId === ws.id}
													onclick={() => (inspectWorkspace = ws)}
												>
													<Eye class="h-3.5 w-3.5" />
													<span>Inspect</span>
												</Button>
												<Button variant="secondary" size="sm" onclick={() => openEditPlanModal(ws)}>
													<SlidersHorizontal class="h-3.5 w-3.5" />
													<span>Plan</span>
												</Button>
												{#if ws.status === 'active'}
													<Button
														variant="danger-outline"
														size="sm"
														onclick={() =>
															openSuspendModal(
																'workspace',
																ws.id,
																ws.name,
																ws.slug,
																`${util.count} member(s) · ${ws.plan.toUpperCase()} plan`
															)}
													>
														<Ban class="h-3.5 w-3.5" />
														<span>Suspend</span>
													</Button>
												{:else}
													<Button
														variant="secondary"
														size="sm"
														disabled={busyId === ws.id}
														onclick={() => handleActivateWorkspace(ws)}
													>
														<CheckCircle2 class="h-3.5 w-3.5" />
														<span>Activate</span>
													</Button>
												{/if}
											</div>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					<!-- Mobile Stacked Cards -->
					<div class="space-y-3 md:hidden px-1">
						{#each pagedWorkspaces as ws (ws.id)}
							{@const isInspecting = authStore.workspace?.id === ws.id}
							{@const util = utilizationStats(ws)}
							<div class="rounded-md border-2 border-ink bg-surface p-4 shadow-brutal-xs space-y-3 {ws.status === 'suspended' ? 'opacity-75' : ''}">
								<div class="flex items-start justify-between gap-2">
									<div class="min-w-0">
										<div class="flex flex-wrap items-center gap-1.5">
											<button
												type="button"
												disabled={busyId === ws.id}
												onclick={() => handleSwitchInto(ws)}
												class="font-heading text-base font-extrabold text-ink text-left cursor-pointer hover:text-cobalt hover:underline"
											>
												{ws.name}
											</button>
											{#if isInspecting}
												<Badge variant="yellow">Inspecting</Badge>
											{/if}
										</div>
										<p class="font-mono text-xs text-ink/70 truncate">{ws.slug}</p>
									</div>
									<Badge variant={ws.status === 'active' ? 'green' : 'red'}>{ws.status}</Badge>
								</div>

								<div class="flex flex-wrap items-center justify-between gap-2 border-y-2 border-ink/10 py-2.5">
									<Badge variant={ws.plan === 'free' ? 'yellow' : 'cobalt'}>{ws.plan}</Badge>
									<div class="text-right">
										<div class="font-mono text-xs font-bold {util.overLimit ? 'text-red-600' : 'text-ink'}">
											{util.count} / {util.unlimited ? '∞' : util.max} Members
											{#if util.overLimit}
												<span class="ml-1 text-[10px] font-extrabold uppercase">(Over Limit)</span>
											{/if}
										</div>
										{#if !util.unlimited}
											<div class="mt-1 h-2 w-28 ml-auto overflow-hidden rounded-sm border border-ink bg-cream">
												<div
													style="width: {util.pct}%"
													class="h-full {util.overLimit ? 'bg-red-500' : util.atLimit ? 'bg-yellow' : 'bg-cobalt'}"
												></div>
											</div>
										{/if}
									</div>
								</div>

								{#if ws.status === 'suspended' && ws.suspended_reason}
									<p class="font-mono text-xs text-ink/75">Reason: {ws.suspended_reason}</p>
								{/if}

								<div class="flex flex-wrap items-center justify-between gap-2 pt-1">
									<div class="flex flex-wrap items-center gap-2">
										<Button
											variant={isInspecting ? 'yellow' : 'primary'}
											size="sm"
											disabled={busyId === ws.id}
											onclick={() => (inspectWorkspace = ws)}
										>
											<Eye class="h-3.5 w-3.5" />
											<span>Inspect</span>
										</Button>
										<Button variant="secondary" size="sm" onclick={() => openEditPlanModal(ws)}>
											<SlidersHorizontal class="h-3.5 w-3.5" />
											<span>Plan</span>
										</Button>
									</div>
									{#if ws.status === 'active'}
										<Button
											variant="danger-outline"
											size="sm"
											onclick={() =>
												openSuspendModal(
													'workspace',
													ws.id,
													ws.name,
													ws.slug,
													`${util.count} member(s) · ${ws.plan.toUpperCase()} plan`
												)}
										>
											<Ban class="h-3.5 w-3.5" />
											<span>Suspend</span>
										</Button>
									{:else}
										<Button
											variant="secondary"
											size="sm"
											disabled={busyId === ws.id}
											onclick={() => handleActivateWorkspace(ws)}
										>
											<CheckCircle2 class="h-3.5 w-3.5" />
											<span>Activate</span>
										</Button>
									{/if}
								</div>
							</div>
						{/each}
					</div>

					<TablePagination
						bind:page={wsPage}
						bind:pageSize={wsPageSize}
						totalItems={filteredWorkspaces.length}
						ariaLabel="All workspaces pagination"
					/>
				{/if}
			</Card>

			<!-- D. Global Users -->
			<Card padding="sm">
				<div class="flex flex-wrap items-center justify-between gap-3 px-2 pb-3">
					<div class="flex items-center gap-2.5">
						<Users class="h-5 w-5 text-cobalt" aria-hidden="true" />
						<h2 class="font-heading text-xl font-extrabold text-ink">Global Users</h2>
					</div>
					<Badge variant="cream">
						{filteredPlatformUsers.length === platformUsers.length
							? `${platformUsers.length} Accounts`
							: `${filteredPlatformUsers.length} of ${platformUsers.length}`}
					</Badge>
				</div>

				{#if !loading}
					<div class="mb-4 grid gap-2.5 px-2 sm:grid-cols-[1fr_150px_140px]">
						<div class="relative">
							<Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink/55" aria-hidden="true" />
							<input
								type="search"
								aria-label="Search global users"
								bind:value={userSearch}
								placeholder="Search name or email..."
								class="h-9 w-full rounded-md border-2 border-ink bg-cream pl-9 pr-3 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
						<select
							aria-label="Filter by role"
							bind:value={userRoleFilter}
							class="h-9 rounded-md border-2 border-ink bg-cream px-2.5 font-heading text-xs font-bold uppercase text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						>
							<option value="all">All Roles</option>
							<option value="superadmin">Super Admin</option>
							<option value="user">User</option>
						</select>
						<select
							aria-label="Filter by user status"
							bind:value={userStatusFilter}
							class="h-9 rounded-md border-2 border-ink bg-cream px-2.5 font-heading text-xs font-bold uppercase text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						>
							<option value="all">All Status</option>
							<option value="active">Active</option>
							<option value="suspended">Suspended</option>
						</select>
					</div>
				{/if}

				{#if loading}
					<div class="px-2 py-8 text-center font-mono text-sm font-bold text-ink/70">
						Loading users...
					</div>
				{:else if filteredPlatformUsers.length === 0}
					<div class="mx-2 rounded-md border-2 border-dashed border-ink/40 bg-cream/40 px-4 py-8 text-center space-y-3">
						<p class="font-mono text-sm font-bold text-ink/75">
							{hasActiveUserFilters ? 'No user accounts match your current search or filters.' : 'No global accounts found.'}
						</p>
						{#if hasActiveUserFilters}
							<Button variant="secondary" size="sm" onclick={resetUserFilters}>
								Reset Filters
							</Button>
						{/if}
					</div>
				{:else}
					<!-- Desktop & Tablet Table -->
					<div class="hidden md:block overflow-x-auto rounded-md border-2 border-ink bg-surface">
						<table class="w-full border-collapse text-left text-sm">
							<thead>
								<tr class="border-b-2 border-ink bg-cream font-mono text-xs font-extrabold uppercase tracking-wider text-ink">
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleUserSort('name')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>User</span>
											{#if userSortKey === 'name'}
												{#if userSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleUserSort('role')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Role</span>
											{#if userSortKey === 'role'}
												{#if userSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleUserSort('workspaces')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Workspaces</span>
											{#if userSortKey === 'workspaces'}
												{#if userSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3">
										<button
											type="button"
											onclick={() => toggleUserSort('status')}
											class="inline-flex items-center gap-1 cursor-pointer hover:text-cobalt"
										>
											<span>Global Status</span>
											{#if userSortKey === 'status'}
												{#if userSortDir === 'asc'}
													<ArrowUp class="h-3.5 w-3.5 text-cobalt" />
												{:else}
													<ArrowDown class="h-3.5 w-3.5 text-cobalt" />
												{/if}
											{:else}
												<ArrowUpDown class="h-3.5 w-3.5 opacity-45" />
											{/if}
										</button>
									</th>
									<th class="px-3 lg:px-4 py-3 text-right">Actions</th>
								</tr>
							</thead>
							<tbody class="divide-y-2 divide-ink/15">
								{#each pagedPlatformUsers as u (u.id)}
									{@const avatarUrl = resolveAvatarUrl(u.avatar_url)}
									{@const isSelf = u.id === authStore.user?.id}
									{@const canGlobalAction = !isSelf && !u.is_superadmin}
									<tr class={u.status === 'suspended' ? 'opacity-70 transition-colors hover:bg-cream/50' : 'transition-colors hover:bg-cream/50'}>
										<td class="px-3 lg:px-4 py-3.5">
											<div class="flex items-center gap-3">
												<div class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-md border-2 border-ink bg-yellow font-heading text-xs font-extrabold uppercase text-ink shadow-brutal-xs">
													{#if avatarUrl}
														<img src={avatarUrl} alt={u.name} class="h-full w-full object-cover" />
													{:else}
														<span>{computeInitials(u.name, u.email)}</span>
													{/if}
												</div>
												<div class="min-w-0">
													<div class="flex flex-wrap items-center gap-1.5">
														<button
															type="button"
															onclick={() => (detailUser = u)}
															class="font-heading font-extrabold text-ink text-left truncate cursor-pointer hover:text-cobalt hover:underline"
														>
															{u.name}
														</button>
														{#if isSelf}
															<span class="rounded border border-ink bg-yellow/60 px-1.5 py-0.5 font-mono text-[10px] font-extrabold uppercase text-ink">
																You
															</span>
														{/if}
													</div>
													<p class="font-mono text-xs text-ink/70 truncate">{u.email}</p>
												</div>
											</div>
										</td>

										<td class="px-3 lg:px-4 py-3.5">
											<Badge variant={u.is_superadmin ? 'cobalt' : 'cream'}>
												{u.is_superadmin ? 'Super Admin' : 'User'}
											</Badge>
										</td>

										<td class="px-3 lg:px-4 py-3.5 font-mono text-xs font-bold text-ink">
											{u.workspace_count}
										</td>

										<td class="px-3 lg:px-4 py-3.5">
											<Badge variant={u.status === 'active' ? 'green' : 'red'}>{u.status}</Badge>
											{#if u.status === 'suspended' && u.suspended_reason}
												<p class="mt-1 font-mono text-xs text-ink/75 max-w-[180px] truncate" title={u.suspended_reason}>
													{u.suspended_reason}
												</p>
											{/if}
										</td>

										<td class="px-3 lg:px-4 py-3.5 text-right whitespace-nowrap">
											<div class="inline-flex items-center justify-end gap-2">
												<Button variant="secondary" size="sm" onclick={() => (detailUser = u)}>
													<Eye class="h-3.5 w-3.5" />
													<span>Details</span>
												</Button>
												{#if canGlobalAction}
													{#if u.status === 'active'}
														<Button
															variant="danger-outline"
															size="sm"
															onclick={() =>
																openSuspendModal(
																	'user',
																	u.id,
																	u.name,
																	u.email,
																	`${u.workspace_count} workspace membership(s)`
																)}
														>
															<Ban class="h-3.5 w-3.5" />
															<span>Global Suspend</span>
														</Button>
													{:else}
														<Button
															variant="secondary"
															size="sm"
															disabled={busyId === u.id}
															onclick={() => handleActivateUser(u)}
														>
															<CheckCircle2 class="h-3.5 w-3.5" />
															<span>Activate</span>
														</Button>
													{/if}
												{/if}
											</div>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					<!-- Mobile Stacked Cards -->
					<div class="space-y-3 md:hidden px-1">
						{#each pagedPlatformUsers as u (u.id)}
							{@const avatarUrl = resolveAvatarUrl(u.avatar_url)}
							{@const isSelf = u.id === authStore.user?.id}
							{@const canGlobalAction = !isSelf && !u.is_superadmin}
							<div class="rounded-md border-2 border-ink bg-surface p-4 shadow-brutal-xs space-y-3 {u.status === 'suspended' ? 'opacity-75' : ''}">
								<div class="flex items-start justify-between gap-2">
									<div class="flex items-center gap-3 min-w-0">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-md border-2 border-ink bg-yellow font-heading text-xs font-extrabold uppercase text-ink shadow-brutal-xs">
											{#if avatarUrl}
												<img src={avatarUrl} alt={u.name} class="h-full w-full object-cover" />
											{:else}
												<span>{computeInitials(u.name, u.email)}</span>
											{/if}
										</div>
										<div class="min-w-0">
											<div class="flex flex-wrap items-center gap-1.5">
												<button
													type="button"
													onclick={() => (detailUser = u)}
													class="font-heading font-extrabold text-ink text-left truncate cursor-pointer hover:text-cobalt hover:underline"
												>
													{u.name}
												</button>
												{#if isSelf}
													<span class="rounded border border-ink bg-yellow/60 px-1.5 py-0.5 font-mono text-[10px] font-extrabold uppercase text-ink">
														You
													</span>
												{/if}
											</div>
											<p class="font-mono text-xs text-ink/70 truncate">{u.email}</p>
										</div>
									</div>
									<Badge variant={u.status === 'active' ? 'green' : 'red'}>{u.status}</Badge>
								</div>

								<div class="flex flex-wrap items-center justify-between gap-2 border-y-2 border-ink/10 py-2">
									<Badge variant={u.is_superadmin ? 'cobalt' : 'cream'}>
										{u.is_superadmin ? 'Super Admin' : 'User'}
									</Badge>
									<span class="font-mono text-xs font-bold text-ink">
										{u.workspace_count} {u.workspace_count === 1 ? 'workspace' : 'workspaces'}
									</span>
								</div>

								{#if u.status === 'suspended' && u.suspended_reason}
									<p class="font-mono text-xs text-ink/75">Reason: {u.suspended_reason}</p>
								{/if}

								<div class="flex flex-wrap items-center justify-between gap-2 pt-1">
									<Button variant="secondary" size="sm" onclick={() => (detailUser = u)}>
										<Eye class="h-3.5 w-3.5" />
										<span>Details</span>
									</Button>
									{#if canGlobalAction}
										{#if u.status === 'active'}
											<Button
												variant="danger-outline"
												size="sm"
												onclick={() =>
													openSuspendModal(
														'user',
														u.id,
														u.name,
														u.email,
														`${u.workspace_count} workspace membership(s)`
													)}
											>
												<Ban class="h-3.5 w-3.5" />
												<span>Global Suspend</span>
											</Button>
										{:else}
											<Button
												variant="secondary"
												size="sm"
												disabled={busyId === u.id}
												onclick={() => handleActivateUser(u)}
											>
												<CheckCircle2 class="h-3.5 w-3.5" />
												<span>Activate</span>
											</Button>
										{/if}
									{/if}
								</div>
							</div>
						{/each}
					</div>

					<TablePagination
						bind:page={userPage}
						bind:pageSize={userPageSize}
						totalItems={filteredPlatformUsers.length}
						ariaLabel="Global users pagination"
					/>
				{/if}
			</Card>
		</div>
	{/if}

	<!-- Workspace Inspect Details Modal -->
	{#if inspectWorkspace}
		{@const ws = inspectWorkspace}
		{@const util = utilizationStats(ws)}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="inspect-ws-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-lg rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<div class="flex flex-wrap items-center gap-2">
							<h2 id="inspect-ws-dialog-title" class="font-heading text-xl font-extrabold text-ink truncate">
								{ws.name}
							</h2>
							<Badge variant={ws.plan === 'free' ? 'yellow' : 'cobalt'}>{ws.plan}</Badge>
							<Badge variant={ws.status === 'active' ? 'green' : 'red'}>{ws.status}</Badge>
						</div>
						<p class="mt-0.5 font-mono text-xs text-ink/70">Slug: {ws.slug} · Created {formatDate(ws.created_at)}</p>
					</div>
					<button
						type="button"
						onclick={() => (inspectWorkspace = null)}
						aria-label="Close workspace details"
						class="flex h-8 w-8 shrink-0 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<div class="grid grid-cols-3 gap-3 rounded-md border-2 border-ink bg-cream/60 p-3.5">
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Seats</span>
						<span class="font-heading text-base font-extrabold {util.overLimit ? 'text-red-600' : 'text-ink'}">
							{util.count} / {util.unlimited ? '∞' : util.max}
						</span>
					</div>
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Max Products</span>
						<span class="font-heading text-base font-extrabold text-ink">
							{ws.max_products === 0 ? '∞' : ws.max_products}
						</span>
					</div>
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Max Licenses</span>
						<span class="font-heading text-base font-extrabold text-ink">
							{ws.max_licenses === 0 ? '∞' : ws.max_licenses}
						</span>
					</div>
				</div>

				{#if ws.override_note}
					<div class="rounded-md border-2 border-ink bg-yellow/25 px-3.5 py-2.5 text-xs">
						<span class="font-mono font-extrabold uppercase text-ink">Override Note:</span>
						<span class="ml-1 font-medium text-ink/85">{ws.override_note}</span>
					</div>
				{/if}

				{#if ws.status === 'suspended' && ws.suspended_reason}
					<div class="rounded-md border-2 border-ink bg-coral/20 px-3.5 py-2.5 text-xs">
						<span class="font-mono font-extrabold uppercase text-ink">Suspended Reason:</span>
						<span class="ml-1 font-medium text-ink/90">{ws.suspended_reason}</span>
					</div>
				{/if}

				<div class="flex flex-wrap items-center justify-between gap-2 pt-2 border-t-2 border-ink/15">
					<div class="flex flex-wrap items-center gap-2">
						<Button variant="secondary" size="sm" onclick={() => openEditPlanModal(ws)}>
							<SlidersHorizontal class="h-3.5 w-3.5" />
							<span>Configure Plan</span>
						</Button>
						{#if ws.status === 'active'}
							<Button
								variant="danger-outline"
								size="sm"
								onclick={() =>
									openSuspendModal(
										'workspace',
										ws.id,
										ws.name,
										ws.slug,
										`${util.count} member(s) · ${ws.plan.toUpperCase()} plan`
									)}
							>
								<Ban class="h-3.5 w-3.5" />
								<span>Suspend</span>
							</Button>
						{:else}
							<Button
								variant="secondary"
								size="sm"
								disabled={busyId === ws.id}
								onclick={() => handleActivateWorkspace(ws)}
							>
								<CheckCircle2 class="h-3.5 w-3.5" />
								<span>Activate</span>
							</Button>
						{/if}
					</div>
					<Button
						variant="primary"
						size="sm"
						disabled={busyId === ws.id}
						onclick={() => handleSwitchInto(ws)}
					>
						<span>Open Workspace Members</span>
						<ArrowRight class="h-3.5 w-3.5" />
					</Button>
				</div>
			</div>
		</div>
	{/if}

	<!-- User Account Details Modal -->
	{#if detailUser}
		{@const u = detailUser}
		{@const avatarUrl = resolveAvatarUrl(u.avatar_url)}
		{@const isSelf = u.id === authStore.user?.id}
		{@const canGlobalAction = !isSelf && !u.is_superadmin}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="user-detail-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div class="flex items-center gap-3 min-w-0">
						<div class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-md border-2 border-ink bg-yellow font-heading text-sm font-extrabold uppercase text-ink shadow-brutal-xs">
							{#if avatarUrl}
								<img src={avatarUrl} alt={u.name} class="h-full w-full object-cover" />
							{:else}
								<span>{computeInitials(u.name, u.email)}</span>
							{/if}
						</div>
						<div class="min-w-0">
							<div class="flex flex-wrap items-center gap-1.5">
								<h2 id="user-detail-dialog-title" class="font-heading text-lg font-extrabold text-ink truncate">
									{u.name}
								</h2>
								{#if isSelf}
									<span class="rounded border border-ink bg-yellow/60 px-1.5 py-0.5 font-mono text-[10px] font-extrabold uppercase text-ink">
										You
									</span>
								{/if}
							</div>
							<p class="font-mono text-xs text-ink/70 truncate">{u.email}</p>
						</div>
					</div>
					<button
						type="button"
						onclick={() => (detailUser = null)}
						aria-label="Close user details"
						class="flex h-8 w-8 shrink-0 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<div class="grid grid-cols-2 gap-3 rounded-md border-2 border-ink bg-cream/60 p-3.5 text-xs">
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Role</span>
						<div class="mt-1">
							<Badge variant={u.is_superadmin ? 'cobalt' : 'cream'}>
								{u.is_superadmin ? 'Super Admin' : 'User'}
							</Badge>
						</div>
					</div>
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Global Status</span>
						<div class="mt-1">
							<Badge variant={u.status === 'active' ? 'green' : 'red'}>{u.status}</Badge>
						</div>
					</div>
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Workspaces</span>
						<span class="mt-0.5 block font-heading text-sm font-extrabold text-ink">
							{u.workspace_count}
						</span>
					</div>
					<div>
						<span class="block font-mono text-[10px] font-bold uppercase text-ink/65">Created</span>
						<span class="mt-0.5 block font-mono font-bold text-ink">
							{formatDate(u.created_at)}
						</span>
					</div>
				</div>

				{#if u.status === 'suspended' && u.suspended_reason}
					<div class="rounded-md border-2 border-ink bg-coral/20 px-3.5 py-2.5 text-xs">
						<span class="font-mono font-extrabold uppercase text-ink">Suspension Reason:</span>
						<span class="ml-1 font-medium text-ink/90">{u.suspended_reason}</span>
					</div>
				{/if}

				<div class="flex items-center justify-between gap-2 pt-2 border-t-2 border-ink/15">
					{#if canGlobalAction}
						{#if u.status === 'active'}
							<Button
								variant="danger-outline"
								size="sm"
								onclick={() =>
									openSuspendModal(
										'user',
										u.id,
										u.name,
										u.email,
										`${u.workspace_count} workspace membership(s)`
									)}
							>
								<Ban class="h-3.5 w-3.5" />
								<span>Global Suspend</span>
							</Button>
						{:else}
							<Button
								variant="secondary"
								size="sm"
								disabled={busyId === u.id}
								onclick={() => handleActivateUser(u)}
							>
								<CheckCircle2 class="h-3.5 w-3.5" />
								<span>Activate</span>
							</Button>
						{/if}
					{:else}
						<span class="font-mono text-[11px] font-bold text-ink/60">
							{isSelf ? 'Current authenticated Super Admin' : 'Protected Super Admin account'}
						</span>
					{/if}
					<Button variant="secondary" size="sm" onclick={() => (detailUser = null)}>
						Close
					</Button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Plan & Overrides Modal -->
	{#if editPlanTarget}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="edit-plan-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div>
						<h2 id="edit-plan-dialog-title" class="font-heading text-lg font-extrabold text-ink">
							Plan & Quota Overrides
						</h2>
						<p class="font-mono text-xs text-ink/70">{editPlanTarget.name} ({editPlanTarget.slug})</p>
					</div>
					<button
						type="button"
						onclick={closeEditPlanModal}
						aria-label="Close plan dialog"
						class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<form onsubmit={handleSavePlan} class="space-y-4">
					<div>
						<label for="platform-ws-plan" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
							Plan Tier
						</label>
						<select
							id="platform-ws-plan"
							bind:value={editPlanValue}
							class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 font-heading text-sm font-bold text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						>
							<option value="free">Free</option>
							<option value="pro">Pro</option>
							<option value="enterprise">Enterprise</option>
						</select>
					</div>

					<div class="grid grid-cols-3 gap-3">
						<div>
							<label for="override-users" class="mb-1 block font-mono text-[11px] font-bold uppercase text-ink">
								Max Users
							</label>
							<input
								id="override-users"
								type="number"
								min="1"
								placeholder="Default"
								bind:value={editMaxUsers}
								class="w-full rounded-md border-2 border-ink bg-cream px-2.5 py-1.5 font-mono text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
						<div>
							<label for="override-products" class="mb-1 block font-mono text-[11px] font-bold uppercase text-ink">
								Max Products
							</label>
							<input
								id="override-products"
								type="number"
								min="1"
								placeholder="Default"
								bind:value={editMaxProducts}
								class="w-full rounded-md border-2 border-ink bg-cream px-2.5 py-1.5 font-mono text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
						<div>
							<label for="override-licenses" class="mb-1 block font-mono text-[11px] font-bold uppercase text-ink">
								Max Licenses
							</label>
							<input
								id="override-licenses"
								type="number"
								min="1"
								placeholder="Default"
								bind:value={editMaxLicenses}
								class="w-full rounded-md border-2 border-ink bg-cream px-2.5 py-1.5 font-mono text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
					</div>

					<div>
						<label for="override-note" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
							Override Note (optional)
						</label>
						<input
							id="override-note"
							type="text"
							maxlength={500}
							bind:value={editOverrideNote}
							placeholder="e.g. Partner / Beta deal"
							class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						/>
					</div>

					{#if editPlanError}
						<p role="alert" class="text-xs font-semibold text-red-600">{editPlanError}</p>
					{/if}

					<div class="flex justify-end gap-3">
						<Button type="button" variant="secondary" onclick={closeEditPlanModal} disabled={savingPlan}>
							Cancel
						</Button>
						<Button type="submit" variant="yellow" disabled={savingPlan}>
							{savingPlan ? 'Saving...' : 'Save Plan'}
						</Button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- Suspend Confirmation Dialog -->
	{#if suspendTarget}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="platform-suspend-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div class="flex items-start gap-2.5">
						<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-coral text-ink shadow-brutal-xs">
							<TriangleAlert class="h-5 w-5" aria-hidden="true" />
						</div>
						<div>
							<h2 id="platform-suspend-dialog-title" class="font-heading text-lg font-extrabold text-ink">
								{suspendTarget.kind === 'workspace' ? 'Suspend Workspace' : 'Global Suspend Account'}
							</h2>
							<p class="text-xs text-ink/70">Explicit administrative confirmation required</p>
						</div>
					</div>
					<button
						type="button"
						onclick={closeSuspendModal}
						aria-label="Close suspend dialog"
						class="flex h-8 w-8 shrink-0 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<div class="rounded-md border-2 border-ink bg-cream px-3.5 py-2.5">
					<p class="font-heading text-sm font-extrabold text-ink">{suspendTarget.label}</p>
					<p class="font-mono text-xs text-ink/75">{suspendTarget.sublabel}</p>
					<p class="mt-1 font-mono text-[11px] font-bold text-ink/65">{suspendTarget.meta}</p>
				</div>

				<div class="rounded-md border-2 border-ink bg-coral/15 px-3.5 py-2.5 text-xs text-ink/90 space-y-2">
					{#if suspendTarget.kind === 'workspace'}
						<p>
							<strong>Consequences:</strong> Suspending this workspace immediately blocks all members from accessing workspace data and rejects workspace-scoped API calls until reactivated.
						</p>
					{:else}
						<p>
							<strong>Cross-Workspace Impact:</strong> Global suspension revokes all active sessions and blocks sign-in across every associated Workspace while preserving individual membership records.
						</p>
						{#if loadingImpact}
							<p class="font-mono text-[11px] font-bold text-ink/70">
								Evaluating workspace membership invariants...
							</p>
						{:else if impactError}
							<p class="font-semibold text-red-600">{impactError}</p>
						{:else if suspendImpact}
							{#if suspendImpact.workspaces.length === 0}
								<p class="font-mono text-[11px] text-ink/75">
									This account currently holds no active workspace memberships.
								</p>
							{:else}
								<ul class="space-y-1.5 pt-1">
									{#each suspendImpact.workspaces as w (w.workspace_id)}
										<li class="rounded border border-ink bg-surface px-2.5 py-1.5 text-[11px]">
											<div class="flex items-center justify-between gap-2">
												<span class="font-heading font-bold text-ink">{w.workspace_name}</span>
												<span class="font-mono uppercase text-ink/70">{w.role}</span>
											</div>
											{#if w.blocked}
												<p class="mt-0.5 font-semibold text-red-600">{w.block_reason}</p>
											{:else}
												<p class="mt-0.5 font-mono text-ink/70">
													Remaining after suspend: {w.next_active_admins} admin(s), {w.next_active_members} active member(s)
												</p>
											{/if}
										</li>
									{/each}
								</ul>
							{/if}
						{/if}
					{/if}
				</div>

				<form onsubmit={handleConfirmSuspend} class="space-y-4">
					<div>
						<label for="platform-suspend-reason" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
							Reason (required, min 3 characters)
						</label>
						<textarea
							id="platform-suspend-reason"
							rows={3}
							required
							minlength={3}
							disabled={suspendTarget.kind === 'user' && suspendImpact !== null && !suspendImpact.allowed}
							bind:value={suspendReason}
							placeholder="e.g. Terms of service violation or billing review"
							class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt disabled:cursor-not-allowed disabled:opacity-60"
						></textarea>
						{#if suspendError}
							<p role="alert" class="mt-1 text-xs font-semibold text-red-600">{suspendError}</p>
						{/if}
					</div>

					<div class="flex justify-end gap-3">
						<Button type="button" variant="secondary" onclick={closeSuspendModal} disabled={suspending}>
							Cancel
						</Button>
						<Button
							type="submit"
							variant="coral"
							disabled={suspending ||
								(suspendTarget.kind === 'user' &&
									(loadingImpact || (suspendImpact !== null && !suspendImpact.allowed)))}
						>
							{suspending
								? 'Suspending...'
								: suspendTarget.kind === 'workspace'
									? 'Suspend Workspace'
									: 'Confirm Global Suspend'}
						</Button>
					</div>
				</form>
			</div>
		</div>
	{/if}
</AppShell>
