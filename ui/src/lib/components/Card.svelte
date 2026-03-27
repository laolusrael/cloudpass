<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		padding?: boolean;
		children: Snippet;
	}

	let { padding = true, children }: Props = $props();
</script>

<div class="bg-white border border-gray-200 rounded {padding ? 'p-4' : ''}">
	{@render children()}
</div>
