<script lang="ts">
	import { onMount } from 'svelte';
	import type { CreateInstanceRequest, Image } from '$lib/types';
	import { images } from '$lib/stores/images';
	import Input from './Input.svelte';
	import Select from './Select.svelte';
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

	const imageOptions = $derived(
		$images.map((img: Image) => ({
			value: img.alias || img.release,
			label: img.os ? `${img.os} ${img.release || img.alias} (${img.alias || img.version})` : (img.release ? `${img.release} (${img.alias || img.version})` : img.alias)
		}))
	);

	onMount(() => {
		images.load();
	});

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
	<Input label="Instance Name" placeholder="my-vm" bind:value={name} required />

	<Select
		label="Image"
		options={imageOptions}
		bind:value={image}
		placeholder="Select an image (default: Ubuntu LTS)"
		searchable={true}
	/>

	<div class="grid grid-cols-3 gap-4">
		<Input label="CPUs" type="number" placeholder="1" bind:value={cpus} />

		<Input label="Memory" placeholder="1G" bind:value={memory} />

		<Input label="Disk" placeholder="5G" bind:value={disk} />
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
