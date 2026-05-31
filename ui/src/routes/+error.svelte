<script lang="ts">
	import type { PageData } from './$types';
	import Card from '$lib/components/Card.svelte';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>Error - CloudPass</title>
</svelte:head>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
	<Card class="max-w-md w-full">
		<div class="text-center">
			<h2 class="text-xl font-semibold text-gray-900 mb-2">
				{data.status ?? 'Error'}
			</h2>
			<p class="text-sm text-gray-600 mb-4">
				{data.message ?? 'An unexpected error occurred.'}
			</p>
			<a href="/" class="text-sm font-medium text-gray-700 hover:text-gray-900 underline">
				Return to Dashboard
			</a>
		</div>
	</Card>
</div>
