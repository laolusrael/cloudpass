<script lang="ts">
	import type { Instance, HostInfo, UpdateResourcesRequest } from '$lib/types';
	import Button from './Button.svelte';

	interface Props {
		instance: Instance;
		hostInfo: HostInfo;
		onclose: () => void;
		onsave: (request: UpdateResourcesRequest) => Promise<void>;
	}

	let { instance, hostInfo, onclose, onsave } = $props<Props>();

	let cpus = $state(instance.cpu);
	let memory = $state(parseMemoryToMB(instance.memory));
	let disk = $state(parseDiskToMB(instance.disk));
	let diskInput = $state(instance.disk);
	let saving = $state(false);
	let error = $state<string | null>(null);

	const cpuMax = $derived(Math.min(hostInfo.cpu_available, 16));
	const memoryMax = $derived(Math.floor(hostInfo.memory_available / (1024 * 1024)));
	const diskMax = $derived(Math.floor(hostInfo.disk_available / (1024 * 1024)));
	const currentDiskMB = $derived(parseDiskToMB(instance.disk));
	const memoryMarks = $derived(
		[256, 512, 1024, 2048, 4096, 8192, 16384, 32768].filter((m) => m <= memoryMax).slice(0, 4)
	);

	function parseMemoryToMB(mem: string): number {
		if (!mem) return 1024;
		const match = mem.match(/^(\d+(?:\.\d+)?)\s*([KMGT]?)/i);
		if (!match) return 1024;
		const val = parseFloat(match[1]);
		const unit = match[2].toUpperCase();
		switch (unit) {
			case 'K':
				return Math.round(val / 1024);
			case 'M':
				return Math.round(val);
			case 'G':
				return Math.round(val * 1024);
			case 'T':
				return Math.round(val * 1024 * 1024);
			default:
				return Math.round(val / (1024 * 1024));
		}
	}

	function parseDiskToMB(disk: string): number {
		if (!disk) return 5120;
		const match = disk.match(/^(\d+(?:\.\d+)?)\s*([KMGT]?)/i);
		if (!match) return 5120;
		const val = parseFloat(match[1]);
		const unit = match[2].toUpperCase();
		switch (unit) {
			case 'K':
				return Math.round(val / 1024);
			case 'M':
				return Math.round(val);
			case 'G':
				return Math.round(val * 1024);
			case 'T':
				return Math.round(val * 1024 * 1024);
			default:
				return Math.round(val / (1024 * 1024));
		}
	}

	function formatMemoryMB(mb: number): string {
		if (mb >= 1024) {
			return `${(mb / 1024).toFixed(1)}G`;
		}
		return `${mb}M`;
	}

	function formatDiskMB(mb: number): string {
		if (mb >= 1024) {
			return `${(mb / 1024).toFixed(1)}G`;
		}
		return `${mb}M`;
	}

	function handleDiskInput(e: Event) {
		const target = e.target as HTMLInputElement;
		diskInput = target.value;
		const parsed = parseDiskToMB(target.value);
		if (!isNaN(parsed)) {
			disk = Math.max(disk, parsed);
		}
	}

	async function handleSubmit() {
		saving = true;
		error = null;
		try {
			await onsave({
				cpus,
				memory: formatMemoryMB(memory),
				disk: formatDiskMB(disk)
			});
			onclose();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update resources';
		} finally {
			saving = false;
		}
	}
</script>

<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
	<div class="bg-white rounded-lg p-6 w-full max-w-md">
		<h3 class="text-lg font-medium mb-4">Edit Resources: {instance.name}</h3>
		<p class="text-sm text-gray-500 mb-4">Instance must be stopped to modify resources.</p>

		<div class="space-y-6">
			<div>
				<label for="cpus" class="block text-sm font-medium text-gray-700 mb-2">
					CPU Cores: {cpus}
				</label>
				<input
					id="cpus"
					type="range"
					min="1"
					max={cpuMax}
					bind:value={cpus}
					class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
				/>
				<div class="flex justify-between text-xs text-gray-500 mt-1">
					<span>1</span>
					<span>{cpuMax} (max)</span>
				</div>
			</div>

			<div>
				<label for="memory" class="block text-sm font-medium text-gray-700 mb-2">
					Memory: {formatMemoryMB(memory)}
				</label>
				<input
					id="memory"
					type="range"
					min="256"
					max={memoryMax}
					step="256"
					bind:value={memory}
					class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
				/>
				<div class="flex justify-between text-xs text-gray-500 mt-1">
					<span>256M</span>
					<span>{formatMemoryMB(memoryMax)} (max)</span>
				</div>
				{#if memoryMarks.length > 0}
					<div class="flex justify-between text-xs text-gray-400 mt-1">
						{#each memoryMarks.slice(0, 4) as mark}
							<span>{formatMemoryMB(mark)}</span>
						{/each}
					</div>
				{/if}
			</div>

			<div>
				<label for="disk" class="block text-sm font-medium text-gray-700 mb-2">
					Disk: {formatDiskMB(disk)} (can only increase)
				</label>
				<input
					id="disk"
					type="range"
					min={currentDiskMB}
					max={diskMax}
					step="1024"
					bind:value={disk}
					class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
				/>
				<div class="flex justify-between text-xs text-gray-500 mt-1">
					<span>{instance.disk} (current)</span>
					<span>{formatDiskMB(diskMax)} (max)</span>
				</div>
				<div class="mt-2">
					<label for="disk-input" class="block text-sm font-medium text-gray-700 mb-1">
						Exact value:
					</label>
					<input
						id="disk-input"
						type="text"
						bind:value={diskInput}
						oninput={handleDiskInput}
						class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						placeholder="e.g., 10G"
					/>
				</div>
			</div>
		</div>

		{#if error}
			<div class="mt-4 p-3 bg-red-50 border border-red-200 rounded">
				<p class="text-sm text-red-600">{error}</p>
			</div>
		{/if}

		<div class="mt-6 flex justify-end gap-3">
			<Button variant="secondary" onclick={onclose} disabled={saving}>Cancel</Button>
			<Button variant="primary" onclick={handleSubmit} disabled={saving}>
				{saving ? 'Saving...' : 'Save'}
			</Button>
		</div>
	</div>
</div>
