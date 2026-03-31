<script lang="ts">
	import { onMount } from 'svelte';
	import { instances } from '$lib/stores/instances';
	import { jobs } from '$lib/stores/jobs';
	import { notifications } from '$lib/stores/notifications';
	import InstanceCard from '$lib/components/InstanceCard.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Badge from '$lib/components/Badge.svelte';

	let loading = $state(true);

	onMount(() => {
		instances.refresh();
		jobs.refresh();
	});

	instances.loading.subscribe((l) => (loading = l));

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
	<title>Instances - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-2xl font-bold text-gray-900">Instances</h2>
			<p class="mt-1 text-sm text-gray-500">Manage all virtual machines</p>
		</div>
		<div class="flex gap-3">
			<Button variant="secondary" onclick={() => { instances.refresh(); jobs.refresh(); }}>Refresh</Button>
			<Button variant="primary" onclick={() => (window.location.href = '/instances/new')}>
				New Instance
			</Button>
		</div>
	</div>

	{#if $jobs.length > 0}
		<div class="bg-white rounded border border-gray-200 p-4">
			<h3 class="text-lg font-semibold text-gray-900 mb-3">Creating</h3>
			<div class="space-y-2">
				{#each $jobs as job (job.id)}
					<div class="flex items-center justify-between py-2 px-3 bg-gray-50 rounded">
						<div class="flex items-center gap-3">
							<Spinner size="sm" />
							<span class="text-sm font-medium text-gray-900">{job.instance_name || job.id}</span>
							<Badge type={job.status === 'pending' ? 'info' : 'warning'}>
								{job.status}
							</Badge>
						</div>
						<span class="text-xs text-gray-500">
							Started {new Date(job.created_at).toLocaleTimeString()}
						</span>
					</div>
				{/each}
			</div>
		</div>
	{/if}

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
