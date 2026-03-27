<script lang="ts">
	import '../app.css';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
</script>

<div class="min-h-screen bg-gray-50">
	<header class="bg-white border-b border-gray-200">
		<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
			<div class="flex justify-between h-16 items-center">
				<div class="flex items-center">
					<h1 class="text-xl font-semibold text-gray-900">CloudPass</h1>
				</div>
				<nav class="flex gap-6">
					<a href="/" class="text-sm font-medium text-gray-700 hover:text-gray-900">
						Dashboard
					</a>
					<a href="/instances" class="text-sm font-medium text-gray-700 hover:text-gray-900">
						Instances
					</a>
				</nav>
			</div>
		</div>
	</header>

	<main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		{@render children()}
	</main>
</div>
