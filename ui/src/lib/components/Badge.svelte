<script lang="ts">
	interface Props {
		state: string;
	}

	let { state }: Props = $props();

	const stateColors: Record<string, string> = {
		Running: 'bg-green-100 text-green-800',
		Starting: 'bg-yellow-100 text-yellow-800',
		Stopped: 'bg-gray-100 text-gray-800',
		Stopping: 'bg-yellow-100 text-yellow-800',
		Suspended: 'bg-blue-100 text-blue-800',
		Deleting: 'bg-red-100 text-red-800',
		Deleted: 'bg-gray-100 text-gray-500'
	};

	const colorClass = $derived(stateColors[state] || 'bg-gray-100 text-gray-800');
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {colorClass}">
	{state}
</span>
