<script lang="ts">
	import QRCode from 'qrcode';
	import { Copy, Check, ShieldCheck, KeyRound, Mail, LogOut } from '@lucide/svelte';
	import { ApiError, authApi, type EnableTwoFactorResponse } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let errorMsg = $state('');
	let successMsg = $state('');

	// TOTP setup state
	let totpSecret = $state('');
	let totpQrDataUrl = $state('');
	let totpVerifyCode = $state('');
	let totpLoading = $state(false);
	let copiedSecret = $state(false);

	// Email 2FA setup state
	let email2faSent = $state(false);
	let email2faCode = $state('');
	let email2faLoading = $state(false);

	// One-time backup codes display
	let backupCodes = $state<string[]>([]);
	let copiedBackupCodes = $state(false);

	// Regenerate backup codes state
	let regenPassword = $state('');
	let regenCode = $state('');
	let regenLoading = $state(false);

	// Disable 2FA state
	let disablePassword = $state('');
	let disableCode = $state('');
	let disableUseBackup = $state(false);
	let disableLoading = $state(false);

	// Change password state
	let currentPassword = $state('');
	let newPassword = $state('');
	let passwordLoading = $state(false);

	// Change email state
	let newEmail = $state('');
	let emailChangePassword = $state('');
	let emailChangeLoading = $state(false);

	async function startTOTPSetup() {
		errorMsg = '';
		successMsg = '';
		totpLoading = true;
		try {
			const res = await authApi.setupTOTP();
			totpSecret = res.secret;
			totpQrDataUrl = await QRCode.toDataURL(res.provisioning_uri, {
				width: 220,
				margin: 2,
				color: { dark: '#171717', light: '#FFFFFF' }
			});
			email2faSent = false;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Could not start TOTP setup.';
		} finally {
			totpLoading = false;
		}
	}

	async function confirmTOTPSetup(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		totpLoading = true;
		try {
			const res = (await authApi.verifyTOTP({
				code: totpVerifyCode.trim()
			})) as EnableTwoFactorResponse;
			backupCodes = res.backup_codes ?? [];
			totpSecret = '';
			totpQrDataUrl = '';
			totpVerifyCode = '';
			await authStore.refreshUser();
			successMsg = 'Authenticator app 2FA enabled. Save your backup codes now.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Invalid TOTP code.';
		} finally {
			totpLoading = false;
		}
	}

	async function startEmail2FA() {
		errorMsg = '';
		successMsg = '';
		email2faLoading = true;
		try {
			await authApi.sendEmailOTP();
			email2faSent = true;
			totpSecret = '';
			totpQrDataUrl = '';
			successMsg = 'A 6-digit verification code was sent to your email.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to send email OTP.';
		} finally {
			email2faLoading = false;
		}
	}

	async function confirmEmail2FA(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		email2faLoading = true;
		try {
			const res = (await authApi.verifyEmailOTP({
				code: email2faCode.trim()
			})) as EnableTwoFactorResponse;
			backupCodes = res.backup_codes ?? [];
			email2faSent = false;
			email2faCode = '';
			await authStore.refreshUser();
			successMsg = 'Email 2FA enabled. Save your backup codes now.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Invalid email OTP code.';
		} finally {
			email2faLoading = false;
		}
	}

	async function handleRegenerateBackupCodes(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		regenLoading = true;
		try {
			const res = await authApi.regenerateBackupCodes({
				password: regenPassword || undefined,
				code: regenCode.trim() || undefined
			});
			backupCodes = res.backup_codes ?? [];
			regenPassword = '';
			regenCode = '';
			successMsg = 'Generated 10 new backup codes. Previous codes are now invalid.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Could not regenerate backup codes.';
		} finally {
			regenLoading = false;
		}
	}

	async function sendDisableEmailOTP() {
		errorMsg = '';
		successMsg = '';
		try {
			await authApi.sendEmailOTP();
			successMsg = 'Verification code sent to your email.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to send code.';
		}
	}

	async function handleDisableTwoFactor(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		disableLoading = true;
		try {
			await authApi.disableTwoFactor({
				password: disablePassword || undefined,
				code: disableUseBackup ? undefined : disableCode.trim(),
				backup_code: disableUseBackup ? disableCode.trim() : undefined
			});
			authStore.setUser(null);
			router.navigate('/login', true);
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to disable 2FA.';
		} finally {
			disableLoading = false;
		}
	}

	async function handleChangePassword(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		if (newPassword.length < 12) {
			errorMsg = 'New password must be at least 12 characters.';
			return;
		}
		passwordLoading = true;
		try {
			await authApi.changePassword(currentPassword, newPassword);
			authStore.setUser(null);
			router.navigate('/login', true);
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to update password.';
		} finally {
			passwordLoading = false;
		}
	}

	async function handleChangeEmail(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		emailChangeLoading = true;
		try {
			const res = await authApi.changeEmail(newEmail, emailChangePassword);
			successMsg = res.message;
			newEmail = '';
			emailChangePassword = '';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to request email change.';
		} finally {
			emailChangeLoading = false;
		}
	}

	async function handleLogoutAll() {
		errorMsg = '';
		try {
			await authApi.logoutAll();
			authStore.setUser(null);
			router.navigate('/login', true);
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to revoke sessions.';
		}
	}

	async function copyText(value: string, target: 'secret' | 'codes') {
		await navigator.clipboard.writeText(value);
		if (target === 'secret') {
			copiedSecret = true;
			setTimeout(() => (copiedSecret = false), 2000);
		} else {
			copiedBackupCodes = true;
			setTimeout(() => (copiedBackupCodes = false), 2000);
		}
	}
</script>

<AppShell>
	{#if authStore.user}
		<div class="space-y-6">
			<div>
				<h1 class="font-heading text-3xl font-extrabold text-ink">Security Settings</h1>
				<p class="mt-1 text-sm text-ink/75">
					Manage two-factor authentication, password, email address, and active sessions.
				</p>
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
					class="rounded-md border-2 border-ink bg-yellow/40 px-4 py-3 text-sm font-semibold text-ink"
				>
					{successMsg}
				</div>
			{/if}

			{#if backupCodes.length > 0}
				<Card variant="yellow" shadow="lg">
					<div class="flex flex-wrap items-center justify-between gap-3">
						<div>
							<h2 class="font-heading text-xl font-extrabold text-ink">
								Save Your Backup Codes (Shown Once)
							</h2>
							<p class="mt-1 text-sm text-ink/80">
								Each code works once if you lose access to your authenticator or email.
							</p>
						</div>
						<Button
							variant="secondary"
							size="sm"
							onclick={() => copyText(backupCodes.join('\n'), 'codes')}
						>
							{#if copiedBackupCodes}
								<Check class="h-4 w-4" />
								<span>Copied</span>
							{:else}
								<Copy class="h-4 w-4" />
								<span>Copy All</span>
							{/if}
						</Button>
					</div>

					<div class="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-5">
						{#each backupCodes as bc (bc)}
							<div
								class="rounded border-2 border-ink bg-surface px-3 py-2 text-center font-mono text-sm font-bold text-ink"
							>
								{bc}
							</div>
						{/each}
					</div>
				</Card>
			{/if}

			<!-- Two-Factor Authentication -->
			<Card>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div class="flex items-center gap-2.5">
						<ShieldCheck class="h-5 w-5 text-cobalt" />
						<h2 class="font-heading text-xl font-extrabold text-ink">
							Two-Factor Authentication
						</h2>
					</div>
					<Badge variant={authStore.user.two_factor_enabled ? 'cobalt' : 'coral'}>
						{authStore.user.two_factor_enabled
							? `Enabled (${authStore.user.two_factor_method?.toUpperCase()})`
							: 'Disabled'}
					</Badge>
				</div>

				{#if !authStore.user.two_factor_enabled}
					<div class="mt-5 flex flex-wrap gap-3">
						<Button disabled={totpLoading} onclick={startTOTPSetup}>
							{totpLoading ? 'Generating...' : 'Set up Authenticator App (TOTP)'}
						</Button>
						<Button variant="secondary" disabled={email2faLoading} onclick={startEmail2FA}>
							{email2faLoading ? 'Sending...' : 'Enable Email 2FA'}
						</Button>
					</div>

					{#if totpQrDataUrl}
						<div class="mt-6 grid gap-6 border-t-2 border-ink/15 pt-6 md:grid-cols-[220px_1fr]">
							<div class="rounded-md border-2 border-ink bg-surface p-2 shadow-brutal-xs">
								<img src={totpQrDataUrl} alt="TOTP QR Code" class="h-auto w-full" />
							</div>
							<div class="space-y-4">
								<div>
									<p class="font-heading text-sm font-bold text-ink">
										1. Scan the QR code or enter this secret key manually:
									</p>
									<div class="mt-2 flex flex-wrap items-center gap-2">
										<code
											class="rounded border-2 border-ink bg-cream px-3 py-1.5 font-mono text-xs font-bold text-ink"
										>
											{totpSecret}
										</code>
										<Button
											variant="secondary"
											size="sm"
											onclick={() => copyText(totpSecret, 'secret')}
										>
											{#if copiedSecret}
												<Check class="h-3.5 w-3.5" />
												<span>Copied</span>
											{:else}
												<Copy class="h-3.5 w-3.5" />
												<span>Copy</span>
											{/if}
										</Button>
									</div>
								</div>

								<form onsubmit={confirmTOTPSetup} class="space-y-3 max-w-xs">
									<label for="totp-verify" class="block font-heading text-sm font-bold text-ink">
										2. Enter the 6-digit code to activate:
									</label>
									<input
										id="totp-verify"
										type="text"
										inputmode="numeric"
										required
										bind:value={totpVerifyCode}
										placeholder="000000"
										class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 font-mono text-base font-bold tracking-widest text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
									/>
									<Button type="submit" disabled={totpLoading}>
										{totpLoading ? 'Verifying...' : 'Verify & Enable TOTP'}
									</Button>
								</form>
							</div>
						</div>
					{/if}

					{#if email2faSent}
						<form onsubmit={confirmEmail2FA} class="mt-6 max-w-xs space-y-3 border-t-2 border-ink/15 pt-6">
							<label for="email2fa-verify" class="block font-heading text-sm font-bold text-ink">
								Enter the 6-digit code sent to {authStore.user.email}:
							</label>
							<input
								id="email2fa-verify"
								type="text"
								inputmode="numeric"
								required
								bind:value={email2faCode}
								placeholder="000000"
								class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 font-mono text-base font-bold tracking-widest text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
							<Button type="submit" disabled={email2faLoading}>
								{email2faLoading ? 'Verifying...' : 'Verify & Enable Email 2FA'}
							</Button>
						</form>
					{/if}
				{:else}
					<div class="mt-6 grid gap-6 border-t-2 border-ink/15 pt-6 md:grid-cols-2">
						<form onsubmit={handleRegenerateBackupCodes} class="space-y-3">
							<h3 class="font-heading text-base font-extrabold text-ink">
								Regenerate Backup Codes
							</h3>
							{#if authStore.user.has_password}
								<input
									type="password"
									bind:value={regenPassword}
									placeholder="Current password"
									class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
								/>
							{/if}
							<input
								type="text"
								bind:value={regenCode}
								placeholder="Or current 6-digit 2FA code"
								class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 font-mono text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
							<Button type="submit" variant="secondary" disabled={regenLoading}>
								{regenLoading ? 'Generating...' : 'Regenerate 10 Backup Codes'}
							</Button>
						</form>

						<form onsubmit={handleDisableTwoFactor} class="space-y-3">
							<h3 class="font-heading text-base font-extrabold text-ink">
								Disable Two-Factor Authentication
							</h3>
							{#if authStore.user.has_password}
								<input
									type="password"
									required
									bind:value={disablePassword}
									placeholder="Current password"
									class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
								/>
							{/if}
							<input
								type="text"
								required
								bind:value={disableCode}
								placeholder={disableUseBackup ? 'Backup code (XXXX-XXXX)' : 'Current 6-digit 2FA code'}
								class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 font-mono text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
							<div class="flex flex-wrap items-center gap-3">
								<Button type="submit" variant="coral" disabled={disableLoading}>
									{disableLoading ? 'Disabling...' : 'Disable 2FA'}
								</Button>
								<button
									type="button"
									onclick={() => (disableUseBackup = !disableUseBackup)}
									class="font-heading text-xs font-bold text-cobalt underline cursor-pointer"
								>
									{disableUseBackup ? 'Use 2FA code' : 'Use backup code'}
								</button>
								{#if authStore.user.two_factor_method === 'email'}
									<button
										type="button"
										onclick={sendDisableEmailOTP}
										class="font-heading text-xs font-bold text-ink underline cursor-pointer"
									>
										Send Email Code
									</button>
								{/if}
							</div>
						</form>
					</div>
				{/if}
			</Card>

			<div class="grid gap-6 md:grid-cols-2">
				<!-- Change Password -->
				<Card>
					<div class="flex items-center gap-2.5">
						<KeyRound class="h-5 w-5 text-cobalt" />
						<h2 class="font-heading text-xl font-extrabold text-ink">
							{authStore.user.has_password ? 'Change Password' : 'Set Password'}
						</h2>
					</div>
					<form onsubmit={handleChangePassword} class="mt-4 space-y-3">
						{#if authStore.user.has_password}
							<div>
								<label for="cur-pw" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
									Current Password
								</label>
								<input
									id="cur-pw"
									type="password"
									required
									bind:value={currentPassword}
									class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
								/>
							</div>
						{/if}
						<div>
							<label for="new-pw" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
								New Password (min 12 chars)
							</label>
							<input
								id="new-pw"
								type="password"
								required
								minlength={12}
								bind:value={newPassword}
								class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
						<Button type="submit" disabled={passwordLoading}>
							{passwordLoading ? 'Updating...' : 'Update Password'}
						</Button>
					</form>
				</Card>

				<!-- Change Email -->
				<Card>
					<div class="flex items-center gap-2.5">
						<Mail class="h-5 w-5 text-cobalt" />
						<h2 class="font-heading text-xl font-extrabold text-ink">Change Email</h2>
					</div>
					<form onsubmit={handleChangeEmail} class="mt-4 space-y-3">
						<div>
							<label for="new-email" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
								New Email Address
							</label>
							<input
								id="new-email"
								type="email"
								required
								bind:value={newEmail}
								placeholder="new-address@company.com"
								class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
							/>
						</div>
						{#if authStore.user.has_password}
							<div>
								<label for="email-cur-pw" class="mb-1 block font-heading text-xs font-bold uppercase text-ink">
									Current Password
								</label>
								<input
									id="email-cur-pw"
									type="password"
									required
									bind:value={emailChangePassword}
									class="w-full rounded-md border-2 border-ink bg-cream px-3 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
								/>
							</div>
						{/if}
						<Button type="submit" variant="secondary" disabled={emailChangeLoading}>
							{emailChangeLoading ? 'Sending...' : 'Send Confirmation to New Email'}
						</Button>
					</form>
				</Card>
			</div>

			<!-- Sessions -->
			<Card>
				<div class="flex flex-wrap items-center justify-between gap-4">
					<div>
						<h2 class="font-heading text-xl font-extrabold text-ink">Active Sessions</h2>
						<p class="mt-1 text-sm text-ink/75">
							Revoke all database sessions across every browser and device.
						</p>
					</div>
					<Button variant="coral" onclick={handleLogoutAll}>
						<LogOut class="h-4 w-4" />
						<span>Sign Out All Devices</span>
					</Button>
				</div>
			</Card>
		</div>
	{/if}
</AppShell>
