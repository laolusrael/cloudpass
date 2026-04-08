<script lang="ts">
	import { onMount } from 'svelte';

	interface Option {
		value: string;
		label: string;
	}

	interface Props {
		label?: string;
		options: Option[];
		value?: string;
		placeholder?: string;
		searchable?: boolean;
		required?: boolean;
		error?: string;
		disabled?: boolean;
		onchange?: (value: string) => void;
	}

	let {
		label = '',
		options = [],
		value = $bindable(''),
		placeholder = 'Select an option',
		searchable = false,
		required = false,
		error = '',
		disabled = false,
		onchange
	}: Props = $props();

	let isOpen = $state(false);
	let searchTerm = $state('');
	let selectEl: HTMLDivElement;
	let id = $state('');

	onMount(() => {
		id = `select-${Math.random().toString(36).slice(2, 9)}`;
	});

	const filteredOptions = $derived(
		searchable && searchTerm
			? options.filter(
					(opt) =>
						opt.label.toLowerCase().includes(searchTerm.toLowerCase()) ||
						opt.value.toLowerCase().includes(searchTerm.toLowerCase())
				)
			: options
	);

	const selectedLabel = $derived(options.find((opt) => opt.value === value)?.label || '');

	function handleSelect(option: Option) {
		value = option.value;
		isOpen = false;
		searchTerm = '';
		onchange?.(option.value);
	}

	function toggleDropdown() {
		if (!disabled) {
			isOpen = !isOpen;
			if (!isOpen) {
				searchTerm = '';
			}
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!isOpen) return;

		if (e.key === 'Escape') {
			isOpen = false;
			searchTerm = '';
		} else if (e.key === 'Enter' && filteredOptions.length > 0) {
			handleSelect(filteredOptions[0]);
		}
	}

	function handleClickOutside(e: MouseEvent) {
		if (selectEl && !selectEl.contains(e.target as Node)) {
			isOpen = false;
			searchTerm = '';
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<div class="w-full">
	{#if label}
		<label for={id} class="block text-sm font-medium text-gray-700 mb-1">
			{label}
			{#if required}
				<span class="text-red-500">*</span>
			{/if}
		</label>
	{/if}

	<div class="relative" bind:this={selectEl} {id}>
		<button
			type="button"
			onclick={toggleDropdown}
			{disabled}
			onkeydown={handleKeydown}
			class="w-full px-3 py-2 text-left border rounded bg-white flex items-center justify-between
				focus:outline-none focus:ring-2 focus:ring-gray-500 focus:border-transparent
				disabled:bg-gray-100 disabled:cursor-not-allowed
				{error ? 'border-red-500' : 'border-gray-300'}"
		>
			<span class={value ? 'text-gray-900' : 'text-gray-400'}>
				{selectedLabel || placeholder}
			</span>
			<svg
				class="w-5 h-5 text-gray-400 transition-transform {isOpen ? 'rotate-180' : ''}"
				fill="none"
				stroke="currentColor"
				viewBox="0 0 24 24"
			>
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
			</svg>
		</button>

		{#if isOpen}
			<div
				class="absolute z-10 w-full mt-1 bg-white border border-gray-200 rounded shadow-lg max-h-60 overflow-hidden"
			>
				{#if searchable}
					<div class="p-2 border-b border-gray-100">
						<input
							type="text"
							bind:value={searchTerm}
							placeholder="Search..."
							class="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-gray-500"
							onclick={(e) => e.stopPropagation()}
						/>
					</div>
				{/if}

				<ul class="overflow-y-auto max-h-40">
					{#if filteredOptions.length === 0}
						<li class="px-3 py-2 text-sm text-gray-500">No options found</li>
					{:else}
						{#each filteredOptions as option}
							<li>
								<button
									type="button"
									onclick={() => handleSelect(option)}
									onkeydown={handleKeydown}
									class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100
										{option.value === value ? 'bg-gray-100 font-medium' : ''}"
								>
									{option.label}
								</button>
							</li>
						{/each}
					{/if}
				</ul>
			</div>
		{/if}
	</div>

	{#if error}
		<p class="mt-1 text-sm text-red-600">{error}</p>
	{/if}
</div>
