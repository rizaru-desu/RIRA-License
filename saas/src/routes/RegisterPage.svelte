<script lang="ts">
	import { ApiError, authApi } from '$lib/api/auth';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import OAuthButtons from '$lib/components/auth/OAuthButtons.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import PasswordInput from '$lib/components/ui/PasswordInput.svelte';

	let name = $state('');
	let email = $state('');
	let password = $state('');
	let submitting = $state(false);
	let errorMsg = $state('');
	let fieldErrors = $state<Record<string, string>>({});
	let successMsg = $state('');

	async function handleRegister(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		fieldErrors = {};
		successMsg = '';

		if (password.length < 12) {
			fieldErrors = { password: 'password must be at least 12 characters' };
			return;
		}

		submitting = true;
		try {
			const res = await authApi.register(email, password, name);
			successMsg = res.message;
			password = '';
		} catch (err) {
			if (err instanceof ApiError) {
				errorMsg = err.message;
				fieldErrors = err.fields ?? {};
			} else {
				errorMsg = 'Registration failed. Please try again.';
			}
		} finally {
			submitting = false;
		}
	}
</script>

<AuthShell
	title="Create your account"
	subtitle="Minimum 12-character password. Email verification required."
>
	{#if errorMsg}
		<div
			role="alert"
			class="mb-4 rounded-md border-2 border-ink bg-coral/25 px-3.5 py-2.5 text-sm font-medium text-ink"
		>
			{errorMsg}
		</div>
	{/if}

	{#if successMsg}
		<div
			class="space-y-4 rounded-md border-2 border-ink bg-yellow/40 p-4 text-sm font-medium text-ink"
		>
			<p>{successMsg}</p>
			<Button variant="secondary" fullWidth onclick={() => router.navigate('/login')}>
				Go to Sign In
			</Button>
		</div>
	{:else}
		<form onsubmit={handleRegister} class="space-y-4">
			<div>
				<label for="reg-name" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Full Name
				</label>
				<input
					id="reg-name"
					type="text"
					autocomplete="name"
					required
					bind:value={name}
					placeholder="Ada Lovelace"
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
				{#if fieldErrors.name}
					<p class="mt-1 text-xs font-semibold text-coral-hover">{fieldErrors.name}</p>
				{/if}
			</div>

			<div>
				<label for="reg-email" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Work Email
				</label>
				<input
					id="reg-email"
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
				<label for="reg-password" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Password (min 12 chars)
				</label>
				<PasswordInput
					id="reg-password"
					autocomplete="new-password"
					required
					minlength={12}
					bind:value={password}
					placeholder="At least 12 characters"
				/>
				{#if fieldErrors.password}
					<p class="mt-1 text-xs font-semibold text-coral-hover">{fieldErrors.password}</p>
				{/if}
			</div>

			<Button type="submit" fullWidth disabled={submitting}>
				{submitting ? 'Creating account...' : 'Create Account'}
			</Button>
		</form>

		<OAuthButtons />
	{/if}

	<p class="mt-6 text-center text-sm text-ink/75">
		Already have an account?
		<button
			type="button"
			onclick={() => router.navigate('/login')}
			class="font-heading font-bold text-cobalt underline cursor-pointer"
		>
			Sign in
		</button>
	</p>
</AuthShell>

