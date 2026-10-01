<script lang="ts">
	import { onMount } from 'svelte';
	import { api, NetworkInUseError } from '$lib/services/api';
	import { notifications } from '$lib/stores/notifications';
	import { networks } from '$lib/stores/networks';
	import type { Network } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import Input from '$lib/components/Input.svelte';
	import Select from '$lib/components/Select.svelte';

	const unverifiedStore = networks.unverified;
	const loadingStore = networks.loading;
	const loadErrorStore = networks.error;

	let showCreateModal = $state(false);
	let newNetworkName = $state('');
	let newNetworkMode = $state('');
	let newNetworkMac = $state('');
	let creating = $state(false);
	let actionError = $state<string | null>(null);

	let pendingDelete = $state<string | null>(null);
	let deleting = $state(false);
	let actingName = $state<string | null>(null);

	const modeOptions = [
		{ value: '', label: 'Auto (daemon default)' },
		{ value: 'manual', label: 'Manual' }
	];

	onMount(() => {
		networks.refresh();
	});

	function closeCreateModal() {
		if (creating) return;
		showCreateModal = false;
		newNetworkName = '';
		newNetworkMode = '';
		newNetworkMac = '';
	}

	async function handleCreate() {
		if (!newNetworkName.trim()) {
			notifications.error('Network name is required');
			return;
		}
		if (newNetworkMac.trim() && !/^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/.test(newNetworkMac.trim())) {
			notifications.error('MAC address must look like aa:bb:cc:dd:ee:ff');
			return;
		}
		creating = true;
		actionError = null;
		try {
			await api.createNetwork(
				newNetworkName.trim(),
				newNetworkMode || undefined,
				newNetworkMac.trim() || undefined
			);
			notifications.success(`Network "${newNetworkName.trim()}" created`);
			closeCreateModal();
			await networks.refresh();
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to create network';
		} finally {
			creating = false;
		}
	}

	function openDeleteModal(name: string) {
		pendingDelete = name;
		actionError = null;
	}

	async function handleDelete() {
		if (!pendingDelete) return;
		const name = pendingDelete;
		deleting = true;
		actionError = null;
		try {
			await api.deleteNetwork(name);
			notifications.success(`Network "${name}" deleted`);
			pendingDelete = null;
			await networks.refresh();
		} catch (e) {
			if (e instanceof NetworkInUseError) {
				actionError = `Cannot delete "${name}": in use by ${e.usedBy.join(', ')}.`;
				await networks.refresh();
			} else {
				actionError = e instanceof Error ? e.message : 'Failed to delete network';
			}
		} finally {
			deleting = false;
		}
	}

	async function handleClaim(name: string) {
		actingName = name;
		actionError = null;
		try {
			await api.claimNetwork(name);
			notifications.success(`Network "${name}" is now managed by CloudPass`);
			await networks.refresh();
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to claim network';
		} finally {
			actingName = null;
		}
	}

	async function handleUnclaim(name: string) {
		actingName = name;
		actionError = null;
		try {
			await api.unclaimNetwork(name);
			notifications.info(`Network "${name}" is no longer managed by CloudPass`);
			await networks.refresh();
		} catch (e) {
			actionError = e instanceof Error ? e.message : 'Failed to unclaim network';
		} finally {
			actingName = null;
		}
	}

	function usedBy(network: Network): string[] {
		return network.used_by ?? [];
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
			<Button variant="secondary" onclick={() => networks.refresh()}>Refresh</Button>
			<Button variant="primary" onclick={() => (showCreateModal = true)}>Create Network</Button>
		</div>
	</div>

	{#if $loadErrorStore}
		<div class="bg-red-50 border border-red-200 rounded p-4">
			<p class="text-sm text-red-600">{$loadErrorStore}</p>
		</div>
	{/if}

	{#if actionError}
		<div class="bg-red-50 border border-red-200 rounded p-4">
			<p class="text-sm text-red-600">{actionError}</p>
		</div>
	{/if}

	{#if $unverifiedStore.length > 0}
		<div class="bg-yellow-50 border border-yellow-200 rounded p-4">
			<p class="text-sm text-yellow-800">
				Instances with unknown network attachments: {$unverifiedStore.join(', ')}. These were
				created before tracking began, outside CloudPass, or are stopped. Deleting a network
				while they exist shows an extra warning; the daemon makes the final call.
			</p>
		</div>
	{/if}

	{#if $loadingStore}
		<div class="py-12">
			<Spinner size="lg" />
		</div>
	{:else if $networks.length === 0}
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
			{#each $networks as network (network.name)}
				{@const users = usedBy(network)}
				<Card>
					<div class="flex items-center justify-between gap-4">
						<div class="min-w-0">
							<div class="flex items-center gap-2">
								<h3 class="font-medium text-gray-900">{network.name}</h3>
								{#if network.managed}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
									>
										Managed
									</span>
								{:else}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
									>
										External
									</span>
								{/if}
							</div>
							<p class="text-sm text-gray-500">
								{network.description ||
									(network.managed ? 'Managed by CloudPass' : 'External — not managed by CloudPass')}
							</p>
							<div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm">
								<span class="text-gray-600">Type: {network.type}</span>
								{#if network.ipv4}
									<span class="text-gray-600">IPv4: {network.ipv4}</span>
								{/if}
								{#if users.length > 0}
									<span class="text-gray-900 font-medium">In use by: {users.join(', ')}</span>
								{/if}
							</div>
						</div>
						<div class="flex shrink-0 gap-2">
							{#if network.managed}
								{#if users.length === 0}
									<Button variant="danger" onclick={() => openDeleteModal(network.name)}>
										Delete
									</Button>
								{:else}
									<span title={`In use by ${users.join(', ')}`}>
										<Button variant="danger" disabled>Delete</Button>
									</span>
								{/if}
								<Button
									variant="secondary"
									onclick={() => handleUnclaim(network.name)}
									disabled={actingName === network.name}
								>
									{actingName === network.name ? 'Working…' : 'Unclaim'}
								</Button>
							{:else}
								<Button
									variant="secondary"
									onclick={() => handleClaim(network.name)}
									disabled={actingName === network.name}
								>
									{actingName === network.name ? 'Working…' : 'Claim as managed'}
								</Button>
							{/if}
						</div>
					</div>
				</Card>
			{/each}
		</div>
	{/if}
</div>

{#snippet createBody()}
	<div class="space-y-4">
		<Input label="Network Name" placeholder="my-network" bind:value={newNetworkName} required />
		<Select label="Mode" options={modeOptions} bind:value={newNetworkMode} />
		<p class="text-xs text-gray-500">
			Auto lets the daemon choose; Manual leaves address assignment to the guest.
		</p>
		<Input label="MAC Address (optional)" placeholder="aa:bb:cc:dd:ee:ff" bind:value={newNetworkMac} />
	</div>
{/snippet}

{#snippet createFooter()}
	<Button variant="secondary" onclick={closeCreateModal} disabled={creating}>Cancel</Button>
	<Button variant="primary" onclick={handleCreate} disabled={creating}>
		{creating ? 'Creating...' : 'Create'}
	</Button>
{/snippet}

<Modal open={showCreateModal} title="Create Network" onclose={closeCreateModal}>
	{@render createBody()}
	{@render createFooter()}
</Modal>

{#snippet deleteBody()}
	<p class="text-sm text-gray-600">
		Delete network "{pendingDelete}"? This cannot be undone.
		{#if $unverifiedStore.length > 0}
			<span class="text-yellow-800">
				Note: {$unverifiedStore.join(', ')} have unknown attachments — the daemon will refuse
				if the network turns out to be in use.
			</span>
		{/if}
	</p>
{/snippet}

{#snippet deleteFooter()}
	<Button variant="secondary" onclick={() => (pendingDelete = null)} disabled={deleting}>
		Cancel
	</Button>
	<Button variant="danger" onclick={handleDelete} disabled={deleting}>
		{deleting ? 'Deleting...' : 'Delete'}
	</Button>
{/snippet}

<Modal open={pendingDelete !== null} title="Delete Network" onclose={() => (pendingDelete = null)}>
	{@render deleteBody()}
	{@render deleteFooter()}
</Modal>
