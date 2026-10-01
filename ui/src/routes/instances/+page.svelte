<script lang="ts">
	import { onMount } from 'svelte';
	import { instances } from '$lib/stores/instances';
	import { jobs } from '$lib/stores/jobs';
	import { notifications } from '$lib/stores/notifications';
	import { api } from '$lib/services/api';
	import { validateSelectedFile, formatFileSize } from '$lib/validation/upload';
	import { waitForJob } from '$lib/utils/job-poller';
	import InstanceCard from '$lib/components/InstanceCard.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import Badge from '$lib/components/Badge.svelte';

	let loading = $state(true);
	let showImportModal = $state(false);
	let importTab = $state<'upload' | 'server'>('upload');
	let importFile = $state<File | null>(null);
	let importName = $state('');
	let importPath = $state('');
	let importMaxMB = $state(10240);
	let importing = $state(false);
	let importError = $state<string | null>(null);

	async function loadImageLimit() {
		try {
			const config = await api.getConfig();
			importMaxMB = config.upload.max_image_size_mb;
		} catch (e) {
			console.error('Failed to load image size limit:', e);
		}
	}

	onMount(() => {
		instances.refresh();
		jobs.refresh();
		loadImageLimit();
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

	function openImportModal() {
		importTab = 'upload';
		importFile = null;
		importName = '';
		importPath = '';
		importError = null;
		showImportModal = true;
	}

	function handleImportFileSelected(e: Event) {
		importError = null;
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0] ?? null;
		if (!file) {
			importFile = null;
			return;
		}
		const problem = validateSelectedFile(file, importMaxMB);
		if (problem) {
			importFile = null;
			input.value = '';
			importError = problem;
			return;
		}
		importFile = file;
	}

	const importReady = $derived(
		importTab === 'upload' ? importFile !== null : importPath.trim() !== ''
	);

	async function handleImport() {
		if (!importReady || importing) return;
		importing = true;
		importError = null;
		const name = importName.trim() || undefined;
		try {
			const job =
				importTab === 'upload' && importFile
					? await api.importInstanceFromFile(importFile, name, {
							idempotencyKey: crypto.randomUUID()
						})
					: await api.importInstanceAsync(
							{ image_path: importPath.trim(), name },
							{ idempotencyKey: crypto.randomUUID() }
						);
			showImportModal = false;
			notifications.info('Import started — you will be notified when ready.');
			try {
				const result = await waitForJob(job.id, { timeoutMs: 30 * 60 * 1000 });
				notifications.success(`Imported instance "${result || job.instance_name || 'new'}".`);
			} catch (e) {
				notifications.error(e instanceof Error ? e.message : 'Import failed');
			}
			await instances.refresh();
		} catch (e) {
			importError = e instanceof Error ? e.message : 'Failed to import';
		} finally {
			importing = false;
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
			<Button
				variant="secondary"
				onclick={() => {
					instances.refresh();
					jobs.refresh();
				}}>Refresh</Button
			>
			<Button variant="secondary" onclick={openImportModal}>Import</Button>
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
							<Badge state={job.status} />
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

{#if showImportModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg p-6 w-full max-w-md">
			<h3 class="text-lg font-medium mb-4">Import Instance</h3>
			<div class="flex gap-2 mb-4">
				<Button
					variant={importTab === 'upload' ? 'primary' : 'secondary'}
					onclick={() => (importTab = 'upload')}
				>
					Upload file
				</Button>
				<Button
					variant={importTab === 'server' ? 'primary' : 'secondary'}
					onclick={() => (importTab = 'server')}
				>
					Server path
				</Button>
			</div>
			<div class="space-y-4">
				{#if importTab === 'upload'}
					<div>
						<label for="import-file" class="block text-sm font-medium text-gray-700 mb-1">
							Image File <span class="text-red-500">*</span>
						</label>
						<input
							id="import-file"
							type="file"
							onchange={handleImportFileSelected}
							class="w-full px-3 py-2 border border-gray-300 rounded text-sm text-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-500"
						/>
						{#if importFile}
							<p class="mt-1 text-xs text-gray-500">
								{importFile.name} ({formatFileSize(importFile.size)})
							</p>
						{:else}
							<p class="mt-1 text-xs text-gray-500">
								Maximum image size: {importMaxMB} MB. Large uploads can take a while.
							</p>
						{/if}
					</div>
				{:else}
					<div>
						<label for="import-path" class="block text-sm font-medium text-gray-700 mb-1">
							Server Image Path <span class="text-red-500">*</span>
						</label>
						<input
							id="import-path"
							type="text"
							bind:value={importPath}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="/path/to/image.img"
						/>
						<p class="mt-1 text-xs text-gray-500">
							Absolute path to an image already on the server.
						</p>
					</div>
				{/if}
				<div>
					<label for="import-name" class="block text-sm font-medium text-gray-700 mb-1">
						Instance Name (optional)
					</label>
					<input
						id="import-name"
						type="text"
						bind:value={importName}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder="Leave empty for upload filename or random name"
					/>
					<p class="mt-1 text-xs text-gray-500">
						Must be unique — importing over an existing instance is rejected.
					</p>
				</div>
			</div>
			{#if importError}
				<div class="mt-4 p-3 bg-red-50 border border-red-200 rounded">
					<p class="text-sm text-red-600">{importError}</p>
				</div>
			{/if}
			<div class="mt-6 flex justify-end gap-3">
				<Button variant="secondary" onclick={() => (showImportModal = false)}>Cancel</Button>
				<Button variant="primary" onclick={handleImport} disabled={importing || !importReady}>
					{importing ? 'Importing...' : 'Import'}
				</Button>
			</div>
		</div>
	</div>
{/if}
