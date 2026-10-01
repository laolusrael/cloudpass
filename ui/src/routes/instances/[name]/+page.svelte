<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/services/api';
	import type { Instance, HostInfo, UpdateResourcesRequest } from '$lib/types';
	import { notifications } from '$lib/stores/notifications';
	import { validateSelectedFile, formatFileSize } from '$lib/validation/upload';
	import { waitForJob } from '$lib/utils/job-poller';
	import { generateIdempotencyKey } from '$lib/utils/id';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import Spinner from '$lib/components/Spinner.svelte';
	import InstanceResourcesModal from '$lib/components/InstanceResourcesModal.svelte';

	let instance = $state<Instance | null>(null);
	let hostInfo = $state<HostInfo | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showMountModal = $state(false);
	let mountSourcePath = $state('');
	let mountTargetPath = $state('');
	let mountType = $state('classic');
	let mountUidMap = $state('');
	let mountGidMap = $state('');
	let mounting = $state(false);
	let mountError = $state<string | null>(null);
	let showUploadModal = $state(false);
	let uploadTargetPath = $state('');
	let uploading = $state(false);
	let selectedFile = $state<File | null>(null);
	let uploadError = $state<string | null>(null);
	let uploadProgress = $state<number | null>(null);
	let uploadMaxMB = $state(100);
	let uploadDefaultPath = $state('/home/ubuntu/uploads');
	let showEditResourcesModal = $state(false);

	const name = $derived($page.params.name ?? '');

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

	async function loadHostInfo() {
		try {
			hostInfo = await api.getHostInfo();
		} catch (e) {
			console.error('Failed to load host info:', e);
		}
	}

	async function loadUploadLimits() {
		try {
			const config = await api.getConfig();
			if (config.upload) {
				uploadMaxMB = config.upload.max_file_size_mb;
				uploadDefaultPath = config.upload.default_path;
			}
		} catch (e) {
			console.error('Failed to load upload limits:', e);
		}
	}

	onMount(() => {
		loadInstance();
		loadHostInfo();
		loadUploadLimits();
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
		const name = instance.name;
		try {
			const job = await api.exportInstanceAsync(name, undefined, {
				idempotencyKey: generateIdempotencyKey()
			});
			notifications.info(`Export of "${name}" started — this can take a while.`);
			const result = await waitForJob(job.id);
			notifications.success(`Instance exported to: ${result || 'image file'}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to export';
		}
	}

	async function handleImport() {
		const imagePath = prompt('Enter image path to import:');
		if (!imagePath) return;
		const name = prompt('Enter instance name (optional):');
		try {
			await api.importInstanceAsync(
				{ image_path: imagePath, name: name || undefined },
				{ idempotencyKey: generateIdempotencyKey() }
			);
			notifications.info('Import started — you will be notified when ready.');
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to import';
		}
	}

	function openMountModal() {
		mountSourcePath = '';
		mountTargetPath = '';
		mountType = 'classic';
		mountUidMap = '';
		mountGidMap = '';
		mountError = null;
		showMountModal = true;
	}

	function closeMountModal() {
		showMountModal = false;
		mountSourcePath = '';
		mountTargetPath = '';
		mountType = 'classic';
		mountUidMap = '';
		mountGidMap = '';
		mountError = null;
	}

	async function handleMount() {
		if (!instance || !mountSourcePath.trim() || !mountTargetPath.trim()) return;
		const source = mountSourcePath.trim();
		const target = mountTargetPath.trim();
		mounting = true;
		mountError = null;
		try {
			const job = await api.mountInstanceAsync(
				instance.name,
				{
					source_path: source,
					target_path: target,
					mount_type: mountType,
					uid_map: mountUidMap.trim() || undefined,
					gid_map: mountGidMap.trim() || undefined
				},
				{ idempotencyKey: generateIdempotencyKey() }
			);
			closeMountModal();
			notifications.info(`Mount of ${source} started — this can take a few minutes.`);
			try {
				await waitForJob(job.id);
			} catch (e) {
				// The modal is closed: surface background failures on the page.
				error = e instanceof Error ? e.message : 'Mount failed';
				return;
			}
			notifications.success(`Mounted ${source} to ${target}`);
			await loadInstance();
		} catch (e) {
			mountError = e instanceof Error ? e.message : 'Failed to mount';
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

	function openUploadModal() {
		selectedFile = null;
		uploadTargetPath = '';
		uploadError = null;
		uploadProgress = null;
		showUploadModal = true;
	}

	function closeUploadModal() {
		showUploadModal = false;
		selectedFile = null;
		uploadTargetPath = '';
		uploadError = null;
		uploadProgress = null;
	}

	function handleFileSelected(e: Event) {
		uploadError = null;
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0] ?? null;
		if (!file) {
			selectedFile = null;
			return;
		}
		const problem = validateSelectedFile(file, uploadMaxMB);
		if (problem) {
			selectedFile = null;
			input.value = '';
			uploadError = problem;
			return;
		}
		selectedFile = file;
	}

	async function handleUpload() {
		if (!instance || !selectedFile) return;
		const file = selectedFile;
		uploading = true;
		uploadError = null;
		uploadProgress = 0;
		try {
			const result = await api.uploadFileWithProgress(
				instance.name,
				file,
				uploadTargetPath.trim() || undefined,
				(loaded, total) => {
					uploadProgress = total > 0 ? Math.round((loaded / total) * 100) : null;
				}
			);
			closeUploadModal();
			notifications.success(`File uploaded to ${result.path ?? file.name}`);
		} catch (e) {
			uploadError = e instanceof Error ? e.message : 'Failed to upload';
		} finally {
			uploading = false;
		}
	}

	async function handleUpdateResources(request: UpdateResourcesRequest) {
		if (!instance) return;
		try {
			await api.updateInstanceResources(instance.name, request);
			await loadInstance();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update resources';
			throw e;
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
				{#if isStopped && hostInfo}
					<Button variant="secondary" onclick={() => (showEditResourcesModal = true)}
						>Edit Resources</Button
					>
				{/if}
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
							<Button variant="secondary" onclick={openUploadModal}>Upload File</Button>
							<Button variant="secondary" onclick={openMountModal}>Add Mount</Button>
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
				Map a host directory into the instance. The host path must be an existing absolute directory
				(no ~ expansion).
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
					<p class="mt-1 text-xs text-gray-500">
						Created if missing; existing contents are overlaid, not deleted.
					</p>
				</div>
				<div>
					<label for="mount-type" class="block text-sm font-medium text-gray-700 mb-1">
						Mount Type
					</label>
					<select
						id="mount-type"
						bind:value={mountType}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
					>
						<option value="classic">Classic (SSHFS, works everywhere)</option>
						<option value="native">Native (faster, Hyper-V/QEMU only)</option>
					</select>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label for="mount-uid" class="block text-sm font-medium text-gray-700 mb-1">
							UID Map (optional)
						</label>
						<input
							id="mount-uid"
							type="text"
							bind:value={mountUidMap}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="1000:1000"
						/>
					</div>
					<div>
						<label for="mount-gid" class="block text-sm font-medium text-gray-700 mb-1">
							GID Map (optional)
						</label>
						<input
							id="mount-gid"
							type="text"
							bind:value={mountGidMap}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="1000:1000"
						/>
					</div>
				</div>
				<p class="text-xs text-gray-500">
					ID maps are host:instance pairs (e.g. 1000:1000). Leave empty for defaults.
				</p>
			</div>
			<div class="mt-4 p-3 bg-yellow-50 border border-yellow-200 rounded">
				<p class="text-xs text-yellow-800">
					Mounted host directories are readable and writable from the instance. Only mount paths you
					trust.
				</p>
			</div>
			{#if mountError}
				<div class="mt-4 p-3 bg-red-50 border border-red-200 rounded">
					<p class="text-sm text-red-600">{mountError}</p>
				</div>
			{/if}
			<div class="mt-6 flex justify-end gap-3">
				<Button variant="secondary" onclick={closeMountModal}>Cancel</Button>
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

{#if showUploadModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg p-6 w-full max-w-md">
			<h3 class="text-lg font-medium mb-4">Upload File</h3>
			<div class="space-y-4">
				<div>
					<label for="upload-file" class="block text-sm font-medium text-gray-700 mb-1">
						File <span class="text-red-500">*</span>
					</label>
					<input
						id="upload-file"
						type="file"
						onchange={handleFileSelected}
						class="w-full px-3 py-2 border border-gray-300 rounded text-sm text-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-500"
					/>
					{#if selectedFile}
						<p class="mt-1 text-xs text-gray-500">
							{selectedFile.name} ({formatFileSize(selectedFile.size)})
						</p>
					{:else}
						<p class="mt-1 text-xs text-gray-500">Maximum file size: {uploadMaxMB} MB.</p>
					{/if}
				</div>
				<div>
					<label for="upload-target" class="block text-sm font-medium text-gray-700 mb-1">
						Target Path (optional)
					</label>
					<input
						id="upload-target"
						type="text"
						bind:value={uploadTargetPath}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder={uploadDefaultPath}
					/>
					<p class="mt-1 text-xs text-gray-500">
						Empty uploads to {uploadDefaultPath}/&lt;filename&gt;. A trailing / targets a directory
						(filename appended); otherwise a full file path is required. Uploading to an existing
						path overwrites it.
					</p>
				</div>
			</div>
			{#if uploadError}
				<div class="mt-4 p-3 bg-red-50 border border-red-200 rounded">
					<p class="text-sm text-red-600">{uploadError}</p>
				</div>
			{/if}
			{#if uploading && uploadProgress !== null}
				<div class="mt-4">
					<div class="w-full bg-gray-200 rounded h-2">
						<div class="bg-gray-700 rounded h-2" style="width: {uploadProgress}%"></div>
					</div>
					<p class="mt-1 text-xs text-gray-500">Uploading... {uploadProgress}%</p>
				</div>
			{/if}
			<div class="mt-6 flex justify-end gap-3">
				<Button variant="secondary" onclick={closeUploadModal}>Cancel</Button>
				<Button variant="primary" onclick={handleUpload} disabled={uploading || !selectedFile}>
					{uploading ? 'Uploading...' : 'Upload'}
				</Button>
			</div>
		</div>
	</div>
{/if}

{#if showEditResourcesModal && instance && hostInfo}
	<InstanceResourcesModal
		{instance}
		{hostInfo}
		onclose={() => (showEditResourcesModal = false)}
		onsave={handleUpdateResources}
	/>
{/if}
