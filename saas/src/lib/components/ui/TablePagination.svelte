<script lang="ts">
	import { ChevronFirst, ChevronLast, ChevronLeft, ChevronRight } from '@lucide/svelte';

	interface Props {
		page: number;
		pageSize: number;
		totalItems: number;
		pageSizeOptions?: number[];
		ariaLabel?: string;
	}

	let {
		page = $bindable(),
		pageSize = $bindable(),
		totalItems,
		pageSizeOptions = [10, 25, 50, 100],
		ariaLabel = 'Table pagination'
	}: Props = $props();

	const selectId = $props.id();

	const totalPages = $derived(Math.max(1, Math.ceil(totalItems / pageSize)));
	const safePage = $derived(Math.min(Math.max(1, page), totalPages));
	const startRecord = $derived(totalItems === 0 ? 0 : (safePage - 1) * pageSize + 1);
	const endRecord = $derived(totalItems === 0 ? 0 : Math.min(totalItems, safePage * pageSize));

	type PageItem = number | 'ellipsis';

	const pageItems = $derived.by<PageItem[]>(() => {
		if (totalPages <= 5) {
			return Array.from({ length: totalPages }, (_, i) => i + 1);
		}

		const visible = new Set<number>([1, totalPages, safePage]);
		if (safePage - 1 >= 1) visible.add(safePage - 1);
		if (safePage + 1 <= totalPages) visible.add(safePage + 1);

		const sorted = [...visible].sort((a, b) => a - b);
		const items: PageItem[] = [];

		for (let i = 0; i < sorted.length; i++) {
			const curr = sorted[i];
			if (i > 0) {
				const prev = sorted[i - 1];
				if (curr - prev === 2) {
					items.push(prev + 1);
				} else if (curr - prev > 2) {
					items.push('ellipsis');
				}
			}
			items.push(curr);
		}

		return items;
	});

	function goToPage(target: number) {
		const clamped = Math.min(Math.max(1, target), totalPages);
		if (clamped !== page) {
			page = clamped;
		}
	}

	function handlePageSizeChange(e: Event) {
		const next = Number((e.currentTarget as HTMLSelectElement).value);
		if (Number.isFinite(next) && next > 0) {
			pageSize = next;
			page = 1;
		}
	}

	const navButtonClass =
		'inline-flex h-8 min-w-8 items-center justify-center rounded-md border-2 border-ink bg-surface px-2 font-mono text-xs font-bold text-ink shadow-brutal-xs transition-all duration-150 cursor-pointer select-none hover:bg-yellow hover:-translate-x-0.5 hover:-translate-y-0.5 hover:shadow-brutal-sm active:translate-x-0.5 active:translate-y-0.5 active:shadow-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt focus-visible:ring-offset-1 focus-visible:ring-offset-surface disabled:opacity-45 disabled:pointer-events-none disabled:shadow-none';

	const activePageButtonClass =
		'inline-flex h-8 min-w-8 items-center justify-center rounded-md border-2 border-ink bg-cobalt px-2 font-mono text-xs font-extrabold text-surface shadow-brutal-xs cursor-default select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt focus-visible:ring-offset-1 focus-visible:ring-offset-surface';
</script>

{#if totalItems > 0}
	<nav
		aria-label={ariaLabel}
		class="mt-4 flex flex-wrap items-center justify-between gap-x-4 gap-y-3 border-t-2 border-ink/15 pt-3.5 px-1 sm:px-2"
	>
		<!-- Left: Rows per page -->
		<div class="flex items-center gap-2">
			<label for={selectId} class="font-mono text-xs font-bold text-ink/75 whitespace-nowrap">
				Rows per page
			</label>
			<select
				id={selectId}
				value={String(pageSize)}
				onchange={handlePageSizeChange}
				class="h-8 rounded-md border-2 border-ink bg-cream px-2 font-mono text-xs font-bold text-ink shadow-brutal-xs cursor-pointer focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
			>
				{#each pageSizeOptions as opt (opt)}
					<option value={String(opt)}>{opt}</option>
				{/each}
			</select>
		</div>

		<!-- Center: Record range & total -->
		<p class="font-mono text-xs font-bold text-ink/75 tabular-nums">
			{startRecord}&ndash;{endRecord} of {totalItems}
		</p>

		<!-- Right: Page navigation -->
		<div class="flex w-full lg:w-auto flex-wrap items-center justify-center sm:justify-end gap-1.5">
			<button
				type="button"
				disabled={safePage <= 1}
				onclick={() => goToPage(1)}
				aria-label="First page"
				title="First page"
				class={navButtonClass}
			>
				<ChevronFirst class="h-3.5 w-3.5" aria-hidden="true" />
			</button>

			<button
				type="button"
				disabled={safePage <= 1}
				onclick={() => goToPage(safePage - 1)}
				aria-label="Previous page"
				title="Previous page"
				class={navButtonClass}
			>
				<ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />
			</button>

			{#each pageItems as item, idx (`${item}-${idx}`)}
				{#if item === 'ellipsis'}
					<span
						aria-hidden="true"
						class="inline-flex h-8 min-w-5 items-center justify-center px-0.5 font-mono text-xs font-bold text-ink/55 select-none"
					>
						&hellip;
					</span>
				{:else}
					<button
						type="button"
						onclick={() => goToPage(item)}
						aria-label={`Page ${item}`}
						aria-current={item === safePage ? 'page' : undefined}
						class={item === safePage ? activePageButtonClass : navButtonClass}
					>
						{item}
					</button>
				{/if}
			{/each}

			<button
				type="button"
				disabled={safePage >= totalPages}
				onclick={() => goToPage(safePage + 1)}
				aria-label="Next page"
				title="Next page"
				class={navButtonClass}
			>
				<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
			</button>

			<button
				type="button"
				disabled={safePage >= totalPages}
				onclick={() => goToPage(totalPages)}
				aria-label="Last page"
				title="Last page"
				class={navButtonClass}
			>
				<ChevronLast class="h-3.5 w-3.5" aria-hidden="true" />
			</button>
		</div>
	</nav>
{/if}

