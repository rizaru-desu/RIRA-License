<script lang="ts">
	import { Users, UserPlus, ShieldAlert, CheckCircle2, Ban, X, ArrowLeft, Trash2, LogOut } from '@lucide/svelte';
	import {
		ApiError,
		authApi,
		resolveAvatarUrl,
		type AdminUserDTO,
		type UserRole
	} from '$lib/api/auth';
	import { authStore, FREE_MAX_USERS } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AppShell, { computeInitials } from '$lib/components/layout/AppShell.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge, { type BadgeVariant } from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	type AssignableRole = 'admin' | 'member' | 'viewer';
	const assignableRoles: AssignableRole[] = ['admin', 'member', 'viewer'];

	let users = $state<AdminUserDTO[]>([]);
	let loading = $state(true);
	let errorMsg = $state('');
	let successMsg = $state('');

	// Invite user form state
	let inviteName = $state('');
	let inviteEmail = $state('');
	let inviteRole = $state<AssignableRole>('member');
	let inviting = $state(false);

	// Role change modal state
	let roleChangeTarget = $state<{
		user: AdminUserDTO;
		nextRole: AssignableRole;
	} | null>(null);
	let roleChangeError = $state('');
	let updatingRoleUserId = $state<string | null>(null);

	// Activate state
	let activatingUserId = $state<string | null>(null);

	// Suspend modal state
	let suspendTarget = $state<AdminUserDTO | null>(null);
	let suspendReason = $state('');
	let suspendReasonError = $state('');
	let suspending = $state(false);

	// Remove member / cancel invite modal state
	let removeTarget = $state<AdminUserDTO | null>(null);
	let removeReason = $state('');
	let removeError = $state('');
	let removing = $state(false);

	// Blocked operation modal state (BR-01 / BR-02)
	type BlockKind = 'last_active_admin' | 'last_active_member';
	let blockedDialog = $state<{
		kind: BlockKind;
		user: AdminUserDTO;
	} | null>(null);

	// Leave workspace modal state
	let leaveOpen = $state(false);
	let leaving = $state(false);

	const canInvite = $derived(authStore.hasPermission('users:invite'));
	const canManageRoles = $derived(authStore.hasPermission('roles:manage'));
	const canSuspend = $derived(authStore.hasPermission('users:suspend'));
	const canRemove = $derived(authStore.hasPermission('users:remove'));

	const isSuperadmin = $derived(Boolean(authStore.user?.is_superadmin));
	const isFreePlan = $derived(authStore.plan === 'free');
	const planSeatLimit = $derived(
		isFreePlan
			? authStore.maxUsers > 0
				? authStore.maxUsers
				: FREE_MAX_USERS
			: authStore.maxUsers
	);
	const isInviteLockedByPlan = $derived(
		!isSuperadmin && (isFreePlan || (planSeatLimit > 0 && users.length >= planSeatLimit))
	);
	const seatsAboveFreeLimit = $derived(
		isFreePlan && !loading && planSeatLimit > 0 && users.length > planSeatLimit
			? users.length - planSeatLimit
			: 0
	);

	function isActiveUser(u: AdminUserDTO): boolean {
		return u.status === 'active' && u.global_status !== 'suspended';
	}

	function isAdminRole(role: UserRole): boolean {
		return role === 'owner' || role === 'admin';
	}

	const activeMemberCount = $derived(users.filter(isActiveUser).length);
	const activeAdminCount = $derived(
		users.filter((u) => isActiveUser(u) && isAdminRole(u.role)).length
	);
	const activeOwnerCount = $derived(
		users.filter((u) => isActiveUser(u) && u.role === 'owner').length
	);

	function blockReasonFor(target: AdminUserDTO, nextRole?: UserRole): BlockKind | null {
		if (!isActiveUser(target)) return null;
		if (nextRole !== undefined) {
			if (target.role === 'owner' && nextRole !== 'owner' && activeOwnerCount <= 1) {
				return 'last_active_admin';
			}
			if (isAdminRole(target.role) && !isAdminRole(nextRole) && activeAdminCount <= 1) {
				return 'last_active_admin';
			}
			return null;
		}
		if (
			(target.role === 'owner' && activeOwnerCount <= 1) ||
			(isAdminRole(target.role) && activeAdminCount <= 1)
		) {
			return 'last_active_admin';
		}
		if (activeMemberCount <= 1) {
			return 'last_active_member';
		}
		return null;
	}

	$effect(() => {
		const activeWorkspaceId = authStore.workspace?.id ?? '';
		void activeWorkspaceId;
		if (authStore.canViewUsers) {
			void loadUsers();
		} else {
			loading = false;
		}
	});

	async function loadUsers() {
		loading = true;
		errorMsg = '';
		try {
			const res = await authApi.listUsers();
			users = res.users ?? [];
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to load users.';
		} finally {
			loading = false;
		}
	}

	function roleBadgeVariant(role: UserRole): BadgeVariant {
		switch (role) {
			case 'owner':
				return 'yellow';
			case 'admin':
				return 'cobalt';
			case 'member':
				return 'cream';
			case 'viewer':
				return 'grey';
		}
	}

	function canChangeRoleFor(target: AdminUserDTO): boolean {
		const me = authStore.user;
		if (!me || !canManageRoles) return false;
		if (target.status === 'pending') return false;
		if (target.id === me.id) return false;
		if (target.is_superadmin) return false;
		if (target.role === 'owner') return false;
		return target.role === 'admin' || target.role === 'member' || target.role === 'viewer';
	}

	function canRemoveTarget(target: AdminUserDTO): boolean {
		const me = authStore.user;
		if (!me || !canRemove) return false;
		if (target.id === me.id) return false;
		if (target.is_superadmin) return false;
		if (me.is_superadmin || me.role === 'owner') return true;
		return target.role === 'member' || target.role === 'viewer';
	}

	function canSuspendTarget(target: AdminUserDTO): boolean {
		const me = authStore.user;
		if (!me || !canSuspend) return false;
		if (target.status !== 'active') return false;
		if (target.id === me.id) return false;
		if (target.is_superadmin) return false;
		if (me.is_superadmin || me.role === 'owner') return true;
		return target.role === 'member' || target.role === 'viewer';
	}

	function canActivateTarget(target: AdminUserDTO): boolean {
		const me = authStore.user;
		if (!me || !canSuspend) return false;
		if (target.status !== 'suspended') return false;
		if (target.id === me.id) return false;
		if (target.is_superadmin) return false;
		if (me.is_superadmin || me.role === 'owner') return true;
		return target.role === 'member' || target.role === 'viewer';
	}

	function actionLockReason(target: AdminUserDTO): string {
		const me = authStore.user;
		if (me && target.id === me.id) return 'You cannot modify or suspend your own account';
		if (target.is_superadmin) return 'Super Admin cannot be suspended or have their role changed';
		if (target.role === 'owner') return 'Only another Owner or Super Admin can manage an Owner';
		return 'You do not have permission to manage this user';
	}

	async function handleInviteUser(e: SubmitEvent) {
		e.preventDefault();
		if (isInviteLockedByPlan) return;
		errorMsg = '';
		successMsg = '';

		const trimmedName = inviteName.trim();
		const trimmedEmail = inviteEmail.trim();
		if (trimmedName.length > 0 && trimmedName.length < 2) {
			errorMsg = 'Name must be at least 2 characters (or leave blank for existing users).';
			return;
		}
		if (!trimmedEmail) {
			errorMsg = 'Email is required.';
			return;
		}

		inviting = true;
		try {
			const res = await authApi.createUser({
				name: trimmedName || undefined,
				email: trimmedEmail,
				role: inviteRole
			});
			inviteName = '';
			inviteEmail = '';
			inviteRole = 'member';
			await loadUsers();
			successMsg =
				res.user.status === 'pending'
					? `Invitation sent to existing user ${res.user.email} as ${res.user.role} (pending acceptance).`
					: `Invited ${res.user.email} as ${res.user.role}. Set-password link sent.`;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to invite user.';
		} finally {
			inviting = false;
		}
	}

	function requestRoleChange(target: AdminUserDTO, nextRole: string) {
		if (nextRole !== 'admin' && nextRole !== 'member' && nextRole !== 'viewer') {
			return;
		}
		if (nextRole === target.role) return;
		errorMsg = '';
		successMsg = '';
		const blocked = blockReasonFor(target, nextRole);
		if (blocked) {
			blockedDialog = { kind: blocked, user: target };
			return;
		}
		roleChangeError = '';
		roleChangeTarget = { user: target, nextRole };
	}

	function closeRoleChangeModal() {
		if (updatingRoleUserId !== null) return;
		roleChangeTarget = null;
		roleChangeError = '';
	}

	async function handleConfirmRoleChange(e: SubmitEvent) {
		e.preventDefault();
		if (!roleChangeTarget || updatingRoleUserId !== null) return;

		const { user: target, nextRole } = roleChangeTarget;
		roleChangeError = '';
		errorMsg = '';
		successMsg = '';
		updatingRoleUserId = target.id;
		try {
			const res = await authApi.updateUserRole(target.id, nextRole);
			roleChangeTarget = null;
			await loadUsers();
			successMsg = `Updated role for ${res.user.name} to ${res.user.role}.`;
		} catch (err) {
			if (err instanceof ApiError && (err.code === 'last_active_admin' || err.code === 'last_active_member')) {
				roleChangeTarget = null;
				blockedDialog = { kind: err.code, user: target };
				await loadUsers();
			} else {
				roleChangeError = err instanceof ApiError ? err.message : 'Failed to update user role.';
			}
		} finally {
			updatingRoleUserId = null;
		}
	}

	function openSuspendModal(target: AdminUserDTO) {
		errorMsg = '';
		successMsg = '';
		const blocked = blockReasonFor(target);
		if (blocked) {
			blockedDialog = { kind: blocked, user: target };
			return;
		}
		suspendTarget = target;
		suspendReason = '';
		suspendReasonError = '';
	}

	function closeSuspendModal() {
		if (suspending) return;
		suspendTarget = null;
		suspendReason = '';
		suspendReasonError = '';
	}

	async function handleConfirmSuspend(e: SubmitEvent) {
		e.preventDefault();
		if (!suspendTarget || suspending) return;

		const trimmed = suspendReason.trim();
		if (trimmed.length < 3) {
			suspendReasonError = 'Reason must be at least 3 characters.';
			return;
		}

		suspendReasonError = '';
		errorMsg = '';
		successMsg = '';
		suspending = true;
		const target = suspendTarget;
		try {
			const res = await authApi.suspendUser(target.id, trimmed);
			suspendTarget = null;
			suspendReason = '';
			await loadUsers();
			successMsg = `Suspended ${res.user.name} in ${authStore.workspace?.name ?? 'this workspace'}.`;
		} catch (err) {
			if (err instanceof ApiError && (err.code === 'last_active_admin' || err.code === 'last_active_member')) {
				suspendTarget = null;
				suspendReason = '';
				blockedDialog = { kind: err.code, user: target };
				await loadUsers();
			} else {
				suspendReasonError = err instanceof ApiError ? err.message : 'Failed to suspend user.';
			}
		} finally {
			suspending = false;
		}
	}

	async function handleActivateUser(target: AdminUserDTO) {
		if (activatingUserId !== null) return;
		errorMsg = '';
		successMsg = '';
		activatingUserId = target.id;
		try {
			const res = await authApi.activateUser(target.id);
			await loadUsers();
			successMsg = `Reactivated ${res.user.name} in ${authStore.workspace?.name ?? 'this workspace'}.`;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to activate user.';
		} finally {
			activatingUserId = null;
		}
	}

	function openRemoveModal(target: AdminUserDTO) {
		errorMsg = '';
		successMsg = '';
		removeError = '';
		removeReason = '';
		if (target.status !== 'pending') {
			const blocked = blockReasonFor(target);
			if (blocked) {
				blockedDialog = { kind: blocked, user: target };
				return;
			}
		}
		removeTarget = target;
	}

	function closeRemoveModal() {
		if (removing) return;
		removeTarget = null;
		removeReason = '';
		removeError = '';
	}

	async function handleConfirmRemove() {
		if (!removeTarget || removing) return;
		errorMsg = '';
		successMsg = '';
		removeError = '';
		removing = true;
		const target = removeTarget;
		try {
			await authApi.removeUser(target.id, removeReason);
			removeTarget = null;
			removeReason = '';
			await loadUsers();
			await authStore.refreshUser();
			successMsg =
				target.status === 'pending'
					? `Canceled pending invitation for ${target.email}.`
					: `Removed ${target.name} from this workspace.`;
		} catch (err) {
			if (err instanceof ApiError && (err.code === 'last_active_admin' || err.code === 'last_active_member')) {
				removeTarget = null;
				removeReason = '';
				blockedDialog = { kind: err.code, user: target };
				await loadUsers();
			} else {
				errorMsg = err instanceof ApiError ? err.message : 'Failed to remove user.';
				removeTarget = null;
				removeReason = '';
			}
		} finally {
			removing = false;
		}
	}

	function handleManageMembersFromBlocked() {
		blockedDialog = null;
		const el = document.getElementById('invite-user-email');
		if (el) {
			el.scrollIntoView({ behavior: 'smooth', block: 'center' });
			el.focus();
		}
	}

	async function handleLeaveWorkspace() {
		if (!authStore.workspace) return;
		errorMsg = '';
		successMsg = '';
		leaving = true;
		try {
			await authStore.leaveWorkspace(authStore.workspace.id);
			leaveOpen = false;
			router.navigate('/dashboard', true);
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to leave workspace.';
			leaveOpen = false;
		} finally {
			leaving = false;
		}
	}

	function handleWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape') return;
		if (blockedDialog) {
			blockedDialog = null;
		} else if (leaveOpen && !leaving) {
			leaveOpen = false;
		} else if (removeTarget) {
			closeRemoveModal();
		} else if (suspendTarget) {
			closeSuspendModal();
		} else if (roleChangeTarget) {
			closeRoleChangeModal();
		}
	}

	function formatDate(iso: string): string {
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
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<AppShell>
	{#if authStore.user && authStore.canViewUsers}
		<div class="space-y-6">
			{#if isSuperadmin && authStore.workspace}
				<div class="flex flex-wrap items-center justify-between gap-3 rounded-md border-2 border-ink bg-yellow/35 px-4 py-2.5 shadow-brutal-xs">
					<span class="font-mono text-xs font-bold text-ink">
						Super Admin inspecting workspace: <span class="uppercase">{authStore.workspace.name}</span>
					</span>
					<Button
						variant="secondary"
						size="sm"
						onclick={async () => {
							await authStore.exitWorkspace();
							router.navigate('/platform');
						}}
					>
						Exit to Platform
					</Button>
				</div>
			{/if}

			<div class="flex flex-wrap items-start justify-between gap-3">
				<div class="space-y-2">
					<button
						type="button"
						onclick={() => router.navigate('/dashboard')}
						class="inline-flex items-center gap-1.5 font-mono text-xs font-bold uppercase tracking-wider text-ink/70 cursor-pointer hover:text-ink"
					>
						<ArrowLeft class="h-3.5 w-3.5" aria-hidden="true" />
						<span>Back to Overview</span>
					</button>
					<div>
						<h1 class="font-heading text-3xl font-extrabold text-ink">Workspace Users</h1>
						<p class="mt-1 text-sm text-ink/75">
							{#if authStore.workspace}
								Manage team members, role assignments, and account status for
								<span class="font-mono font-bold text-ink">{authStore.workspace.name}</span>.
							{:else}
								Manage workspace team members, role assignments, and account status.
							{/if}
						</p>
					</div>
				</div>
				<div class="flex flex-wrap items-center gap-2.5">
					{#if !isSuperadmin && authStore.workspace && authStore.user.role !== 'owner' && authStore.workspaces.length > 1}
						<Button variant="danger-outline" size="sm" onclick={() => (leaveOpen = true)}>
							<LogOut class="h-3.5 w-3.5" />
							<span>Leave Workspace</span>
						</Button>
					{/if}
					{#if isFreePlan}
						<div data-seat-indicator>
							<Badge variant="yellow">
								Free Plan · {loading ? '…' : users.length}/{planSeatLimit} Seats
							</Badge>
						</div>
					{:else}
						<div data-seat-indicator>
							<Badge variant="cream">
								{authStore.plan} Plan · {loading ? '…' : users.length} {users.length === 1 ? 'User' : 'Users'}
							</Badge>
						</div>
					{/if}
				</div>
			</div>

			{#if errorMsg}
				<div
					role="alert"
					class="rounded-md border-2 border-ink bg-coral/25 px-4 py-3 text-sm font-semibold text-ink"
				>
					{errorMsg}
				</div>
			{/if}

			{#if successMsg}
				<div
					role="status"
					class="rounded-md border-2 border-ink bg-yellow/40 px-4 py-3 text-sm font-semibold text-ink"
				>
					{successMsg}
				</div>
			{/if}

			{#if canInvite}
				<div>
					<Card>
						<div class="flex items-center gap-2.5">
							<UserPlus class="h-5 w-5 text-cobalt" />
							<h2 class="font-heading text-xl font-extrabold text-ink">Invite User</h2>
						</div>
						<p class="mt-1 text-sm text-ink/75">
							Invite existing users by email (they will accept from their dashboard) or onboard new members with a set-password link.
						</p>

						{#if isInviteLockedByPlan}
							<div
								role="status"
								data-invite-plan-notice
								class="mt-4 rounded-md border-2 border-ink bg-yellow/35 px-4 py-3 text-sm font-semibold text-ink shadow-brutal-xs"
							>
								Invite is available on Pro — upgrade to add team members.
							</div>
						{/if}

						<form
							onsubmit={handleInviteUser}
							class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_180px_auto] lg:items-end"
						>
							<div>
								<label
									for="invite-user-name"
									class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
								>
									Full Name <span class="font-mono text-[10px] font-normal text-ink/60">(new users)</span>
								</label>
								<input
									id="invite-user-name"
									type="text"
									maxlength={100}
									disabled={isInviteLockedByPlan || inviting}
									bind:value={inviteName}
									placeholder="Optional if user already exists"
									class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt disabled:cursor-not-allowed disabled:opacity-60"
								/>
							</div>

							<div>
								<label
									for="invite-user-email"
									class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
								>
									Email Address
								</label>
								<input
									id="invite-user-email"
									type="email"
									required
									disabled={isInviteLockedByPlan || inviting}
									bind:value={inviteEmail}
									placeholder="grace@company.com"
									class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt disabled:cursor-not-allowed disabled:opacity-60"
								/>
							</div>

							<div>
								<label
									for="invite-user-role"
									class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
								>
									Role
								</label>
								<select
									id="invite-user-role"
									disabled={isInviteLockedByPlan || inviting}
									bind:value={inviteRole}
									class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 font-heading text-sm font-bold text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt disabled:cursor-not-allowed disabled:opacity-60"
								>
									<option value="admin">Admin</option>
									<option value="member">Member</option>
									<option value="viewer">Viewer</option>
								</select>
							</div>

							<div>
								<Button type="submit" disabled={isInviteLockedByPlan || inviting} size="md">
									<UserPlus class="h-4 w-4" />
									<span>{inviting ? 'Inviting...' : 'Invite User'}</span>
								</Button>
							</div>
						</form>
					</Card>

					{#if seatsAboveFreeLimit > 0}
						<p
							data-superadmin-seat-note
							class="mt-2 px-1 font-mono text-xs font-semibold text-ink/75"
						>
							{seatsAboveFreeLimit} {seatsAboveFreeLimit === 1 ? 'seat' : 'seats'} above the free limit — Super Admin override.
						</p>
					{/if}
				</div>
			{/if}

			<Card padding="sm">
				<div class="flex flex-wrap items-center gap-2.5 px-2 pb-4">
					<Users class="h-5 w-5 text-cobalt" />
					<h2 class="font-heading text-xl font-extrabold text-ink">Members</h2>
				</div>

				{#if loading}
					<div class="px-2 py-8 text-center font-mono text-sm font-bold text-ink/70">
						Loading users...
					</div>
				{:else if users.length === 0}
					<div class="px-2 py-8 text-center font-mono text-sm font-bold text-ink/70">
						No users found.
					</div>
				{:else}
					<div class="overflow-x-auto rounded-md border-2 border-ink bg-surface">
						<table class="w-full border-collapse text-left text-sm">
							<thead>
								<tr class="border-b-2 border-ink bg-cream font-mono text-xs font-extrabold uppercase tracking-wider text-ink">
									<th class="px-4 py-3">User</th>
									<th class="px-4 py-3">Role</th>
									<th class="px-4 py-3">Status</th>
									<th class="px-4 py-3">Joined</th>
									<th class="px-4 py-3 text-right">Actions</th>
								</tr>
							</thead>
							<tbody class="divide-y-2 divide-ink/15">
								{#each users as u (u.id)}
									{@const avatarUrl = resolveAvatarUrl(u.avatar_url)}
									{@const initials = computeInitials(u.name, u.email)}
									{@const isSelf = u.id === authStore.user.id}
									{@const showRoleSelect = canChangeRoleFor(u)}
									{@const showSuspend = canSuspendTarget(u)}
									{@const showActivate = canActivateTarget(u)}
									{@const showRemove = canRemoveTarget(u)}
									{@const suspendBlock = blockReasonFor(u)}
									<tr
										data-user-row={u.email}
										class={u.status === 'suspended'
											? 'opacity-60 transition-colors hover:bg-cream/50'
											: 'transition-colors hover:bg-cream/50'}
									>
										<td class="px-4 py-3.5">
											<div class="flex items-center gap-3">
												<div
													class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-md border-2 border-ink bg-yellow font-heading text-xs font-extrabold uppercase text-ink shadow-brutal-xs"
												>
													{#if avatarUrl}
														<img
															src={avatarUrl}
															alt={u.name}
															class="h-full w-full object-cover"
														/>
													{:else}
														<span>{initials}</span>
													{/if}
												</div>
												<div class="min-w-0">
													<div class="flex flex-wrap items-center gap-1.5">
														<span class="font-heading font-extrabold text-ink truncate">
															{u.name}
														</span>
														{#if isSelf}
															<span
																class="rounded border border-ink bg-cream px-1.5 py-0.5 font-mono text-[10px] font-bold uppercase text-ink"
															>
																You
															</span>
														{/if}
													</div>
													<p class="font-mono text-xs text-ink/70 truncate">{u.email}</p>
												</div>
											</div>
										</td>

										<td class="px-4 py-3.5">
											<div class="flex flex-wrap items-center gap-1.5">
												<Badge variant={roleBadgeVariant(u.role)}>{u.role}</Badge>
												{#if u.is_superadmin}
													<Badge variant="cobalt">Super Admin</Badge>
												{/if}
											</div>
										</td>

										<td class="px-4 py-3.5">
											<div class="space-y-1">
												<div class="flex flex-wrap items-center gap-1.5">
													<Badge
														variant={u.status === 'active'
															? 'green'
															: u.status === 'pending'
																? 'yellow'
																: 'red'}
													>
														{u.status}
													</Badge>
													{#if u.global_status === 'suspended'}
														<Badge variant="red">Global Suspended</Badge>
													{/if}
												</div>
												{#if u.status === 'suspended' && u.suspended_reason}
													<p class="font-mono text-xs text-ink/75">
														Reason: {u.suspended_reason}
													</p>
												{/if}
											</div>
										</td>

										<td class="px-4 py-3.5 font-mono text-xs font-semibold text-ink/75 whitespace-nowrap">
											{formatDate(u.joined_at ?? u.created_at)}
										</td>

										<td class="px-4 py-3.5 text-right whitespace-nowrap">
											{#if showRoleSelect || showSuspend || showActivate || showRemove}
												<div class="inline-flex items-center justify-end gap-2">
													{#if showRoleSelect}
														<select
															aria-label={`Change role for ${u.name}`}
															value={u.role}
															disabled={updatingRoleUserId === u.id}
															onchange={(e) => {
																const selectEl = e.currentTarget as HTMLSelectElement;
																const nextRole = selectEl.value;
																selectEl.value = u.role;
																requestRoleChange(u, nextRole);
															}}
															class="h-9 rounded-md border-2 border-ink bg-cream px-2.5 font-heading text-xs font-bold uppercase text-ink shadow-brutal-xs cursor-pointer focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt disabled:cursor-not-allowed disabled:opacity-60"
														>
															{#each assignableRoles as roleOpt (roleOpt)}
																{@const optBlocked = blockReasonFor(u, roleOpt) !== null}
																<option
																	value={roleOpt}
																	selected={u.role === roleOpt}
																	disabled={optBlocked}
																>
																	{roleOpt}{optBlocked ? ' (Blocked)' : ''}
																</option>
															{/each}
														</select>
													{/if}

													{#if showSuspend}
														<Button
															variant="danger-outline"
															size="sm"
															disabled={suspendBlock !== null || suspending}
															title={suspendBlock === 'last_active_admin'
																? 'Cannot suspend the last active Admin in this workspace'
																: suspendBlock === 'last_active_member'
																	? 'Cannot suspend the last active member in this workspace'
																	: undefined}
															onclick={() => openSuspendModal(u)}
														>
															<Ban class="h-3.5 w-3.5" />
															<span>Suspend</span>
														</Button>
													{:else if showActivate}
														<Button
															variant="secondary"
															size="sm"
															disabled={activatingUserId === u.id}
															onclick={() => handleActivateUser(u)}
														>
															<CheckCircle2 class="h-3.5 w-3.5" />
															<span>
																{activatingUserId === u.id ? 'Activating...' : 'Activate'}
															</span>
														</Button>
													{/if}

													{#if showRemove}
														<Button
															variant="danger-outline"
															size="sm"
															disabled={removing}
															onclick={() => openRemoveModal(u)}
														>
															<Trash2 class="h-3.5 w-3.5" />
															<span>{u.status === 'pending' ? 'Cancel' : 'REMOVE FROM WORKSPACE'}</span>
														</Button>
													{/if}
												</div>
											{:else}
												<span
													title={actionLockReason(u)}
													aria-label={actionLockReason(u)}
													class="inline-block cursor-help font-mono text-xs text-ink/55 underline decoration-dotted underline-offset-4"
												>
													—
												</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</Card>
		</div>
	{/if}

	{#if roleChangeTarget}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="role-change-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div
				class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4"
			>
				<div class="flex items-start justify-between gap-3">
					<div>
						<h2
							id="role-change-dialog-title"
							class="font-heading text-lg font-extrabold text-ink"
						>
							Change role for {roleChangeTarget.user.name}
						</h2>
						<p class="font-mono text-xs text-ink/70">{roleChangeTarget.user.email}</p>
					</div>
					<button
						type="button"
						onclick={closeRoleChangeModal}
						aria-label="Close role change dialog"
						class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<p class="text-sm text-ink/85">
					Change role from
					<span class="font-mono font-bold uppercase">{roleChangeTarget.user.role}</span>
					to
					<span class="font-mono font-bold uppercase">{roleChangeTarget.nextRole}</span>?
				</p>

				{#if roleChangeError}
					<p role="alert" class="text-xs font-semibold text-red-600">
						{roleChangeError}
					</p>
				{/if}

				<form onsubmit={handleConfirmRoleChange} class="flex justify-end gap-3">
					<Button
						type="button"
						variant="secondary"
						onclick={closeRoleChangeModal}
						disabled={updatingRoleUserId !== null}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						variant="yellow"
						disabled={updatingRoleUserId !== null}
					>
						{updatingRoleUserId !== null ? 'Updating...' : 'Confirm Role Change'}
					</Button>
				</form>
			</div>
		</div>
	{/if}

	{#if suspendTarget}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="suspend-user-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div
				class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4"
			>
				<div class="flex items-start justify-between gap-3">
					<div class="flex items-center gap-2.5">
						<div
							class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-coral text-ink shadow-brutal-xs"
						>
							<ShieldAlert class="h-5 w-5" />
						</div>
						<div>
							<h2
								id="suspend-user-dialog-title"
								class="font-heading text-lg font-extrabold text-ink"
							>
								Suspend {suspendTarget.name}
							</h2>
							<p class="font-mono text-xs text-ink/70">{suspendTarget.email}</p>
						</div>
					</div>
					<button
						type="button"
						onclick={closeSuspendModal}
						aria-label="Close suspend dialog"
						class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<p class="text-sm text-ink/85">
					Suspending this user restricts their access only within
					<span class="font-bold">{authStore.workspace?.name ?? 'this Workspace'}</span>. Their global
					account and memberships in other Workspaces will remain unchanged.
				</p>

				<form onsubmit={handleConfirmSuspend} class="space-y-4">
					<div>
						<label
							for="suspend-reason-input"
							class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
						>
							Suspension Reason (required, min 3 characters)
						</label>
						<textarea
							id="suspend-reason-input"
							rows={3}
							required
							minlength={3}
							bind:value={suspendReason}
							placeholder="e.g. Policy violation or compromised credentials"
							class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						></textarea>
						{#if suspendReasonError}
							<p role="alert" class="mt-1 text-xs font-semibold text-coral-hover">
								{suspendReasonError}
							</p>
						{/if}
					</div>

					<div class="flex justify-end gap-3">
						<Button
							type="button"
							variant="secondary"
							onclick={closeSuspendModal}
							disabled={suspending}
						>
							Cancel
						</Button>
						<Button type="submit" variant="coral" disabled={suspending}>
							{suspending ? 'Suspending...' : 'Confirm Suspend'}
						</Button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	{#if removeTarget}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="remove-member-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div class="flex items-start gap-2.5">
						<div
							class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-coral text-ink shadow-brutal-xs"
						>
							<Trash2 class="h-4 w-4" />
						</div>
						<div>
							<h2 id="remove-member-dialog-title" class="font-heading text-lg font-extrabold text-ink">
								{removeTarget.status === 'pending' ? 'Cancel Invitation' : 'Remove User from Workspace?'}
							</h2>
							<p class="font-mono text-xs text-ink/70">{removeTarget.name} · {removeTarget.email}</p>
						</div>
					</div>
					<button
						type="button"
						onclick={closeRemoveModal}
						aria-label="Close remove dialog"
						class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<p class="text-sm text-ink/85">
					{#if removeTarget.status === 'pending'}
						Cancel the pending workspace invitation for <span class="font-bold">{removeTarget.email}</span>?
					{:else}
						This user will lose access to this Workspace. Their global account and memberships in other Workspaces will remain unchanged.
					{/if}
				</p>

				{#if removeTarget.status !== 'pending'}
					<div>
						<label
							for="remove-reason-input"
							class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
						>
							Removal Reason (optional, recorded in audit log)
						</label>
						<input
							id="remove-reason-input"
							type="text"
							maxlength={500}
							bind:value={removeReason}
							placeholder="e.g. Project completed or role transition"
							class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						/>
					</div>
				{/if}

				{#if removeError}
					<p role="alert" class="text-xs font-semibold text-red-600">{removeError}</p>
				{/if}

				<div class="flex justify-end gap-3">
					<Button type="button" variant="secondary" onclick={closeRemoveModal} disabled={removing}>
						Cancel
					</Button>
					<Button type="button" variant="coral" onclick={handleConfirmRemove} disabled={removing}>
						{removing
							? 'Removing...'
							: removeTarget.status === 'pending'
								? 'Cancel Invite'
								: 'Remove from Workspace'}
					</Button>
				</div>
			</div>
		</div>
	{/if}

	{#if blockedDialog}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="blocked-operation-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div class="flex items-start gap-2.5">
						<div
							class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border-2 border-ink bg-yellow text-ink shadow-brutal-xs"
						>
							<ShieldAlert class="h-5 w-5" />
						</div>
						<div>
							<h2 id="blocked-operation-dialog-title" class="font-heading text-lg font-extrabold text-ink">
								{blockedDialog.kind === 'last_active_admin'
									? 'Cannot Remove Last Admin'
									: 'Cannot Remove Last Member'}
							</h2>
							<p class="font-mono text-xs text-ink/70">
								{blockedDialog.user.name} · {blockedDialog.user.email}
							</p>
						</div>
					</div>
					<button
						type="button"
						onclick={() => (blockedDialog = null)}
						aria-label="Close blocked operation dialog"
						class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<div class="rounded-md border-2 border-ink bg-coral/15 px-3.5 py-3 text-sm font-medium text-ink">
					{#if blockedDialog.kind === 'last_active_admin'}
						This Workspace must have at least one active Admin. Assign another Admin before removing or deactivating this user.
					{:else}
						A Workspace must have at least one active member. Add another member before removing this user.
					{/if}
				</div>

				<div class="flex justify-end gap-3">
					<Button type="button" variant="secondary" onclick={() => (blockedDialog = null)}>
						Cancel
					</Button>
					{#if canInvite}
						<Button type="button" variant="primary" onclick={handleManageMembersFromBlocked}>
							Manage Members
						</Button>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	{#if leaveOpen && authStore.workspace}
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="leave-workspace-dialog-title"
			class="fixed inset-0 z-50 flex items-center justify-center bg-ink/55 p-4"
		>
			<div class="w-full max-w-md rounded-md border-2 border-ink bg-surface p-6 shadow-brutal-lg space-y-4">
				<div class="flex items-start justify-between gap-3">
					<div>
						<h2 id="leave-workspace-dialog-title" class="font-heading text-lg font-extrabold text-ink">
							Leave {authStore.workspace.name}?
						</h2>
						<p class="font-mono text-xs text-ink/70">{authStore.workspace.slug}</p>
					</div>
					<button
						type="button"
						onclick={() => !leaving && (leaveOpen = false)}
						aria-label="Close leave workspace dialog"
						class="flex h-8 w-8 items-center justify-center rounded border-2 border-ink bg-cream text-ink shadow-brutal-xs cursor-pointer hover:bg-yellow"
					>
						<X class="h-4 w-4" />
					</button>
				</div>

				<p class="text-sm text-ink/85">
					You will lose access to <span class="font-bold">{authStore.workspace.name}</span> unless re-invited. Your own workspaces remain untouched.
				</p>

				<div class="flex justify-end gap-3">
					<Button type="button" variant="secondary" onclick={() => (leaveOpen = false)} disabled={leaving}>
						Cancel
					</Button>
					<Button type="button" variant="coral" onclick={handleLeaveWorkspace} disabled={leaving}>
						{leaving ? 'Leaving...' : 'Leave Workspace'}
					</Button>
				</div>
			</div>
		</div>
	{/if}
</AppShell>

