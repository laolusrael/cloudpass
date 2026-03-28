<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/services/api';
	import { notifications } from '$lib/stores/notifications';
	import type { Network } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let networks = $state<Network[]>([]);
	let loading = $state(true);
	let showCreateModal = $state(false);
	let newNetworkName = $state('');
	let newNetworkMode = $state('');
	let creating = $state(false);

	async function loadNetworks() {
		loading = true;
		try {
			const data = await api.getNetworks();
			networks = data.networks;
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to load networks');
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadNetworks();
	});

	async function handleCreate() {
		if (!newNetworkName.trim()) {
			notifications.error('Network name is required');
			return;
		}
		creating = true;
		try {
			await api.createNetwork(newNetworkName.trim(), newNetworkMode || undefined);
			notifications.success(`Network "${newNetworkName}" created`);
			showCreateModal = false;
			newNetworkName = '';
			newNetworkMode = '';
			await loadNetworks();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to create network');
		} finally {
			creating = false;
		}
	}

	async function handleDelete(name: string) {
		if (confirm(`Are you sure you want to delete network "${name}"?`)) {
			try {
				await api.deleteNetwork(name);
				notifications.success(`Network "${name}" deleted`);
				await loadNetworks();
			} catch (e) {
				notifications.error(e instanceof Error ? e.message : 'Failed to delete network');
			}
		}
	}
</script>

<svelte:head>
	<title>Networks - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-2xl font-bold text-gray-900">Networks</h2>
			<p class="mt-1 text-sm text-gray-500">Manage custom network bridges</p>
		</div>
		<div class="flex gap-3">
			<Button variant="secondary" onclick={loadNetworks}>Refresh</Button>
			<Button variant="primary" onclick={() => (showCreateModal = true)}>Create Network</Button>
		</div>
	</div>

	{#if loading}
		<div class="py-12">
			<Spinner size="lg" />
		</div>
	{:else if networks.length === 0}
		<div class="text-center py-12 bg-white rounded border border-gray-200">
			<p class="text-gray-500">No custom networks found</p>
			<div class="mt-4">
				<Button variant="primary" onclick={() => (showCreateModal = true)}>
					Create First Network
				</Button>
			</div>
		</div>
	{:else}
		<div class="grid gap-4">
			{#each networks as network (network.name)}
				<Card>
					<div class="flex items-center justify-between">
						<div>
							<h3 class="font-medium text-gray-900">{network.name}</h3>
							<p class="text-sm text-gray-500">{network.description || 'No description'}</p>
							<div class="mt-2 flex gap-4 text-sm">
								<span class="text-gray-600">Type: {network.type}</span>
								{#if network.ipv4}
									<span class="text-gray-600">IPv4: {network.ipv4}</span>
								{/if}
							</div>
						</div>
						<Button variant="danger" onclick={() => handleDelete(network.name)}>Delete</Button>
					</div>
				</Card>
			{/each}
		</div>
	{/if}
</div>

{#if showCreateModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg p-6 w-full max-w-md">
			<h3 class="text-lg font-medium mb-4">Create Network</h3>
			<div class="space-y-4">
				<div>
					<label for="network-name" class="block text-sm font-medium text-gray-700 mb-1">Network Name</label>
					<input
						id="network-name"
						type="text"
						bind:value={newNetworkName}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder="my-network"
					/>
				</div>
				<div>
					<label for="network-mode" class="block text-sm font-medium text-gray-700 mb-1">Mode (optional)</label>
					<select
						id="network-mode"
						bind:value={newNetworkMode}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
					>
						<option value="">Auto</option>
						<option value="manual">Manual</option>
					</select>
				</div>
			</div>
			<div class="mt-6 flex justify-end gap-3">
				<Button variant="secondary" onclick={() => (showCreateModal = false)}>Cancel</Button>
				<Button variant="primary" onclick={handleCreate} disabled={creating}>
					{creating ? 'Creating...' : 'Create'}
				</Button>
			</div>
		</div>
	</div>
{/if}
