<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/services/api';
	import type { Instance } from '$lib/types/instance';
	import Terminal from '$lib/components/Terminal.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	const name = $derived($page.params.name || '');

	let instance = $state<Instance | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const isRunning = $derived(instance?.state === 'Running');

	onMount(async () => {
		try {
			instance = await api.getInstance(name);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load instance';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Terminal - {name} - CloudPass</title>
</svelte:head>

<div class="h-[calc(100vh-8rem)]">
	<a href="/instances/{name}" class="inline-block mb-4 text-gray-500 hover:text-gray-700">
		← Back to {name}
	</a>

	{#if loading}
		<div class="flex items-center justify-center h-full">
			<Spinner size="lg" />
		</div>
	{:else if error}
		<div
			class="flex flex-col items-center justify-center h-full bg-white border border-gray-200 rounded p-8"
		>
			<p class="text-red-600 mb-4">{error}</p>
			<a href="/instances/{name}" class="text-gray-600 hover:text-gray-800">Return to instance</a>
		</div>
	{:else if !isRunning}
		<div
			class="flex flex-col items-center justify-center h-full bg-white border border-gray-200 rounded p-8"
		>
			<p class="text-gray-600 mb-4">Instance is not running</p>
			<p class="text-sm text-gray-500 mb-4">
				The terminal requires the instance to be in Running state.
			</p>
			<a href="/instances/{name}" class="text-gray-600 hover:text-gray-800">Return to instance</a>
		</div>
	{:else}
		<div class="h-full border border-gray-200 rounded overflow-hidden bg-[#1a1a1a]">
			<Terminal instanceName={name} />
		</div>
	{/if}
</div>
