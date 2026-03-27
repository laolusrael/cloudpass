<script lang="ts">
	import type { CreateInstanceRequest } from '$lib/types';
	import Input from './Input.svelte';
	import Button from './Button.svelte';

	interface Props {
		onSubmit: (data: CreateInstanceRequest) => void;
		onCancel: () => void;
		loading?: boolean;
	}

	let { onSubmit, onCancel, loading = false }: Props = $props();

	let name = $state('');
	let image = $state('');
	let cpus = $state(1);
	let memory = $state('1G');
	let disk = $state('5G');

	function handleSubmit(e: Event) {
		e.preventDefault();
		onSubmit({
			name,
			image: image || undefined,
			cpus: cpus || undefined,
			memory: memory || undefined,
			disk: disk || undefined
		});
	}
</script>

<form onsubmit={handleSubmit} class="space-y-4">
	<Input
		label="Instance Name"
		placeholder="my-vm"
		bind:value={name}
		required
	/>

	<Input
		label="Image"
		placeholder="22.04 (default: Ubuntu LTS)"
		bind:value={image}
	/>

	<div class="grid grid-cols-3 gap-4">
		<Input
			label="CPUs"
			type="number"
			placeholder="1"
			bind:value={cpus}
		/>

		<Input
			label="Memory"
			placeholder="1G"
			bind:value={memory}
		/>

		<Input
			label="Disk"
			placeholder="5G"
			bind:value={disk}
		/>
	</div>

	<div class="flex justify-end gap-3 pt-4">
		<Button variant="secondary" type="button" onclick={onCancel}>
			Cancel
		</Button>
		<Button variant="primary" type="submit" disabled={loading}>
			{loading ? 'Creating...' : 'Create Instance'}
		</Button>
	</div>
</form>
