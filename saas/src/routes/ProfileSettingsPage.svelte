<script lang="ts">
	import { User, Upload, Trash2, Check, X, Mail } from '@lucide/svelte';
	import { ApiError, authApi, resolveAvatarUrl } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import { computeInitials } from '$lib/components/layout/AppShell.svelte';
	import SettingsLayout from '$lib/components/layout/SettingsLayout.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	const MAX_AVATAR_BYTES = 2 * 1024 * 1024; // 2 MB
	const MIN_NAME_LENGTH = 2;
	const MAX_NAME_LENGTH = 100;

	let errorMsg = $state('');
	let successMsg = $state('');

	// Display Name form state
	let displayName = $state(authStore.user?.name ?? '');
	let nameInitialized = $state(false);
	let nameFieldError = $state('');
	let nameSaving = $state(false);

	// Avatar state
	let fileInputEl = $state<HTMLInputElement | null>(null);
	let previewDataUrl = $state<string | null>(null);
	let pendingAvatarBlob = $state<Blob | null>(null);
	let pendingAvatarFilename = $state('avatar.png');
	let avatarSaving = $state(false);
	let avatarRemoving = $state(false);
	let currentAvatarImgError = $state(false);

	$effect(() => {
		if (authStore.user && !nameInitialized) {
			displayName = authStore.user.name;
			nameInitialized = true;
		}
	});

	const resolvedCurrentAvatar = $derived(resolveAvatarUrl(authStore.user?.avatar_url));

	$effect(() => {
		void resolvedCurrentAvatar;
		currentAvatarImgError = false;
	});

	const userInitials = $derived(
		computeInitials(authStore.user?.name, authStore.user?.email)
	);

	function detectAllowedMime(bytes: Uint8Array): { mime: string; ext: string } | null {
		if (bytes.length < 12) return null;
		// JPEG: FF D8 FF
		if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
			return { mime: 'image/jpeg', ext: 'jpg' };
		}
		// PNG: 89 50 4E 47 0D 0A 1A 0A
		if (
			bytes[0] === 0x89 &&
			bytes[1] === 0x50 &&
			bytes[2] === 0x4e &&
			bytes[3] === 0x47 &&
			bytes[4] === 0x0d &&
			bytes[5] === 0x0a &&
			bytes[6] === 0x1a &&
			bytes[7] === 0x0a
		) {
			return { mime: 'image/png', ext: 'png' };
		}
		// WebP: RIFF....WEBP
		if (
			bytes[0] === 0x52 &&
			bytes[1] === 0x49 &&
			bytes[2] === 0x46 &&
			bytes[3] === 0x46 &&
			bytes[8] === 0x57 &&
			bytes[9] === 0x45 &&
			bytes[10] === 0x42 &&
			bytes[11] === 0x50
		) {
			return { mime: 'image/webp', ext: 'webp' };
		}
		return null;
	}

	function cropSquareImage(
		file: File,
		fallbackMime: string,
		fallbackExt: string
	): Promise<{ blob: Blob; dataUrl: string; ext: string }> {
		return new Promise((resolve, reject) => {
			const objectUrl = URL.createObjectURL(file);
			const img = new Image();

			img.onload = () => {
				URL.revokeObjectURL(objectUrl);
				const width = img.naturalWidth || img.width;
				const height = img.naturalHeight || img.height;
				if (width <= 0 || height <= 0) {
					reject(new Error('Invalid image dimensions.'));
					return;
				}

				const side = Math.min(width, height);
				const sx = Math.floor((width - side) / 2);
				const sy = Math.floor((height - side) / 2);
				const targetSize = 256;

				const canvas = document.createElement('canvas');
				canvas.width = targetSize;
				canvas.height = targetSize;
				const ctx = canvas.getContext('2d');
				if (!ctx) {
					reject(new Error('Could not initialize image cropper.'));
					return;
				}

				ctx.drawImage(img, sx, sy, side, side, 0, 0, targetSize, targetSize);

				const webpProbe = canvas.toDataURL('image/webp', 0.92);
				const useWebp = webpProbe.startsWith('data:image/webp');
				const outputMime = useWebp ? 'image/webp' : fallbackMime;
				const outputExt = useWebp ? 'webp' : fallbackExt;
				const dataUrl = useWebp ? webpProbe : canvas.toDataURL(outputMime, 0.92);

				canvas.toBlob(
					(blob) => {
						if (!blob) {
							reject(new Error('Failed to crop image.'));
							return;
						}
						resolve({ blob, dataUrl, ext: outputExt });
					},
					outputMime,
					0.92
				);
			};

			img.onerror = () => {
				URL.revokeObjectURL(objectUrl);
				reject(
					new Error(
						'Unsupported or corrupted image file. Only valid JPG, PNG, and WebP images are allowed.'
					)
				);
			};

			img.src = objectUrl;
		});
	}

	async function handleFileChange(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		errorMsg = '';
		successMsg = '';

		if (file.size > MAX_AVATAR_BYTES) {
			clearPreview();
			errorMsg = 'Avatar file exceeds the 2 MB maximum size limit.';
			return;
		}

		try {
			const headerBuf = await file.slice(0, 12).arrayBuffer();
			const detected = detectAllowedMime(new Uint8Array(headerBuf));
			if (!detected) {
				clearPreview();
				errorMsg =
					'Unsupported file format. Only JPG, PNG, and WebP images are allowed.';
				return;
			}

			const { blob, dataUrl, ext } = await cropSquareImage(
				file,
				detected.mime,
				detected.ext
			);
			if (blob.size > MAX_AVATAR_BYTES) {
				clearPreview();
				errorMsg = 'Cropped avatar exceeds the 2 MB maximum size limit.';
				return;
			}

			pendingAvatarBlob = blob;
			pendingAvatarFilename = `avatar.${ext}`;
			previewDataUrl = dataUrl;
		} catch (err) {
			clearPreview();
			errorMsg =
				err instanceof Error
					? err.message
					: 'Unsupported image format. Only JPG, PNG, and WebP images are allowed.';
		}
	}

	function clearPreview() {
		previewDataUrl = null;
		pendingAvatarBlob = null;
		if (fileInputEl) {
			fileInputEl.value = '';
		}
	}

	async function handleConfirmAvatarUpload() {
		if (!pendingAvatarBlob) return;
		errorMsg = '';
		successMsg = '';
		avatarSaving = true;
		try {
			await authApi.uploadAvatar(pendingAvatarBlob, pendingAvatarFilename);
			clearPreview();
			await authStore.refreshUser();
			successMsg = 'Avatar updated.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to upload avatar.';
		} finally {
			avatarSaving = false;
		}
	}

	async function handleRemoveAvatar() {
		errorMsg = '';
		successMsg = '';
		avatarRemoving = true;
		try {
			await authApi.deleteAvatar();
			clearPreview();
			await authStore.refreshUser();
			successMsg = 'Avatar removed.';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Failed to remove avatar.';
		} finally {
			avatarRemoving = false;
		}
	}

	async function handleSaveName(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		nameFieldError = '';

		const trimmed = displayName.trim();
		if (trimmed.length === 0) {
			nameFieldError = 'Display name is required.';
			errorMsg = 'Display name is required.';
			return;
		}
		if (trimmed.length < MIN_NAME_LENGTH || trimmed.length > MAX_NAME_LENGTH) {
			nameFieldError = 'Display name must be between 2 and 100 characters.';
			errorMsg = 'Display name must be between 2 and 100 characters.';
			return;
		}

		nameSaving = true;
		try {
			await authApi.updateProfile(trimmed);
			await authStore.refreshUser();
			displayName = authStore.user?.name ?? trimmed;
			successMsg = 'Profile updated.';
		} catch (err) {
			if (err instanceof ApiError) {
				errorMsg = err.message;
				if (err.fields?.name) {
					nameFieldError = err.fields.name;
				}
			} else {
				errorMsg = 'Failed to update profile.';
			}
		} finally {
			nameSaving = false;
		}
	}
</script>

<SettingsLayout>
	{#if authStore.user}
		<div class="space-y-6">
			<div>
				<h1 class="font-heading text-3xl font-extrabold text-ink">Profile Settings</h1>
				<p class="mt-1 text-sm text-ink/75">
					Manage your public avatar and display name across the control plane.
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
					role="status"
					class="rounded-md border-2 border-ink bg-yellow/40 px-4 py-3 text-sm font-semibold text-ink"
				>
					{successMsg}
				</div>
			{/if}

			<!-- Avatar Card -->
			<Card>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div class="flex items-center gap-2.5">
						<User class="h-5 w-5 text-cobalt" />
						<h2 class="font-heading text-xl font-extrabold text-ink">Avatar</h2>
					</div>
					{#if previewDataUrl}
						<Badge variant="yellow">Square Preview Ready</Badge>
					{/if}
				</div>

				<p class="mt-1 text-sm text-ink/75">
					JPG, PNG, or WebP. Maximum 2 MB. Automatically square-cropped before upload.
				</p>

				<div class="mt-5 flex flex-wrap items-center gap-6">
					<div
						data-avatar-preview
						class="flex h-24 w-24 shrink-0 items-center justify-center overflow-hidden rounded-md border-2 border-ink bg-yellow font-heading text-2xl font-extrabold uppercase tracking-tight text-ink shadow-brutal-sm"
					>
						{#if previewDataUrl}
							<img
								src={previewDataUrl}
								alt="Cropped avatar preview"
								class="h-full w-full object-cover"
							/>
						{:else if resolvedCurrentAvatar && !currentAvatarImgError}
							<img
								src={resolvedCurrentAvatar}
								alt={authStore.user.name || 'Current avatar'}
								class="h-full w-full object-cover"
								onerror={() => (currentAvatarImgError = true)}
							/>
						{:else}
							<span>{userInitials}</span>
						{/if}
					</div>

					<div class="space-y-3">
						<input
							bind:this={fileInputEl}
							type="file"
							accept="image/jpeg,image/png,image/webp"
							onchange={handleFileChange}
							class="sr-only"
							aria-label="Upload avatar image"
						/>

						{#if previewDataUrl}
							<div class="flex flex-wrap items-center gap-3">
								<Button
									onclick={handleConfirmAvatarUpload}
									disabled={avatarSaving}
								>
									<Check class="h-4 w-4" />
									<span>{avatarSaving ? 'Uploading...' : 'Save Avatar'}</span>
								</Button>
								<Button
									variant="secondary"
									onclick={clearPreview}
									disabled={avatarSaving}
								>
									<X class="h-4 w-4" />
									<span>Cancel</span>
								</Button>
							</div>
							<p class="text-xs font-semibold text-ink/75">
								Previewing square-cropped avatar. Click <strong>Save Avatar</strong> to apply.
							</p>
						{:else}
							<div class="flex flex-wrap items-center gap-3">
								<Button
									variant="secondary"
									onclick={() => fileInputEl?.click()}
									disabled={avatarSaving || avatarRemoving}
								>
									<Upload class="h-4 w-4" />
									<span>Upload</span>
								</Button>

								{#if authStore.user.avatar_url}
									<Button
										variant="coral"
										onclick={handleRemoveAvatar}
										disabled={avatarRemoving || avatarSaving}
									>
										<Trash2 class="h-4 w-4" />
										<span>{avatarRemoving ? 'Removing...' : 'Remove'}</span>
									</Button>
								{/if}
							</div>
						{/if}
					</div>
				</div>
			</Card>

			<!-- Display Name & Read-Only Email Card -->
			<Card>
				<div class="flex items-center gap-2.5">
					<Mail class="h-5 w-5 text-cobalt" />
					<h2 class="font-heading text-xl font-extrabold text-ink">Profile Details</h2>
				</div>

				<form onsubmit={handleSaveName} novalidate class="mt-4 max-w-md space-y-4">
					<div>
						<label
							for="profile-name"
							class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
						>
							Display Name
						</label>
						<input
							id="profile-name"
							type="text"
							required
							minlength={MIN_NAME_LENGTH}
							maxlength={MAX_NAME_LENGTH}
							bind:value={displayName}
							placeholder="Ada Lovelace"
							class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
						/>
						{#if nameFieldError}
							<p class="mt-1 text-xs font-semibold text-coral-hover">{nameFieldError}</p>
						{:else}
							<p class="mt-1 text-xs text-ink/65">Required, 2–100 characters.</p>
						{/if}
					</div>

					<div>
						<label
							for="profile-email"
							class="mb-1 block font-heading text-xs font-bold uppercase text-ink"
						>
							Email Address
						</label>
						<input
							id="profile-email"
							type="email"
							value={authStore.user.email}
							readonly
							disabled
							class="w-full rounded-md border-2 border-ink/50 bg-cream-dark px-3.5 py-2 text-sm text-ink/75 cursor-not-allowed"
						/>
						<p class="mt-1.5 text-xs text-ink/75">
							Change your email in the
							<a
								href="/settings/security"
								onclick={(e) => {
									e.preventDefault();
									router.navigate('/settings/security');
								}}
								class="font-heading font-bold text-cobalt underline cursor-pointer"
							>
								Security tab
							</a>.
						</p>
					</div>

					<Button type="submit" disabled={nameSaving}>
						{nameSaving ? 'Saving...' : 'Save'}
					</Button>
				</form>
			</Card>
		</div>
	{/if}
</SettingsLayout>

