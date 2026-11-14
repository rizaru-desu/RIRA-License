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
	import SecuritySettingsPage from './routes/SecuritySettingsPage.svelte';

	const protectedRoutes = new Set(['/dashboard', '/settings/security']);
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
		if (protectedRoutes.has(path) && !authStore.user) {
			router.navigate('/login', true);
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
{:else if router.pathname === '/settings/security'}
	<SecuritySettingsPage />
{:else}
	<LoginPage />
{/if}

