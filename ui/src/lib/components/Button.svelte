<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'primary' | 'secondary' | 'danger';
		size?: 'sm' | 'md' | 'lg';
		disabled?: boolean;
		type?: 'button' | 'submit';
		onclick?: () => void;
		children: Snippet;
	}

	let {
		variant = 'primary',
		size = 'md',
		disabled = false,
		type = 'button',
		onclick,
		children
	}: Props = $props();

	const baseClasses =
		'font-medium rounded transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2';

	const variantClasses = {
		primary: 'bg-gray-700 text-white hover:bg-gray-600 focus:ring-gray-500',
		secondary: 'bg-gray-200 text-gray-700 hover:bg-gray-300 focus:ring-gray-400',
		danger: 'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500'
	};

	const sizeClasses = {
		sm: 'px-3 py-1.5 text-sm',
		md: 'px-4 py-2 text-sm',
		lg: 'px-6 py-3 text-base'
	};

	const disabledClasses = 'opacity-50 cursor-not-allowed';
</script>

<button
	{type}
	{disabled}
	{onclick}
	class="{baseClasses} {variantClasses[variant]} {sizeClasses[size]} {disabled
		? disabledClasses
		: ''}"
>
	{@render children()}
</button>
