<script lang="ts">
	import { ShieldCheck, MailCheck, UserCheck, ArrowRight } from '@lucide/svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
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

