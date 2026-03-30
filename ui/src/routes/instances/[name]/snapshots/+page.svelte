<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api } from '$lib/services/api';
	import type { Snapshot, InstanceState, SnapshotResponse } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Table from '$lib/components/Table.svelte';
	import Modal from '$lib/components/Modal.svelte';

	let snapshots = $state<Snapshot[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let creating = $state(false);
	let restoring = $state<string | null>(null);
	let snapshotName = $state('');
	let snapshotComment = $state('');

	let instanceState = $state<InstanceState | null>(null);
	let showStopModal = $state(false);
	let showSuccessModal = $state(false);
	let successResponse = $state<SnapshotResponse | null>(null);

	const name = $derived($page.params.name);

	async function loadSnapshots() {
		loading = true;
		error = null;
		try {
			const data = await api.getSnapshots(name);
			snapshots = data.snapshots;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load snapshots';
		} finally {
			loading = false;
		}
	}

	async function checkInstanceState() {
		try {
			instanceState = await api.getInstanceState(name);
		} catch {
			instanceState = null;
		}
	}

	onMount(() => {
		loadSnapshots();
		checkInstanceState();
	});

	async function handleCreateSnapshot() {
		if (!snapshotName.trim()) {
			error = 'Snapshot name is required';
			return;
		}

		await checkInstanceState();

		if (instanceState?.state === 'Running') {
			showStopModal = true;
			return;
		}

		await createSnapshot();
	}

	async function createSnapshot() {
		showStopModal = false;
		creating = true;
		error = null;
		try {
			const response = await api.createSnapshot(name, { name: snapshotName, comment: snapshotComment });
			snapshotName = '';
			snapshotComment = '';
			
			successResponse = response;
			showSuccessModal = true;
			
			await loadSnapshots();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create snapshot';
		} finally {
			creating = false;
		}
	}

	async function handleConfirmStopAndSnapshot() {
		try {
			await api.stopInstance(name);
			await createSnapshot();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to stop instance';
			creating = false;
		}
	}

	async function handleRestore(snapshotNameVal: string) {
		restoring = snapshotNameVal;
		error = null;
		try {
			await api.restoreSnapshot(name, snapshotNameVal);
			alert('Snapshot restored successfully');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to restore snapshot';
		} finally {
			restoring = null;
		}
	}

	async function handleDelete(snapshotNameVal: string) {
		if (confirm(`Delete snapshot "${snapshotNameVal}"?`)) {
			error = null;
			try {
				await api.deleteSnapshot(name, snapshotNameVal);
				await loadSnapshots();
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to delete snapshot';
			}
		}
	}

	function formatSize(bytes?: number): string {
		if (!bytes) return '-';
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
		return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
	}
</script>

<svelte:head>
	<title>Snapshots - {name} - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-4">
			<a href="/instances/{name}" class="text-gray-500 hover:text-gray-700">← Back</a>
			<h2 class="text-2xl font-bold text-gray-900">Snapshots: {name}</h2>
			{#if instanceState}
				<span class="px-2 py-1 text-xs font-medium rounded {instanceState.state === 'Running' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'}">
					{instanceState.state}
				</span>
			{/if}
		</div>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 rounded p-4">
			<p class="text-sm text-red-600">{error}</p>
		</div>
	{/if}

	<Card>
		<h3 class="text-sm font-medium text-gray-500 mb-3">Create Snapshot</h3>
		<div class="flex gap-2">
			<input
				type="text"
				placeholder="Snapshot name"
				bind:value={snapshotName}
				class="flex-1 px-3 py-2 border border-gray-300 rounded text-sm"
			/>
			<input
				type="text"
				placeholder="Comment (optional)"
				bind:value={snapshotComment}
				class="flex-1 px-3 py-2 border border-gray-300 rounded text-sm"
			/>
			<Button variant="primary" onclick={handleCreateSnapshot} disabled={creating}>
				{creating ? 'Creating...' : 'Create'}
			</Button>
		</div>
	</Card>

	{#snippet actions(snapshot: Snapshot)}
		<div class="flex gap-2 justify-end">
			<Button
				variant="secondary"
				size="sm"
				onclick={() => handleRestore(snapshot.name)}
				disabled={restoring === snapshot.name}
			>
				{restoring === snapshot.name ? 'Restoring...' : 'Restore'}
			</Button>
			<Button variant="danger" size="sm" onclick={() => handleDelete(snapshot.name)}>
				Delete
			</Button>
		</div>
	{/snippet}

	{#if loading}
		<div class="py-12">
			<Spinner size="lg" />
		</div>
	{:else if snapshots.length === 0}
		<div class="text-center py-12 bg-white rounded border border-gray-200">
			<p class="text-gray-500">No snapshots found</p>
		</div>
	{:else}
		<Table
			data={snapshots}
			columns={[
				{ key: 'name', header: 'Name', sortable: true },
				{ key: 'created_at', header: 'Created', sortable: true, render: (s) => s.created_at ? new Date(s.created_at).toLocaleString() : '-' },
				{ key: 'comment', header: 'Comment', sortable: true, render: (s) => s.comment || '-' },
				{ key: 'disk_size', header: 'Disk Size', sortable: true, render: (s) => formatSize(s.disk_size) }
			]}
			{actions}
		/>
	{/if}
</div>

<Modal open={showStopModal} title="Instance is Running" onclose={() => showStopModal = false}>
	<div class="space-y-4">
		<p class="text-gray-600">
			The instance <strong>"{name}"</strong> is currently <strong>Running</strong>.
		</p>
		<p class="text-gray-600">
			Snapshots can only be created on stopped instances. Would you like to stop the instance and create a snapshot?
		</p>
		<div class="bg-yellow-50 border border-yellow-200 rounded p-3 text-sm text-yellow-800">
			<strong>Note:</strong> The instance will be stopped, snapshot will be created, and then the instance will remain stopped. 
			You can start it again after the snapshot is created.
		</div>
	</div>

	{#snippet footer()}
		<div class="flex gap-2 justify-end">
			<Button variant="secondary" onclick={() => showStopModal = false}>
				Cancel
			</Button>
			<Button variant="primary" onclick={handleConfirmStopAndSnapshot} disabled={creating}>
				{creating ? 'Stopping...' : 'Stop & Create Snapshot'}
			</Button>
		</div>
	{/snippet}
</Modal>

<Modal open={showSuccessModal} title="Snapshot Created" size="sm" onclose={() => showSuccessModal = false}>
	{#if successResponse}
		<div class="space-y-4">
			<div class="text-green-600 font-medium">✅ Snapshot created successfully!</div>
			
			<div class="bg-gray-50 rounded p-3 space-y-1 text-sm">
				<p><strong>Instance:</strong> {successResponse.instance_name}</p>
				<p><strong>Snapshot:</strong> {successResponse.snapshot_name}</p>
			</div>

			{#if successResponse.instance_stopped}
				<div class="bg-yellow-50 border border-yellow-200 rounded p-3 text-sm text-yellow-800">
					<strong>Instance remains stopped.</strong> Go to the instance page to start it when ready.
				</div>
			{/if}

			<div class="text-gray-600 text-sm">
				<strong>To restore this snapshot:</strong>
				<ul class="list-disc ml-4 mt-1">
					<li>Go to the instance page</li>
					<li>Click on "Snapshots"</li>
					<li>Click "Restore" on the snapshot you want to restore</li>
				</ul>
			</div>
		</div>
	{/if}

	{#snippet footer()}
		<Button variant="primary" onclick={() => showSuccessModal = false}>
			OK
		</Button>
	{/snippet}
</Modal>
