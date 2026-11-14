import { authApi, type UserDTO } from '$lib/api/auth';

class AuthStore {
	user = $state<UserDTO | null>(null);
	providers = $state<string[]>([]);
	initialized = $state(false);
	loading = $state(false);

	async init() {
		this.loading = true;
		try {
			const [providersRes, meRes] = await Promise.allSettled([
				authApi.getProviders(),
				authApi.me()
			]);

			if (providersRes.status === 'fulfilled') {
				this.providers = providersRes.value.providers ?? [];
			}

			if (meRes.status === 'fulfilled') {
				this.user = meRes.value.user;
			} else {
				this.user = null;
			}
		} finally {
			this.loading = false;
			this.initialized = true;
		}
	}

	async refreshUser() {
		try {
			const res = await authApi.me();
			this.user = res.user;
		} catch {
			this.user = null;
		}
	}

	setUser(user: UserDTO | null) {
		this.user = user;
	}

	async logout() {
		try {
			await authApi.logout();
		} finally {
			this.user = null;
		}
	}
}

export const authStore = new AuthStore();

