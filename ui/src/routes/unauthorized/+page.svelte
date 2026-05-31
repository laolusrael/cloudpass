<svelte:head>
	<title>Access Denied - CloudPass</title>
</svelte:head>

<script lang="ts">
	import Card from '$lib/components/Card.svelte';
</script>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
	<Card class="max-w-md w-full">
		<div class="text-center">
			<div class="mb-6">
				<div class="mx-auto w-16 h-16 flex items-center justify-center rounded-full bg-gray-200">
					<svg class="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
							d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
					</svg>
				</div>
			</div>

			<h2 class="text-xl font-semibold text-gray-900 mb-2">Access Denied</h2>
			<p class="text-sm text-gray-600 mb-4">
				Your IP address is not authorized to access CloudPass.
			</p>
			<p class="text-xs text-gray-500">
				Contact your administrator to add your IP to the allowed list.
			</p>
		</div>
	</Card>
</div>
