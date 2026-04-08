<script lang="ts">
	import { instances } from '$lib/stores/instances';
	import { notifications } from '$lib/stores/notifications';
	import { goto } from '$app/navigation';
	import type { CreateInstanceRequest } from '$lib/types';
	import InstanceForm from '$lib/components/InstanceForm.svelte';
	import Card from '$lib/components/Card.svelte';

	let loading = $state(false);
	let error = $state<string | null>(null);
	let status = $state<string | null>(null);

	async function handleSubmit(data: CreateInstanceRequest) {
		loading = true;
		error = null;
		status = 'pending';
		try {
			await instances.createAsync(data, (jobStatus) => {
				if (jobStatus === 'pending') {
					status = 'Preparing instance creation...';
				} else if (jobStatus === 'running') {
					status =
						'Creating instance (this may take a few minutes for first-time image download)...';
				}
			});
			notifications.success(`Instance "${data.name || 'new instance'}" created successfully`);
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create instance';
			notifications.error(error);
		} finally {
			loading = false;
			status = null;
		}
	}

	function handleCancel() {
		goto('/');
	}
</script>

<svelte:head>
	<title>New Instance - CloudPass</title>
</svelte:head>

<div class="max-w-2xl mx-auto">
	<div class="mb-6">
		<h2 class="text-2xl font-bold text-gray-900">Create New Instance</h2>
		<p class="mt-1 text-sm text-gray-500">Configure and launch a new virtual machine</p>
	</div>

	{#if error}
		<div class="mb-6 bg-red-50 border border-red-200 rounded p-4">
			<p class="text-sm text-red-600">{error}</p>
		</div>
	{/if}

	{#if status}
		<div class="mb-6 bg-blue-50 border border-blue-200 rounded p-4">
			<div class="flex items-center gap-3">
				<div
					class="animate-spin h-5 w-5 border-2 border-blue-500 border-t-transparent rounded-full"
				></div>
				<p class="text-sm text-blue-600">{status}</p>
			</div>
		</div>
	{/if}

	<Card>
		<InstanceForm onSubmit={handleSubmit} onCancel={handleCancel} {loading} />
	</Card>
</div>
