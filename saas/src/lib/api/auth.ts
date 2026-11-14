export interface UserDTO {
	id: string;
	email: string;
	name: string;
	avatar_url: string | null;
	email_verified_at: string | null;
	status: 'active' | 'suspended';
	two_factor_enabled: boolean;
	two_factor_method: 'totp' | 'email' | null;
	has_password: boolean;
	last_login_at: string | null;
	created_at: string;
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

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
	const headers = new Headers(init.headers);
	if (init.body && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
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
		return request<{ user: UserDTO }>('/api/v1/auth/me');
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
	login(email: string, password: string) {
		return request<LoginResponse>('/api/v1/auth/login', {
			method: 'POST',
			body: JSON.stringify({ email, password })
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
	oauthStartUrl(provider: string) {
		return `${API_BASE}/api/v1/auth/oauth/${encodeURIComponent(provider)}`;
	}
};
