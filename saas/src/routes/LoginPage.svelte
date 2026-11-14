<script lang="ts">
	import { ApiError, authApi, type LoginResponse } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import OAuthButtons from '$lib/components/auth/OAuthButtons.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let email = $state('');
	let password = $state('');
	let submitting = $state(false);
	let errorMsg = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let needsEmailVerify = $state(false);

	// 2FA challenge state
	let twoFactorMethod = $state<'totp' | 'email' | null>(null);
	let code = $state('');
	let useBackupCode = $state(false);
	let rememberDevice = $state(false);
	let infoMsg = $state('');

	$effect(() => {
		const oauthErr = router.queryParam('error');
		if (oauthErr === 'oauth_failed' || oauthErr === 'oauth_denied') {
			errorMsg = 'OAuth sign-in could not be completed. Please try again.';
		}
	});

	async function handleLogin(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		infoMsg = '';
		fieldErrors = {};
		needsEmailVerify = false;
		submitting = true;

		try {
			const res = await authApi.login(email, password);
			if (res.two_factor_required) {
				twoFactorMethod = res.method ?? 'totp';
				code = '';
				useBackupCode = false;
				return;
			}
			if (res.user) {
				authStore.setUser(res.user);
				router.navigate('/dashboard', true);
			}
		} catch (err) {
			if (err instanceof ApiError) {
				errorMsg = err.message;
				fieldErrors = err.fields ?? {};
				if (err.code === 'email_not_verified') {
					needsEmailVerify = true;
				}
			} else {
				errorMsg = 'Unable to sign in right now.';
			}
		} finally {
			submitting = false;
		}
	}

	async function handleTwoFactorSubmit(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		infoMsg = '';
		submitting = true;

		try {
			const payload = useBackupCode
				? { backup_code: code.trim(), remember_device: rememberDevice }
				: { code: code.trim(), remember_device: rememberDevice };

			const res = (
				twoFactorMethod === 'email'
					? await authApi.verifyEmailOTP(payload)
					: await authApi.verifyTOTP(payload)
			) as LoginResponse;

			if (res.user) {
				authStore.setUser(res.user);
				router.navigate('/dashboard', true);
			}
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Invalid verification code.';
		} finally {
			submitting = false;
		}
	}

	async function handleResendEmailOTP() {
		errorMsg = '';
		infoMsg = '';
		try {
			await authApi.sendEmailOTP();
			infoMsg = 'A fresh 6-digit verification code has been sent to your email.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to send code.';
		}
	}

	function resetToPasswordStep() {
		twoFactorMethod = null;
		code = '';
		errorMsg = '';
		infoMsg = '';
	}
</script>

<AuthShell
	title={twoFactorMethod ? 'Two-Factor Verification' : 'Sign in to RiraLicense'}
	subtitle={twoFactorMethod
		? twoFactorMethod === 'email'
			? 'Enter the 6-digit code sent to your email address.'
			: 'Enter the 6-digit code from your authenticator app.'
		: 'Access your licensing control plane.'}
>
	{#if errorMsg}
		<div
			role="alert"
			class="mb-4 rounded-md border-2 border-ink bg-coral/25 px-3.5 py-2.5 text-sm font-medium text-ink"
		>
			<p>{errorMsg}</p>
			{#if needsEmailVerify}
				<button
					type="button"
					onclick={() => router.navigate(`/verify-email?email=${encodeURIComponent(email)}`)}
					class="mt-1 font-heading text-xs font-bold underline cursor-pointer"
				>
					Resend verification email
				</button>
			{/if}
		</div>
	{/if}

	{#if infoMsg}
		<div
			class="mb-4 rounded-md border-2 border-ink bg-yellow/40 px-3.5 py-2.5 text-sm font-medium text-ink"
		>
			{infoMsg}
		</div>
	{/if}

	{#if twoFactorMethod}
		<form onsubmit={handleTwoFactorSubmit} class="space-y-4">
			<div>
				<label for="twofa-code" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					{useBackupCode ? 'Backup Code' : 'Verification Code'}
				</label>
				<input
					id="twofa-code"
					type="text"
					autocomplete="one-time-code"
					required
					bind:value={code}
					placeholder={useBackupCode ? 'XXXX-XXXX' : '000000'}
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 font-mono text-base font-bold tracking-widest text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
			</div>

			<label class="flex items-center gap-2.5 text-sm font-medium text-ink cursor-pointer">
				<input
					type="checkbox"
					bind:checked={rememberDevice}
					class="h-4 w-4 rounded border-2 border-ink accent-cobalt"
				/>
				<span>Remember this device for 30 days</span>
			</label>

			<Button type="submit" fullWidth disabled={submitting}>
				{submitting ? 'Verifying...' : 'Verify & Sign In'}
			</Button>

			<div class="flex flex-wrap items-center justify-between gap-2 pt-2 text-xs font-bold">
				<button
					type="button"
					onclick={() => {
						useBackupCode = !useBackupCode;
						code = '';
						errorMsg = '';
					}}
					class="font-heading text-cobalt underline cursor-pointer"
				>
					{useBackupCode ? 'Use authenticator / email code instead' : 'Use backup code'}
				</button>

				{#if twoFactorMethod === 'email' && !useBackupCode}
					<button
						type="button"
						onclick={handleResendEmailOTP}
						class="font-heading text-ink underline cursor-pointer"
					>
						Send code by email
					</button>
				{/if}
			</div>

			<div class="pt-2 border-t border-ink/15">
				<button
					type="button"
					onclick={resetToPasswordStep}
					class="font-heading text-xs font-bold text-ink/70 hover:text-ink underline cursor-pointer"
				>
					Back to sign in
				</button>
			</div>
		</form>
	{:else}
		<form onsubmit={handleLogin} class="space-y-4">
			<div>
				<label for="login-email" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Email
				</label>
				<input
					id="login-email"
					type="email"
					autocomplete="email"
					required
					bind:value={email}
					placeholder="you@company.com"
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
				{#if fieldErrors.email}
					<p class="mt-1 text-xs font-semibold text-coral-hover">{fieldErrors.email}</p>
				{/if}
			</div>

			<div>
				<div class="mb-1.5 flex items-center justify-between">
					<label for="login-password" class="font-heading text-sm font-bold text-ink">
						Password
					</label>
					<button
						type="button"
						onclick={() => router.navigate('/forgot-password')}
						class="font-heading text-xs font-bold text-cobalt underline cursor-pointer"
					>
						Forgot password?
					</button>
				</div>
				<input
					id="login-password"
					type="password"
					autocomplete="current-password"
					required
					bind:value={password}
					placeholder="••••••••••••"
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
				{#if fieldErrors.password}
					<p class="mt-1 text-xs font-semibold text-coral-hover">{fieldErrors.password}</p>
				{/if}
			</div>

			<Button type="submit" fullWidth disabled={submitting}>
				{submitting ? 'Signing in...' : 'Sign In'}
			</Button>
		</form>

		<OAuthButtons />

		<p class="mt-6 text-center text-sm text-ink/75">
			Don't have an account?
			<button
				type="button"
				onclick={() => router.navigate('/register')}
				class="font-heading font-bold text-cobalt underline cursor-pointer"
			>
				Create account
			</button>
		</p>
	{/if}
</AuthShell>
