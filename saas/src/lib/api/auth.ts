export type UserRole = 'owner' | 'admin' | 'member' | 'viewer';
export type WorkspacePlan = 'free' | 'pro' | 'enterprise';

export interface WorkspaceDTO {
	id: string;
	name: string;
	slug: string;
	plan: WorkspacePlan;
	status: 'active' | 'suspended';
	role?: UserRole;
	suspended_reason?: string | null;
	suspended_at?: string | null;
	max_users: number;
	max_products: number;
	max_licenses: number;
	max_users_override?: number | null;
	max_products_override?: number | null;
	max_licenses_override?: number | null;
	override_expires_at?: string | null;
	override_note?: string | null;
	member_count?: number;
	created_at: string;
}

export interface PlatformUserDTO {
	id: string;
	name: string;
	email: string;
	avatar_url?: string | null;
	is_superadmin: boolean;
	status: 'active' | 'suspended';
	suspended_reason?: string | null;
	suspended_at?: string | null;
	workspace_count: number;
	created_at: string;
}

export interface UserDTO {
	id: string;
	email: string;
	name: string;
	avatar_url: string | null;
	email_verified_at: string | null;
	role: UserRole;
	is_superadmin: boolean;
	permissions: string[];
	status: 'active' | 'suspended';
	suspended_reason?: string | null;
	suspended_at?: string | null;
	active_workspace_id?: string | null;
	two_factor_enabled: boolean;
	two_factor_method: 'totp' | 'email' | null;
	has_password: boolean;
	last_login_at: string | null;
	created_at: string;
}

export interface PendingWorkspaceInviteDTO {
	workspace_id: string;
	workspace_name: string;
	workspace_slug: string;
	workspace_plan: WorkspacePlan;
	role: UserRole;
	invited_at: string;
	invited_by_name?: string | null;
	invited_by_email?: string | null;
}

export interface AdminUserDTO {
	id: string;
	name: string;
	email: string;
	avatar_url?: string | null;
	role: UserRole;
	is_superadmin: boolean;
	status: 'pending' | 'active' | 'suspended';
	global_status?: 'active' | 'suspended';
	suspended_reason?: string | null;
	suspended_at?: string | null;
	email_verified_at?: string | null;
	joined_at?: string | null;
	created_at: string;
}

export interface GlobalSuspendWorkspaceImpactDTO {
	workspace_id: string;
	workspace_name: string;
	workspace_slug: string;
	role: UserRole;
	active_members: number;
	active_admins: number;
	active_owners: number;
	next_active_members: number;
	next_active_admins: number;
	next_active_owners: number;
	blocked: boolean;
	block_code?: string;
	block_reason?: string;
}

export interface GlobalSuspendImpactDTO {
	user_id: string;
	allowed: boolean;
	workspaces: GlobalSuspendWorkspaceImpactDTO[];
}

export interface MeResponse {
	user: UserDTO;
	role: UserRole;
	is_superadmin: boolean;
	permissions: string[];
	plan?: string;
	max_users?: number;
	workspace?: WorkspaceDTO | null;
	workspaces?: WorkspaceDTO[];
}

export interface LoginResponse {
	two_factor_required: boolean;
	method?: 'totp' | 'email';
	user?: UserDTO;
}

export interface TOTPSetupResponse {
	secret: string;
	provisioning_uri: string;
}

export interface EnableTwoFactorResponse {
	two_factor_enabled: boolean;
	method: 'totp' | 'email';
	backup_codes: string[];
}

export class ApiError extends Error {
	status: number;
	code?: string;
	fields?: Record<string, string>;

	constructor(message: string, status: number, code?: string, fields?: Record<string, string>) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
		this.fields = fields;
	}
}

const API_BASE = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.replace(/\/$/, '') ?? '';

let activeWorkspaceHeader: string | null = null;

export function setActiveWorkspaceHeader(workspaceId: string | null) {
	activeWorkspaceHeader = workspaceId;
}

export function resolveAvatarUrl(avatarUrl: string | null | undefined): string | null {
	if (!avatarUrl) return null;
	if (
		avatarUrl.startsWith('http://') ||
		avatarUrl.startsWith('https://') ||
		avatarUrl.startsWith('data:') ||
		avatarUrl.startsWith('blob:')
	) {
		return avatarUrl;
	}
	return `${API_BASE}${avatarUrl.startsWith('/') ? '' : '/'}${avatarUrl}`;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
	const headers = new Headers(init.headers);
	if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}
	if (activeWorkspaceHeader && !headers.has('X-Workspace-ID')) {
		headers.set('X-Workspace-ID', activeWorkspaceHeader);
	}

	const resp = await fetch(`${API_BASE}${path}`, {
		...init,
		headers,
		credentials: 'include'
	});

	const contentType = resp.headers.get('content-type') ?? '';
	const isJson = contentType.includes('application/json');
	const payload = isJson ? await resp.json().catch(() => null) : null;

	if (!resp.ok) {
		const msg =
			(payload && typeof payload.error === 'string' && payload.error) ||
			`Request failed (${resp.status})`;
		const code = payload && typeof payload.code === 'string' ? payload.code : undefined;
		const fields =
			payload && payload.fields && typeof payload.fields === 'object'
				? (payload.fields as Record<string, string>)
				: undefined;
		throw new ApiError(msg, resp.status, code, fields);
	}

	return payload as T;
}

export const authApi = {
	getProviders() {
		return request<{ providers: string[] }>('/api/v1/auth/providers');
	},
	me() {
		return request<MeResponse>('/api/v1/auth/me');
	},
	listWorkspaces() {
		return request<{ workspaces: WorkspaceDTO[] }>('/api/v1/workspaces');
	},
	createWorkspace(body: { name: string; slug?: string }) {
		return request<{ workspace: WorkspaceDTO }>('/api/v1/workspaces', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	switchWorkspace(workspaceId: string) {
		return request<{
			workspace: WorkspaceDTO;
			role: UserRole;
			permissions: string[];
			plan: string;
			max_users: number;
		}>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/switch`, {
			method: 'POST'
		});
	},
	exitWorkspace() {
		return request<{ status: string }>('/api/v1/workspaces/exit', {
			method: 'POST'
		});
	},
	listPendingInvites() {
		return request<{ invites: PendingWorkspaceInviteDTO[] }>('/api/v1/workspaces/invites');
	},
	acceptWorkspaceInvite(workspaceId: string) {
		return request<{ workspace: WorkspaceDTO }>(
			`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/accept`,
			{
				method: 'POST'
			}
		);
	},
	declineWorkspaceInvite(workspaceId: string) {
		return request<{ status: string }>(
			`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/decline`,
			{
				method: 'POST'
			}
		);
	},
	leaveWorkspace(workspaceId: string) {
		return request<{ status: string }>(
			`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/leave`,
			{
				method: 'POST'
			}
		);
	},
	updateWorkspace(workspaceId: string, name: string, slug: string) {
		return request<{ workspace: WorkspaceDTO }>(
			`/api/v1/workspaces/${encodeURIComponent(workspaceId)}`,
			{
				method: 'PATCH',
				body: JSON.stringify({ name, slug })
			}
		);
	},
	deleteWorkspace(workspaceId: string) {
		return request<{ status: string }>(
			`/api/v1/workspaces/${encodeURIComponent(workspaceId)}`,
			{
				method: 'DELETE'
			}
		);
	},
	transferWorkspaceOwnership(workspaceId: string, userId: string) {
		return request<{ workspace: WorkspaceDTO }>(
			`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/transfer-ownership`,
			{
				method: 'POST',
				body: JSON.stringify({ user_id: userId })
			}
		);
	},
	platformListWorkspaces() {
		return request<{ workspaces: WorkspaceDTO[] }>('/api/v1/platform/workspaces');
	},
	platformUpdateWorkspacePlan(
		workspaceId: string,
		body: {
			plan: WorkspacePlan;
			max_users_override?: number | null;
			max_products_override?: number | null;
			max_licenses_override?: number | null;
			override_expires_at?: string | null;
			override_note?: string;
		}
	) {
		return request<{ workspace: WorkspaceDTO }>(
			`/api/v1/platform/workspaces/${encodeURIComponent(workspaceId)}/plan`,
			{
				method: 'PATCH',
				body: JSON.stringify(body)
			}
		);
	},
	platformSuspendWorkspace(workspaceId: string, reason: string) {
		return request<{ workspace: WorkspaceDTO }>(
			`/api/v1/platform/workspaces/${encodeURIComponent(workspaceId)}/suspend`,
			{
				method: 'POST',
				body: JSON.stringify({ reason })
			}
		);
	},
	platformActivateWorkspace(workspaceId: string) {
		return request<{ workspace: WorkspaceDTO }>(
			`/api/v1/platform/workspaces/${encodeURIComponent(workspaceId)}/activate`,
			{
				method: 'POST'
			}
		);
	},
	platformListUsers() {
		return request<{ users: PlatformUserDTO[] }>('/api/v1/platform/users');
	},
	platformGetUserSuspendImpact(userId: string) {
		return request<GlobalSuspendImpactDTO>(
			`/api/v1/platform/users/${encodeURIComponent(userId)}/suspend-impact`
		);
	},
	platformSuspendUser(userId: string, reason: string) {
		return request<{ user: PlatformUserDTO }>(
			`/api/v1/platform/users/${encodeURIComponent(userId)}/suspend`,
			{
				method: 'POST',
				body: JSON.stringify({ reason })
			}
		);
	},
	platformActivateUser(userId: string) {
		return request<{ user: PlatformUserDTO }>(
			`/api/v1/platform/users/${encodeURIComponent(userId)}/activate`,
			{
				method: 'POST'
			}
		);
	},
	listUsers() {
		return request<{ users: AdminUserDTO[] }>('/api/v1/admin/users');
	},
	createUser(body: { email: string; name?: string; role: 'admin' | 'member' | 'viewer' }) {
		return request<{ message: string; user: AdminUserDTO }>('/api/v1/admin/users', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	removeUser(userId: string, reason?: string) {
		const trimmed = reason?.trim();
		return request<{ status: string }>(`/api/v1/admin/users/${encodeURIComponent(userId)}`, {
			method: 'DELETE',
			...(trimmed ? { body: JSON.stringify({ reason: trimmed }) } : {})
		});
	},
	updateUserRole(userId: string, role: 'admin' | 'member' | 'viewer') {
		return request<{ user: AdminUserDTO }>(
			`/api/v1/admin/users/${encodeURIComponent(userId)}/role`,
			{
				method: 'PATCH',
				body: JSON.stringify({ role })
			}
		);
	},
	suspendUser(userId: string, reason: string) {
		return request<{ user: AdminUserDTO }>(
			`/api/v1/admin/users/${encodeURIComponent(userId)}/suspend`,
			{
				method: 'POST',
				body: JSON.stringify({ reason })
			}
		);
	},
	activateUser(userId: string) {
		return request<{ user: AdminUserDTO }>(
			`/api/v1/admin/users/${encodeURIComponent(userId)}/activate`,
			{
				method: 'POST'
			}
		);
	},
	updateProfile(name: string) {
		return request<{ user: UserDTO }>('/api/v1/users/me', {
			method: 'PATCH',
			body: JSON.stringify({ name })
		});
	},
	uploadAvatar(file: Blob, filename = 'avatar.png') {
		const formData = new FormData();
		formData.append('avatar', file, filename);
		return request<{ user: UserDTO }>('/api/v1/users/me/avatar', {
			method: 'POST',
			body: formData
		});
	},
	deleteAvatar() {
		return request<{ user: UserDTO }>('/api/v1/users/me/avatar', {
			method: 'DELETE'
		});
	},
	register(email: string, password: string, name: string) {
		return request<{ message: string; user: UserDTO }>('/api/v1/auth/register', {
			method: 'POST',
			body: JSON.stringify({ email, password, name })
		});
	},
	verifyEmail(token: string) {
		return request<{ message: string }>('/api/v1/auth/verify-email', {
			method: 'POST',
			body: JSON.stringify({ token })
		});
	},
	resendVerification(email: string) {
		return request<{ message: string }>('/api/v1/auth/resend-verification', {
			method: 'POST',
			body: JSON.stringify({ email })
		});
	},
	login(email: string, password: string, rememberMe = false) {
		return request<LoginResponse>('/api/v1/auth/login', {
			method: 'POST',
			body: JSON.stringify({ email, password, remember_me: rememberMe })
		});
	},
	logout() {
		return request<{ message: string }>('/api/v1/auth/logout', {
			method: 'POST'
		});
	},
	logoutAll() {
		return request<{ message: string }>('/api/v1/auth/logout-all', {
			method: 'POST'
		});
	},
	forgotPassword(email: string) {
		return request<{ message: string }>('/api/v1/auth/forgot-password', {
			method: 'POST',
			body: JSON.stringify({ email })
		});
	},
	resetPassword(token: string, newPassword: string) {
		return request<{ message: string }>('/api/v1/auth/reset-password', {
			method: 'POST',
			body: JSON.stringify({ token, new_password: newPassword })
		});
	},
	changePassword(currentPassword: string, newPassword: string) {
		return request<{ message: string }>('/api/v1/auth/password/change', {
			method: 'POST',
			body: JSON.stringify({ current_password: currentPassword, new_password: newPassword })
		});
	},
	changeEmail(newEmail: string, currentPassword: string) {
		return request<{ message: string }>('/api/v1/auth/email/change', {
			method: 'POST',
			body: JSON.stringify({ new_email: newEmail, current_password: currentPassword })
		});
	},
	confirmEmailChange(token: string) {
		return request<{ message: string }>('/api/v1/auth/email/confirm', {
			method: 'POST',
			body: JSON.stringify({ token })
		});
	},
	setupTOTP() {
		return request<TOTPSetupResponse>('/api/v1/auth/2fa/totp/setup', {
			method: 'POST'
		});
	},
	verifyTOTP(body: { code?: string; backup_code?: string; remember_device?: boolean }) {
		return request<LoginResponse | EnableTwoFactorResponse>('/api/v1/auth/2fa/totp/verify', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	sendEmailOTP() {
		return request<{ message: string }>('/api/v1/auth/2fa/email/send', {
			method: 'POST'
		});
	},
	verifyEmailOTP(body: { code?: string; backup_code?: string; remember_device?: boolean }) {
		return request<LoginResponse | EnableTwoFactorResponse>('/api/v1/auth/2fa/email/verify', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	disableTwoFactor(body: { password?: string; code?: string; backup_code?: string }) {
		return request<{ message: string }>('/api/v1/auth/2fa/totp/disable', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	regenerateBackupCodes(body: { password?: string; code?: string }) {
		return request<{ backup_codes: string[] }>('/api/v1/auth/2fa/backup-codes/regenerate', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	requestAccountDeletion(body: {
		password?: string;
		email_otp?: string;
		code?: string;
		backup_code?: string;
	}) {
		return request<{ message: string }>('/api/v1/auth/account/delete/request', {
			method: 'POST',
			body: JSON.stringify(body)
		});
	},
	confirmAccountDeletion(token: string) {
		return request<{ message: string }>('/api/v1/auth/account/delete/confirm', {
			method: 'POST',
			body: JSON.stringify({ token })
		});
	},
	cancelAccountDeletion(email: string) {
		return request<{ message: string }>('/api/v1/auth/account/delete/cancel', {
			method: 'POST',
			body: JSON.stringify({ email })
		});
	},
	restoreAccount(token: string) {
		return request<{ message: string }>('/api/v1/auth/account/restore', {
			method: 'POST',
			body: JSON.stringify({ token })
		});
	},
	oauthStartUrl(provider: string) {
		return `${API_BASE}/api/v1/auth/oauth/${encodeURIComponent(provider)}`;
	},
	oauthCallbackUrl(search: string) {
		return `${API_BASE}/api/v1/auth/oauth/callback${search}`;
	}
};

