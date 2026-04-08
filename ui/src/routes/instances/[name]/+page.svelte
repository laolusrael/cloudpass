<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/services/api';
	import type { Instance } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let instance = $state<Instance | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showMountModal = $state(false);
	let mountSourcePath = $state('');
	let mountTargetPath = $state('');
	let mounting = $state(false);
	let showUploadModal = $state(false);
	let uploadTargetPath = $state('');
	let uploading = $state(false);
	let fileInput: HTMLInputElement;

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
				goto('/');
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to delete';
			}
		}
	}

	async function handleExport() {
		if (!instance) return;
		try {
			const result = await api.exportInstance(instance.name);
			alert(`Instance exported to: ${result.image_path}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to export';
		}
	}

	async function handleImport() {
		const imagePath = prompt('Enter image path to import:');
		if (!imagePath) return;
		const name = prompt('Enter instance name (optional):');
		try {
			await api.importInstance({ image_path: imagePath, name: name || undefined });
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to import';
		}
	}

	async function handleMount() {
		if (!instance || !mountSourcePath.trim() || !mountTargetPath.trim()) return;
		mounting = true;
		error = null;
		try {
			await api.mountInstance(instance.name, {
				source_path: mountSourcePath.trim(),
				target_path: mountTargetPath.trim()
			});
			showMountModal = false;
			mountSourcePath = '';
			mountTargetPath = '';
			await loadInstance();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to mount';
		} finally {
			mounting = false;
		}
	}

	async function handleUnmount(targetPath: string) {
		if (!instance) return;
		if (confirm(`Unmount "${targetPath}"?`)) {
			try {
				await api.unmountInstance(instance.name, targetPath);
				await loadInstance();
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to unmount';
			}
		}
	}

	async function handleUpload() {
		if (!instance || !fileInput?.files?.length) return;
		const file = fileInput.files[0];
		uploading = true;
		error = null;
		try {
			await api.uploadFile(instance.name, file, uploadTargetPath || undefined);
			showUploadModal = false;
			uploadTargetPath = '';
			fileInput.value = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to upload';
		} finally {
			uploading = false;
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
			<Button variant="secondary" onclick={loadInstance}>Refresh</Button>
			{#if instance}
				<a href="/instances/{name}/snapshots">
					<Button variant="secondary">Snapshots</Button>
				</a>
				{#if isRunning}
					<a href="/instances/{name}/terminal">
						<Button variant="secondary">Terminal</Button>
					</a>
					<Button variant="secondary" onclick={handleStop}>Stop</Button>
					<Button variant="secondary" onclick={handleRestart}>Restart</Button>
				{:else if isStopped}
					<Button variant="primary" onclick={handleStart}>Start</Button>
				{/if}
				<Button variant="secondary" onclick={handleExport}>Export</Button>
				<Button variant="secondary" onclick={handleImport}>Import</Button>
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
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-sm font-medium text-gray-500">Mounts</h3>
					{#if isRunning}
						<div class="flex gap-2">
							<Button variant="secondary" onclick={() => (showUploadModal = true)}
								>Upload File</Button
							>
							<Button variant="secondary" onclick={() => (showMountModal = true)}>Add Mount</Button>
						</div>
					{/if}
				</div>
				{#if instance.mounts && instance.mounts.length > 0}
					<ul class="space-y-2">
						{#each instance.mounts as mount}
							<li class="flex items-center justify-between text-sm">
								<span>
									<span class="font-mono">{mount.source}</span>
									<span class="text-gray-400"> → </span>
									<span class="font-mono">{mount.target}</span>
								</span>
								{#if isRunning}
									<Button variant="danger" onclick={() => handleUnmount(mount.target)}
										>Unmount</Button
									>
								{/if}
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

{#if showMountModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg p-6 w-full max-w-md">
			<h3 class="text-lg font-medium mb-4">Add Mount</h3>
			<p class="text-sm text-gray-500 mb-4">
				Enter the host path to mount into the instance. If the path does not exist, you will be
				prompted to create it.
			</p>
			<div class="space-y-4">
				<div>
					<label for="mount-source" class="block text-sm font-medium text-gray-700 mb-1">
						Host Path (source)
					</label>
					<input
						id="mount-source"
						type="text"
						bind:value={mountSourcePath}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder="/home/user/projects"
					/>
				</div>
				<div>
					<label for="mount-target" class="block text-sm font-medium text-gray-700 mb-1">
						Instance Path (target)
					</label>
					<input
						id="mount-target"
						type="text"
						bind:value={mountTargetPath}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder="/home/ubuntu/projects"
					/>
				</div>
			</div>
			{#if error}
				<div class="mt-4 p-3 bg-red-50 border border-red-200 rounded">
					<p class="text-sm text-red-600">{error}</p>
				</div>
			{/if}
			<div class="mt-6 flex justify-end gap-3">
				<Button variant="secondary" onclick={() => (showMountModal = false)}>Cancel</Button>
				<Button
					variant="primary"
					onclick={handleMount}
					disabled={mounting || !mountSourcePath.trim() || !mountTargetPath.trim()}
				>
					{mounting ? 'Mounting...' : 'Mount'}
				</Button>
			</div>
		</div>
	</div>
{/if}

<input type="file" bind:this={fileInput} onchange={() => (showUploadModal = true)} class="hidden" />

{#if showUploadModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg p-6 w-full max-w-md">
			<h3 class="text-lg font-medium mb-4">Upload File</h3>
			<p class="text-sm text-gray-500 mb-4">Select a file to upload to the instance.</p>
			<div class="space-y-4">
				<div>
					<label for="upload-target" class="block text-sm font-medium text-gray-700 mb-1">
						Target Path (optional)
					</label>
					<input
						id="upload-target"
						type="text"
						bind:value={uploadTargetPath}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder="/home/ubuntu/uploads"
					/>
				</div>
			</div>
			{#if error}
				<div class="mt-4 p-3 bg-red-50 border border-red-200 rounded">
					<p class="text-sm text-red-600">{error}</p>
				</div>
			{/if}
			<div class="mt-6 flex justify-end gap-3">
				<Button variant="secondary" onclick={() => (showUploadModal = false)}>Cancel</Button>
				<Button variant="primary" onclick={handleUpload} disabled={uploading}>
					{uploading ? 'Uploading...' : 'Upload'}
				</Button>
			</div>
		</div>
	</div>
{/if}
