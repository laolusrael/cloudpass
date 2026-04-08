<script lang="ts">
	import { onMount } from 'svelte';

	interface Props {
		message: string;
		type?: 'success' | 'error' | 'info';
		onclose?: () => void;
	}

	let { message, type = 'info', onclose }: Props = $props();

	const colors = {
		success: 'bg-green-50 border-green-200 text-green-800',
		error: 'bg-red-50 border-red-200 text-red-800',
		info: 'bg-blue-50 border-blue-200 text-blue-800'
	};

	onMount(() => {
		const timer = setTimeout(() => {
			onclose?.();
		}, 5000);
		return () => clearTimeout(timer);
	});
</script>

<div class="fixed bottom-4 right-4 z-50 animate-slide-up">
	<div class="flex items-center gap-3 px-4 py-3 rounded border shadow-lg {colors[type]}">
		<p class="text-sm font-medium">{message}</p>
		<button onclick={onclose} class="text-lg leading-none opacity-60 hover:opacity-100"
			>&times;</button
		>
	</div>
</div>

<style>
	@keyframes slide-up {
		from {
			transform: translateY(100%);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}
	.animate-slide-up {
		animation: slide-up 0.3s ease-out;
	}
</style>
