<script lang="ts">
	interface Props {
		label?: string;
		type?: string;
		placeholder?: string;
		value?: string;
		error?: string;
		required?: boolean;
		disabled?: boolean;
		oninput?: (e: Event) => void;
	}

	let {
		label = '',
		type = 'text',
		placeholder = '',
		value = $bindable(''),
		error = '',
		required = false,
		disabled = false,
		oninput
	}: Props = $props();

	const inputId = `input-${Math.random().toString(36).slice(2, 9)}`;
</script>

<div class="w-full">
	{#if label}
		<label for={inputId} class="block text-sm font-medium text-gray-700 mb-1">
			{label}
			{#if required}
				<span class="text-red-500">*</span>
			{/if}
		</label>
	{/if}
	<input
		id={inputId}
		{type}
		{placeholder}
		bind:value
		{required}
		{disabled}
		{oninput}
		class="w-full px-3 py-2 border rounded text-gray-700 placeholder-gray-400
			focus:outline-none focus:ring-2 focus:ring-gray-500 focus:border-transparent
			disabled:bg-gray-100 disabled:cursor-not-allowed
			{error ? 'border-red-500' : 'border-gray-300'}"
	/>
	{#if error}
		<p class="mt-1 text-sm text-red-600">{error}</p>
	{/if}
</div>
