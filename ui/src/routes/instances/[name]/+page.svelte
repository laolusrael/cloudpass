<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api } from '$lib/services/api';
	import type { Instance } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let instance = $state<Instance | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const name = $derived($page.params.name);

	async function loadInstance() {
		loading = true;
		error = null;
		try {
			instance = await api.getInstance(name);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load instance';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadInstance();
	});

	async function handleStart() {
		if (!instance) return;
		try {
			await api.startInstance(instance.name);
			await loadInstance();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to start';
		}
	}

	async function handleStop() {
		if (!instance) return;
		try {
			await api.stopInstance(instance.name);
			await loadInstance();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to stop';
		}
	}

	async function handleRestart() {
		if (!instance) return;
		try {
			await api.restartInstance(instance.name);
			await loadInstance();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to restart';
		}
	}

	async function handleDelete() {
		if (!instance) return;
		if (confirm(`Delete instance "${instance.name}"?`)) {
			try {
				await api.deleteInstance(instance.name);
				window.location.href = '/';
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to delete';
			}
		}
	}

	const isRunning = $derived(instance?.state === 'Running');
	const isStopped = $derived(instance?.state === 'Stopped');
</script>

<svelte:head>
	<title>{name} - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-4">
			<a href="/" class="text-gray-500 hover:text-gray-700">← Back</a>
			<h2 class="text-2xl font-bold text-gray-900">{name}</h2>
			{#if instance}
				<Badge state={instance.state} />
			{/if}
		</div>
		<div class="flex gap-2">
			{#if instance}
				{#if isRunning}
					<Button variant="secondary" onclick={handleStop}>Stop</Button>
					<Button variant="secondary" onclick={handleRestart}>Restart</Button>
				{:else if isStopped}
					<Button variant="primary" onclick={handleStart}>Start</Button>
				{/if}
				<Button variant="danger" onclick={handleDelete}>Delete</Button>
			{/if}
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
	{:else if instance}
		<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-3">General</h3>
				<dl class="space-y-2">
					<div class="flex justify-between">
						<dt class="text-gray-600">Name</dt>
						<dd class="font-medium">{instance.name}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-gray-600">State</dt>
						<dd><Badge state={instance.state} /></dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-gray-600">Image</dt>
						<dd class="font-medium">{instance.image || '-'}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-gray-600">Release</dt>
						<dd class="font-medium">{instance.release || '-'}</dd>
					</div>
				</dl>
			</Card>

			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-3">Resources</h3>
				<dl class="space-y-2">
					<div class="flex justify-between">
						<dt class="text-gray-600">CPU</dt>
						<dd class="font-medium">{instance.cpu}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-gray-600">Memory</dt>
						<dd class="font-medium">{instance.memory || '-'}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-gray-600">Disk</dt>
						<dd class="font-medium">{instance.disk || '-'}</dd>
					</div>
				</dl>
			</Card>

			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-3">Network</h3>
				{#if instance.ipv4 && instance.ipv4.length > 0}
					<dl class="space-y-2">
						{#each instance.ipv4 as ip}
							<div class="flex justify-between">
								<dt class="text-gray-600">IPv4</dt>
								<dd class="font-mono text-sm">{ip}</dd>
							</div>
						{/each}
						{#if instance.ipv6}
							{#each instance.ipv6 as ip}
								<div class="flex justify-between">
									<dt class="text-gray-600">IPv6</dt>
									<dd class="font-mono text-sm">{ip}</dd>
								</div>
							{/each}
						{/if}
					</dl>
				{:else}
					<p class="text-gray-500 text-sm">No network addresses</p>
				{/if}
			</Card>

			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-3">Mounts</h3>
				{#if instance.mounts && instance.mounts.length > 0}
					<ul class="space-y-2">
						{#each instance.mounts as mount}
							<li class="text-sm">
								<span class="font-mono">{mount.source}</span>
								<span class="text-gray-400"> → </span>
								<span class="font-mono">{mount.target}</span>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="text-gray-500 text-sm">No mounts</p>
				{/if}
			</Card>
		</div>
	{:else}
		<div class="text-center py-12 bg-white rounded border border-gray-200">
			<p class="text-gray-500">Instance not found</p>
		</div>
	{/if}
</div>
