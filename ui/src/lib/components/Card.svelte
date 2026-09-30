<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		padding?: boolean;
		class?: string;
		children: Snippet;
	}

	let { padding = true, class: className = '', children }: Props = $props();
</script>

<div class="bg-white border border-gray-200 rounded {padding ? 'p-4' : ''} {className}">
	{@render children()}
</div>
