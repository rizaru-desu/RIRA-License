<script lang="ts" module>
	export const MIN_PASSWORD_LENGTH = 6;
	export const RECOMMENDED_PASSWORD_LENGTH = 12;

	export type StrengthBand = 'Weak' | 'Fair' | 'Good' | 'Strong';

	export interface PasswordStrength {
		score: 0 | 1 | 2 | 3 | 4;
		label: StrengthBand;
		feedback: string;
	}

	export function evaluatePasswordStrength(password: string): PasswordStrength {
		const len = password.length;
		if (len === 0) {
			return {
				score: 0,
				label: 'Weak',
				feedback: 'Min 6 chars, 12+ recommended'
			};
		}

		const hasLower = /[a-z]/.test(password);
		const hasUpper = /[A-Z]/.test(password);
		const hasDigit = /\d/.test(password);
		const hasSymbol = /[^A-Za-z0-9]/.test(password);
		const variety = Number(hasLower) + Number(hasUpper) + Number(hasDigit) + Number(hasSymbol);

		if (len < MIN_PASSWORD_LENGTH) {
			return {
				score: 1,
				label: 'Weak',
				feedback: 'Too short — at least 6 characters required'
			};
		}

		if (len >= RECOMMENDED_PASSWORD_LENGTH && variety >= 3) {
			return {
				score: 4,
				label: 'Strong',
				feedback: 'Strong password — meets 12+ character recommendation'
			};
		}

		let score: 1 | 2 | 3 = 1;
		let label: StrengthBand = 'Weak';

		if (
			len >= RECOMMENDED_PASSWORD_LENGTH ||
			(len >= 8 && variety >= 3) ||
			(len >= 10 && variety >= 2)
		) {
			score = 3;
			label = 'Good';
		} else if (variety >= 2 || len >= 8) {
			score = 2;
			label = 'Fair';
		}

		let feedback = 'Longer is stronger — 12+ chars recommended';
		if (variety < 3) {
			if (!hasUpper) {
				feedback = 'Add an uppercase letter';
			} else if (!hasDigit) {
				feedback = 'Add a number';
			} else if (!hasSymbol) {
				feedback = 'Add a symbol';
			} else if (!hasLower) {
				feedback = 'Add a lowercase letter';
			}
		}

		return { score, label, feedback };
	}
</script>

<script lang="ts">
	interface Props {
		password?: string;
	}

	let { password = '' }: Props = $props();

	const strength = $derived(evaluatePasswordStrength(password));

	const fillClass = $derived.by(() => {
		if (password.length === 0) return 'w-0 bg-red-500';
		switch (strength.score) {
			case 0:
			case 1:
				return 'w-1/4 bg-red-500';
			case 2:
				return 'w-2/4 bg-orange-500';
			case 3:
				return 'w-3/4 bg-lime-500';
			case 4:
				return 'w-full bg-green-500';
		}
	});

	const badgeColorClass = $derived.by(() => {
		switch (strength.score) {
			case 0:
			case 1:
				return 'bg-red-500 text-surface';
			case 2:
				return 'bg-orange-500 text-ink';
			case 3:
				return 'bg-lime-500 text-ink';
			case 4:
				return 'bg-green-500 text-ink';
		}
	});
</script>

<div class="mt-2 space-y-1.5">
	<div
		role="progressbar"
		aria-label="Password strength"
		aria-valuemin={0}
		aria-valuemax={4}
		aria-valuenow={strength.score}
		aria-valuetext={`${strength.label}: ${strength.feedback}`}
		class="h-2.5 w-full overflow-hidden rounded-sm border-2 border-ink bg-cream-dark"
	>
		<div class="h-full transition-all duration-150 {fillClass}"></div>
	</div>

	<div class="flex flex-wrap items-center justify-between gap-2 text-xs">
		<span aria-live="polite" class="inline-flex items-center gap-1.5 font-heading font-bold text-ink">
			<span>Strength:</span>
			<span
				class="inline-block rounded-sm border border-ink px-1.5 py-0.5 text-[11px] uppercase tracking-wider {badgeColorClass}"
			>
				{strength.label}
			</span>
		</span>
		<span class="font-medium text-ink/80">{strength.feedback}</span>
	</div>
</div>

