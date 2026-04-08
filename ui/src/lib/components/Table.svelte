<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Column {
		key: string;
		header: string;
		sortable?: boolean;
		render?: (item: Record<string, unknown>) => string;
	}

	interface Props {
		data: Record<string, unknown>[];
		columns: Column[];
		pageSize?: number;
		pageSizes?: number[];
		actions?: Snippet<[Record<string, unknown>]>;
	}

	let { data, columns, pageSizes = [10, 25, 50], actions }: Props = $props();

	let currentPage = $state(1);
	let sortKey = $state<string | null>(null);
	let sortDir = $state<'asc' | 'desc'>('asc');
	let effectivePageSize = $state(10);

	const totalPages = $derived(Math.ceil(data.length / effectivePageSize));

	const sortedData = $derived.by(() => {
		if (!sortKey) return data;
		return [...data].sort((a, b) => {
			const aVal = a[sortKey!];
			const bVal = b[sortKey!];
			if (aVal === bVal) return 0;
			if (aVal === null || aVal === undefined) return 1;
			if (bVal === null || bVal === undefined) return -1;
			const cmp = aVal < bVal ? -1 : 1;
			return sortDir === 'asc' ? cmp : -cmp;
		});
	});

	const paginatedData = $derived(
		sortedData.slice((currentPage - 1) * effectivePageSize, currentPage * effectivePageSize)
	);

	function handleSort(key: string) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'asc';
		}
	}

	function handlePageSizeChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		effectivePageSize = parseInt(target.value);
		currentPage = 1;
	}

	function goToPage(page: number) {
		if (page >= 1 && page <= totalPages) {
			currentPage = page;
		}
	}
</script>

<div class="overflow-hidden">
	<table class="min-w-full divide-y divide-gray-200">
		<thead class="bg-gray-50">
			<tr>
				{#each columns as column}
					<th
						class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider {column.sortable
							? 'cursor-pointer hover:text-gray-700 select-none'
							: ''}"
						onclick={() => column.sortable && handleSort(column.key)}
					>
						<div class="flex items-center gap-1">
							{column.header}
							{#if column.sortable && sortKey === column.key}
								<span class="text-gray-400">
									{#if sortDir === 'asc'}
										<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path
												stroke-linecap="round"
												stroke-linejoin="round"
												stroke-width="2"
												d="M5 15l7-7 7 7"
											/>
										</svg>
									{:else}
										<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path
												stroke-linecap="round"
												stroke-linejoin="round"
												stroke-width="2"
												d="M19 9l-7 7-7-7"
											/>
										</svg>
									{/if}
								</span>
							{/if}
						</div>
					</th>
				{/each}
				{#if actions}
					<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
				{/if}
			</tr>
		</thead>
		<tbody class="bg-white divide-y divide-gray-200">
			{#each paginatedData as item (item)}
				<tr class="hover:bg-gray-50">
					{#each columns as column}
						<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
							{#if column.render}
								{column.render(item)}
							{:else}
								{item[column.key]}
							{/if}
						</td>
					{/each}
					{#if actions}
						<td class="px-6 py-4 whitespace-nowrap text-right text-sm">
							{@render actions(item)}
						</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>
</div>

{#if totalPages > 1}
	<div class="flex items-center justify-between px-6 py-4 bg-white border-t border-gray-200">
		<div class="flex items-center gap-2">
			<span class="text-sm text-gray-700">Rows per page:</span>
			<select
				value={effectivePageSize}
				onchange={handlePageSizeChange}
				class="border border-gray-300 rounded px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-gray-500"
			>
				{#each pageSizes as size}
					<option value={size}>{size}</option>
				{/each}
			</select>
		</div>
		<div class="flex items-center gap-2">
			<span class="text-sm text-gray-700">
				{(currentPage - 1) * effectivePageSize + 1}-{Math.min(
					currentPage * effectivePageSize,
					data.length
				)} of {data.length}
			</span>
			<div class="flex gap-1">
				<button
					type="button"
					class="p-1 rounded hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
					disabled={currentPage === 1}
					onclick={() => goToPage(currentPage - 1)}
					aria-label="Previous page"
				>
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M15 19l-7-7 7-7"
						/>
					</svg>
				</button>
				<button
					type="button"
					class="p-1 rounded hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
					disabled={currentPage === totalPages}
					onclick={() => goToPage(currentPage + 1)}
					aria-label="Next page"
				>
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M9 5l7 7-7 7"
						/>
					</svg>
				</button>
			</div>
		</div>
	</div>
{/if}
