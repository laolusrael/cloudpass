<script lang="ts">
	import { instances } from '$lib/stores/instances';
	import InstanceCard from '$lib/components/InstanceCard.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import { onMount } from 'svelte';

	let loading = $state(true);

	onMount(() => {
		instances.refresh();
	});

	instances.loading.subscribe((l) => (loading = l));

	async function handleStart(name: string) {
		try {
			await instances.start(name);
		} catch (e) {
			console.error('Failed to start:', e);
		}
	}

	async function handleStop(name: string) {
		try {
			await instances.stop(name);
		} catch (e) {
			console.error('Failed to stop:', e);
		}
	}

	async function handleRestart(name: string) {
		try {
			await instances.restart(name);
		} catch (e) {
			console.error('Failed to restart:', e);
		}
	}

	async function handleDelete(name: string) {
		if (confirm(`Are you sure you want to delete "${name}"?`)) {
			try {
				await instances.delete(name);
			} catch (e) {
				console.error('Failed to delete:', e);
			}
		}
	}
</script>

<svelte:head>
	<title>Instances - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-2xl font-bold text-gray-900">Instances</h2>
			<p class="mt-1 text-sm text-gray-500">Manage all virtual machines</p>
		</div>
		<div class="flex gap-3">
			<Button variant="secondary" onclick={() => instances.refresh()}>Refresh</Button>
			<Button variant="primary" onclick={() => (window.location.href = '/instances/new')}>
				New Instance
			</Button>
		</div>
	</div>

	{#if loading}
		<div class="py-12">
			<Spinner size="lg" />
		</div>
	{:else if $instances.length === 0}
		<div class="text-center py-12 bg-white rounded border border-gray-200">
			<p class="text-gray-500">No instances found</p>
			<div class="mt-4">
				<Button variant="primary" onclick={() => (window.location.href = '/instances/new')}>
					Create First Instance
				</Button>
			</div>
		</div>
	{:else}
		<div class="grid gap-4">
			{#each $instances as instance (instance.name)}
				<InstanceCard
					{instance}
					onstart={() => handleStart(instance.name)}
					onstop={() => handleStop(instance.name)}
					onrestart={() => handleRestart(instance.name)}
					ondelete={() => handleDelete(instance.name)}
				/>
			{/each}
		</div>
	{/if}
</div>
