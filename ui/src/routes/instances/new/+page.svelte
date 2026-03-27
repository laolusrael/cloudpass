<script lang="ts">
	import { instances } from '$lib/stores/instances';
	import { goto } from '$app/navigation';
	import type { CreateInstanceRequest } from '$lib/types';
	import InstanceForm from '$lib/components/InstanceForm.svelte';
	import Card from '$lib/components/Card.svelte';

	let loading = $state(false);
	let error = $state<string | null>(null);

	async function handleSubmit(data: CreateInstanceRequest) {
		loading = true;
		error = null;
		try {
			await instances.create(data);
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create instance';
		} finally {
			loading = false;
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

	<Card>
		<InstanceForm onSubmit={handleSubmit} onCancel={handleCancel} {loading} />
	</Card>
</div>
