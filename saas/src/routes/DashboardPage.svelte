<script lang="ts">
	import { ShieldCheck, MailCheck, UserCheck, ArrowRight, Building2, Check, X, Users } from '@lucide/svelte';
	import { ApiError } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let inviteBusyId = $state<string | null>(null);
	let inviteError = $state('');

	async function handleAcceptInvite(workspaceId: string) {
		inviteError = '';
		inviteBusyId = workspaceId;
		try {
			await authStore.acceptInvite(workspaceId);
		} catch (err) {
			inviteError = err instanceof ApiError ? err.message : 'Failed to accept invitation.';
		} finally {
			inviteBusyId = null;
		}
	}

	async function handleDeclineInvite(workspaceId: string) {
		inviteError = '';
		inviteBusyId = workspaceId;
		try {
			await authStore.declineInvite(workspaceId);
		} catch (err) {
			inviteError = err instanceof ApiError ? err.message : 'Failed to decline invitation.';
		} finally {
			inviteBusyId = null;
		}
	}
</script>

<AppShell>
	{#if authStore.user}
		<div class="space-y-6">
			<div class="flex flex-wrap items-center justify-between gap-4">
				<div>
					<h1 class="font-heading text-3xl font-extrabold text-ink">
						Welcome, {authStore.user.name}
					</h1>
					<p class="mt-1 text-sm text-ink/75">
						Control plane session is active via httpOnly database cookie.
					</p>
				</div>
				<Button onclick={() => router.navigate('/settings/security')}>
					<ShieldCheck class="h-4 w-4" />
					<span>Security & 2FA Settings</span>
					<ArrowRight class="h-4 w-4" />
				</Button>
			</div>

			{#if authStore.pendingInvites.length > 0}
				<Card>
					<div class="flex flex-wrap items-center justify-between gap-2">
						<div class="flex items-center gap-2.5">
							<Building2 class="h-5 w-5 text-cobalt" />
							<h2 class="font-heading text-lg font-extrabold text-ink">
								Pending Workspace Invitations
							</h2>
						</div>
						<Badge variant="yellow">{authStore.pendingInvites.length} Pending</Badge>
					</div>

					{#if inviteError}
						<p role="alert" class="mt-3 text-xs font-semibold text-red-600">{inviteError}</p>
					{/if}

					<div class="mt-4 divide-y-2 divide-ink/15 rounded-md border-2 border-ink bg-cream/60">
						{#each authStore.pendingInvites as inv (inv.workspace_id)}
							<div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<span class="font-heading font-extrabold text-ink">{inv.workspace_name}</span>
										<Badge variant="cobalt">{inv.role}</Badge>
										<Badge variant={inv.workspace_plan === 'free' ? 'yellow' : 'cream'}>
											{inv.workspace_plan}
										</Badge>
									</div>
									<p class="mt-0.5 font-mono text-xs text-ink/70">
										{#if inv.invited_by_email}
											Invited by {inv.invited_by_name ? `${inv.invited_by_name} (${inv.invited_by_email})` : inv.invited_by_email}
										{:else}
											Workspace slug: {inv.workspace_slug}
										{/if}
									</p>
								</div>
								<div class="flex items-center gap-2">
									<Button
										variant="yellow"
										size="sm"
										disabled={inviteBusyId === inv.workspace_id}
										onclick={() => handleAcceptInvite(inv.workspace_id)}
									>
										<Check class="h-3.5 w-3.5" />
										<span>Accept</span>
									</Button>
									<Button
										variant="danger-outline"
										size="sm"
										disabled={inviteBusyId === inv.workspace_id}
										onclick={() => handleDeclineInvite(inv.workspace_id)}
									>
										<X class="h-3.5 w-3.5" />
										<span>Decline</span>
									</Button>
								</div>
							</div>
						{/each}
					</div>
				</Card>
			{/if}

			{#if !authStore.user.is_superadmin && authStore.workspace}
				<Card>
					<div class="flex flex-wrap items-center justify-between gap-4">
						<div>
							<div class="flex items-center gap-2">
								<Building2 class="h-5 w-5 text-cobalt" />
								<span class="font-mono text-xs font-bold uppercase tracking-wider text-ink/65">
									Active Workspace
								</span>
							</div>
							<div class="mt-2 flex flex-wrap items-center gap-2">
								<h2 class="font-heading text-xl font-extrabold text-ink">
									{authStore.workspace.name}
								</h2>
								<Badge variant={authStore.workspace.plan === 'free' ? 'yellow' : 'cobalt'}>
									{authStore.workspace.plan}
								</Badge>
								<Badge variant="cream">{authStore.user.role}</Badge>
							</div>
							<p class="mt-1 font-mono text-xs text-ink/70">
								Slug: {authStore.workspace.slug} ·
								{authStore.maxUsers === 0 ? 'Unlimited seats' : `${authStore.maxUsers} seat limit`}
							</p>
						</div>
						{#if authStore.canViewUsers}
							<Button variant="secondary" size="sm" onclick={() => router.navigate('/workspace/users')}>
								<Users class="h-4 w-4" />
								<span>Manage Members</span>
								<ArrowRight class="h-4 w-4" />
							</Button>
						{/if}
					</div>
				</Card>
			{/if}

			<div class="grid gap-6 md:grid-cols-3">
				<Card>
					<div class="flex items-center justify-between">
						<span class="font-mono text-xs font-bold uppercase tracking-wider text-ink/65">
							Account
						</span>
						<UserCheck class="h-5 w-5 text-cobalt" />
					</div>
					<p class="mt-3 font-heading text-lg font-extrabold text-ink">{authStore.user.email}</p>
					<div class="mt-3">
						<Badge variant="yellow">{authStore.user.status}</Badge>
					</div>
				</Card>

				<Card>
					<div class="flex items-center justify-between">
						<span class="font-mono text-xs font-bold uppercase tracking-wider text-ink/65">
							Email Status
						</span>
						<MailCheck class="h-5 w-5 text-cobalt" />
					</div>
					<p class="mt-3 font-heading text-lg font-extrabold text-ink">
						{authStore.user.email_verified_at ? 'Verified' : 'Unverified'}
					</p>
					<div class="mt-3">
						<Badge variant={authStore.user.email_verified_at ? 'cobalt' : 'coral'}>
							{authStore.user.email_verified_at ? 'Confirmed' : 'Pending'}
						</Badge>
					</div>
				</Card>

				<Card>
					<div class="flex items-center justify-between">
						<span class="font-mono text-xs font-bold uppercase tracking-wider text-ink/65">
							Two-Factor Auth
						</span>
						<ShieldCheck class="h-5 w-5 text-cobalt" />
					</div>
					<p class="mt-3 font-heading text-lg font-extrabold text-ink">
						{authStore.user.two_factor_enabled
							? `Enabled (${authStore.user.two_factor_method?.toUpperCase()})`
							: 'Disabled'}
					</p>
					<div class="mt-3">
						<Badge variant={authStore.user.two_factor_enabled ? 'cobalt' : 'coral'}>
							{authStore.user.two_factor_enabled ? 'Protected' : 'Recommended'}
						</Badge>
					</div>
				</Card>
			</div>
		</div>
	{/if}
</AppShell>

