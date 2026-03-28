<script lang="ts">
	import { page } from '$app/stores';
	import Terminal from '$lib/components/Terminal.svelte';

	const name = $derived($page.params.name);
</script>

<svelte:head>
	<title>Terminal - {name} - CloudPass</title>
</svelte:head>

<div class="h-[calc(100vh-8rem)]">
	<a href="/instances/{name}" class="inline-block mb-4 text-gray-500 hover:text-gray-700">
		← Back to {name}
	</a>
	<div class="h-full border border-gray-200 rounded overflow-hidden bg-[#1a1a1a]">
		<Terminal instanceName={name} />
	</div>
</div>
