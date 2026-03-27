<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { instances } from '$lib/stores/instances';
	import { notifications } from '$lib/stores/notifications';
	import InstanceCard from '$lib/components/InstanceCard.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let loading = $state(true);
	let error = $state<string | null>(null);
	let refreshInterval: ReturnType<typeof setInterval>;

	onMount(() => {
		instances.refresh();
		refreshInterval = setInterval(() => {
			instances.refresh();
		}, 10000);
	});

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});

	instances.loading.subscribe((l) => (loading = l));
	instances.error.subscribe((e) => (error = e));
	instances.subscribe(() => (loading = false));

	async function handleStart(name: string) {
		try {
			await instances.start(name);
			notifications.success(`Instance "${name}" started`);
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to start');
		}
	}

	async function handleStop(name: string) {
		try {
			await instances.stop(name);
			notifications.success(`Instance "${name}" stopped`);
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to stop');
		}
	}

	async function handleRestart(name: string) {
		try {
			await instances.restart(name);
			notifications.success(`Instance "${name}" restarted`);
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to restart');
		}
	}

	async function handleDelete(name: string) {
		if (confirm(`Are you sure you want to delete "${name}"?`)) {
			try {
				await instances.delete(name);
				notifications.success(`Instance "${name}" deleted`);
			} catch (e) {
				notifications.error(e instanceof Error ? e.message : 'Failed to delete');
			}
		}
	}
</script>

<svelte:head>
	<title>Dashboard - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-2xl font-bold text-gray-900">Instances</h2>
			<p class="mt-1 text-sm text-gray-500">Manage your Multipass virtual machines</p>
		</div>
		<div class="flex gap-3">
			<Button variant="secondary" onclick={() => instances.refresh()}>Refresh</Button>
			<Button variant="primary" onclick={() => (window.location.href = '/instances/new')}>
				New Instance
			</Button>
		</div>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 rounded p-4">
			<p class="text-sm text-red-600">{error}</p>
		</div>
	{/if}

	{#if loading}
		<div class="py-12">
			<Spinner size="lg" />
		</div>
	{:else if $instances.length === 0}
		<div class="text-center py-12 bg-white rounded border border-gray-200">
			<p class="text-gray-500">No instances found</p>
			<p class="text-sm text-gray-400 mt-1">Create your first instance to get started</p>
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
