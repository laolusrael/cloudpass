<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api } from '$lib/services/api';
	import type { Snapshot } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Table from '$lib/components/Table.svelte';

	let snapshots = $state<Snapshot[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let creating = $state(false);
	let restoring = $state<string | null>(null);
	let snapshotName = $state('');
	let snapshotComment = $state('');

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

	onMount(() => {
		loadSnapshots();
	});

	async function handleCreateSnapshot() {
		if (!snapshotName.trim()) {
			error = 'Snapshot name is required';
			return;
		}
		creating = true;
		error = null;
		try {
			await api.createSnapshot(name, { name: snapshotName, comment: snapshotComment });
			snapshotName = '';
			snapshotComment = '';
			await loadSnapshots();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create snapshot';
		} finally {
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
