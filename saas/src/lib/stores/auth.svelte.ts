import {
	authApi,
	setActiveWorkspaceHeader,
	type MeResponse,
	type PendingWorkspaceInviteDTO,
	type UserDTO,
	type WorkspaceDTO
} from '$lib/api/auth';

export const FREE_MAX_USERS = 1;

class AuthStore {
	user = $state<UserDTO | null>(null);
	workspace = $state<WorkspaceDTO | null>(null);
	workspaces = $state<WorkspaceDTO[]>([]);
	pendingInvites = $state<PendingWorkspaceInviteDTO[]>([]);
	plan = $state<string>('free');
	maxUsers = $state<number>(FREE_MAX_USERS);
	providers = $state<string[]>([]);
	initialized = $state(false);
	loading = $state(false);

	private applyMe(val: MeResponse) {
		this.user = {
			...val.user,
			role: val.role ?? val.user.role,
			is_superadmin: val.is_superadmin ?? val.user.is_superadmin,
			permissions: val.permissions ?? val.user.permissions ?? []
		};
		this.workspace = val.workspace ?? null;
		this.workspaces = val.workspaces ?? [];
		this.plan = val.workspace?.plan ?? val.plan ?? 'free';
		this.maxUsers = val.workspace?.max_users ?? val.max_users ?? FREE_MAX_USERS;
		setActiveWorkspaceHeader(this.workspace?.id ?? null);
	}

	private clearSessionState() {
		this.user = null;
		this.workspace = null;
		this.workspaces = [];
		this.pendingInvites = [];
		this.plan = 'free';
		this.maxUsers = FREE_MAX_USERS;
		setActiveWorkspaceHeader(null);
	}

	async refreshPendingInvites() {
		if (!this.user || this.user.is_superadmin) {
			this.pendingInvites = [];
			return;
		}
		try {
			const res = await authApi.listPendingInvites();
			this.pendingInvites = res.invites ?? [];
		} catch {
			// Ignore transient failures on non-critical invite polling
		}
	}

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
				this.applyMe(meRes.value);
				await this.refreshPendingInvites();
			} else {
				this.clearSessionState();
			}
		} finally {
			this.loading = false;
			this.initialized = true;
		}
	}

	async refreshUser() {
		try {
			const res = await authApi.me();
			this.applyMe(res);
			await this.refreshPendingInvites();
		} catch {
			this.clearSessionState();
		}
	}

	async switchWorkspace(workspaceId: string) {
		const res = await authApi.switchWorkspace(workspaceId);
		setActiveWorkspaceHeader(res.workspace.id);
		await this.refreshUser();
	}

	async exitWorkspace() {
		await authApi.exitWorkspace();
		setActiveWorkspaceHeader(null);
		await this.refreshUser();
	}

	async createWorkspace(name: string, slug?: string) {
		const res = await authApi.createWorkspace({ name, slug: slug?.trim() || undefined });
		setActiveWorkspaceHeader(res.workspace.id);
		await this.refreshUser();
		return res.workspace;
	}

	async acceptInvite(workspaceId: string) {
		const res = await authApi.acceptWorkspaceInvite(workspaceId);
		setActiveWorkspaceHeader(res.workspace.id);
		await this.refreshUser();
		return res.workspace;
	}

	async declineInvite(workspaceId: string) {
		await authApi.declineWorkspaceInvite(workspaceId);
		await this.refreshPendingInvites();
	}

	async leaveWorkspace(workspaceId: string) {
		await authApi.leaveWorkspace(workspaceId);
		setActiveWorkspaceHeader(null);
		await this.refreshUser();
	}

	hasPermission(perm: string): boolean {
		if (!this.user) return false;
		if (this.user.is_superadmin) return true;
		return (this.user.permissions ?? []).includes(perm);
	}

	get canViewUsers(): boolean {
		if (!this.user) return false;
		if (this.user.is_superadmin) {
			return this.workspace !== null;
		}
		return (
			this.user.role === 'owner' ||
			this.user.role === 'admin' ||
			this.hasPermission('users:read')
		);
	}

	setUser(user: UserDTO | null) {
		this.user = user;
		if (!user) {
			this.clearSessionState();
		}
	}

	async logout() {
		try {
			await authApi.logout();
		} finally {
			this.clearSessionState();
		}
	}
}

export const authStore = new AuthStore();
