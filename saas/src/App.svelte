<script lang="ts">
	import { authApi } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import LoginPage from './routes/LoginPage.svelte';
	import RegisterPage from './routes/RegisterPage.svelte';
	import VerifyEmailPage from './routes/VerifyEmailPage.svelte';
	import ForgotPasswordPage from './routes/ForgotPasswordPage.svelte';
	import ResetPasswordPage from './routes/ResetPasswordPage.svelte';
	import ConfirmEmailChangePage from './routes/ConfirmEmailChangePage.svelte';
	import ConfirmAccountDeletePage from './routes/ConfirmAccountDeletePage.svelte';
	import RestoreAccountPage from './routes/RestoreAccountPage.svelte';
	import DashboardPage from './routes/DashboardPage.svelte';
	import ProfileSettingsPage from './routes/ProfileSettingsPage.svelte';
	import SecuritySettingsPage from './routes/SecuritySettingsPage.svelte';
	import WorkspaceUsersPage from './routes/WorkspaceUsersPage.svelte';
	import PlatformPage from './routes/PlatformPage.svelte';

	const protectedRoutes = new Set([
		'/dashboard',
		'/platform',
		'/workspace/users',
		'/settings',
		'/settings/profile',
		'/settings/security'
	]);
	const guestOnlyRoutes = new Set(['/login', '/register', '/forgot-password']);

	$effect(() => {
		if (typeof window !== 'undefined') {
			const params = new URLSearchParams(window.location.search);
			if (params.has('state') && (params.has('code') || params.has('error'))) {
				window.location.replace(authApi.oauthCallbackUrl(window.location.search));
				return;
			}
		}
		void authStore.init();
	});

	$effect(() => {
		if (!authStore.initialized) return;

		const path = router.pathname;
		if (path === '/') {
			router.navigate(authStore.user ? '/dashboard' : '/login', true);
			return;
		}
		if (path === '/settings') {
			router.navigate(authStore.user ? '/settings/profile' : '/login', true);
			return;
		}
		if (path === '/settings/users') {
			router.navigate(
				!authStore.user
					? '/login'
					: authStore.canViewUsers
						? '/workspace/users'
						: authStore.user.is_superadmin
							? '/platform'
							: '/dashboard',
				true
			);
			return;
		}
		if (protectedRoutes.has(path) && !authStore.user) {
			router.navigate('/login', true);
			return;
		}
		if (path === '/workspace/users' && authStore.user && !authStore.canViewUsers) {
			router.navigate(authStore.user.is_superadmin ? '/platform' : '/dashboard', true);
			return;
		}
		if (path === '/platform' && authStore.user && !authStore.user.is_superadmin) {
			router.navigate('/dashboard', true);
			return;
		}
		if (guestOnlyRoutes.has(path) && authStore.user) {
			router.navigate('/dashboard', true);
		}
	});
</script>

{#if !authStore.initialized}
	<div class="min-h-screen bg-cream bg-brutal-grid flex items-center justify-center">
		<div class="rounded-md border-2 border-ink bg-surface px-6 py-4 font-mono text-sm font-bold shadow-brutal">
			Loading RiraLicense...
		</div>
	</div>
{:else if router.pathname === '/register'}
	<RegisterPage />
{:else if router.pathname === '/verify-email'}
	<VerifyEmailPage />
{:else if router.pathname === '/forgot-password'}
	<ForgotPasswordPage />
{:else if router.pathname === '/reset-password'}
	<ResetPasswordPage />
{:else if router.pathname === '/confirm-email-change'}
	<ConfirmEmailChangePage />
{:else if router.pathname === '/settings/account/delete/confirm'}
	<ConfirmAccountDeletePage />
{:else if router.pathname === '/account/restore' || router.pathname === '/settings/account/restore'}
	<RestoreAccountPage />
{:else if router.pathname === '/dashboard'}
	<DashboardPage />
{:else if router.pathname === '/settings/profile'}
	<ProfileSettingsPage />
{:else if router.pathname === '/settings/security'}
	<SecuritySettingsPage />
{:else if router.pathname === '/workspace/users'}
	<WorkspaceUsersPage />
{:else if router.pathname === '/platform'}
	<PlatformPage />
{:else}
	<LoginPage />
{/if}

