class RouterStore {
	pathname = $state(typeof window !== 'undefined' ? window.location.pathname : '/login');
	search = $state(typeof window !== 'undefined' ? window.location.search : '');

	constructor() {
		if (typeof window !== 'undefined') {
			window.addEventListener('popstate', () => {
				this.pathname = window.location.pathname;
				this.search = window.location.search;
			});
		}
	}

	navigate(to: string, replace = false) {
		if (typeof window === 'undefined') return;
		const url = new URL(to, window.location.origin);
		if (replace) {
			window.history.replaceState({}, '', url.pathname + url.search);
		} else {
			window.history.pushState({}, '', url.pathname + url.search);
		}
		this.pathname = url.pathname;
		this.search = url.search;
	}

	queryParam(key: string): string {
		return new URLSearchParams(this.search).get(key) ?? '';
	}
}

export const router = new RouterStore();
