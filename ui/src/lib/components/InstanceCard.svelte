<script lang="ts">
	import type { Instance } from '$lib/types';
	import Card from './Card.svelte';
	import Badge from './Badge.svelte';
	import Button from './Button.svelte';

	interface Props {
		instance: Instance;
		onstart?: () => void;
		onstop?: () => void;
		onrestart?: () => void;
	ondelete?: () => void;
	}

	let { instance, onstart, onstop, onrestart, ondelete }: Props = $props();

	const isRunning = $derived(instance.state === 'Running');
	const isStopped = $derived(instance.state === 'Stopped');
</script>

<Card>
	<div class="flex items-start justify-between">
		<div class="flex-1 min-w-0">
			<div class="flex items-center gap-3">
				<h3 class="text-lg font-medium text-gray-900 truncate">
					{instance.name}
				</h3>
				<Badge state={instance.state} />
			</div>
			
			<div class="mt-2 grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
				<div>
					<span class="text-gray-500">IP:</span>
					<span class="ml-1 text-gray-700 font-mono">
						{instance.ipv4?.[0] || '-'}
					</span>
				</div>
				<div>
					<span class="text-gray-500">CPU:</span>
					<span class="ml-1 text-gray-700">{instance.cpu}</span>
				</div>
				<div>
					<span class="text-gray-500">Memory:</span>
					<span class="ml-1 text-gray-700">{instance.memory || '-'}</span>
				</div>
				<div>
					<span class="text-gray-500">Disk:</span>
					<span class="ml-1 text-gray-700">{instance.disk || '-'}</span>
				</div>
			</div>

			{#if instance.image}
				<p class="mt-2 text-xs text-gray-500">
					Image: {instance.image}
				</p>
			{/if}
		</div>

		<div class="flex flex-col gap-2 ml-4">
			{#if isRunning}
				<Button variant="secondary" size="sm" onclick={onstop}>
					Stop
				</Button>
				<Button variant="secondary" size="sm" onclick={onrestart}>
					Restart
				</Button>
			{:else if isStopped}
				<Button variant="primary" size="sm" onclick={onstart}>
					Start
				</Button>
			{/if}
			<Button variant="danger" size="sm" onclick={ondelete}>
				Delete
			</Button>
		</div>
	</div>
</Card>
