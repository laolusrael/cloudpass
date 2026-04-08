<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/services/api';
	import { notifications } from '$lib/stores/notifications';
	import type { ConfigResponse } from '$lib/types';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Spinner from '$lib/components/Spinner.svelte';

	let config = $state<ConfigResponse | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	let serverHost = $state('');
	let serverPort = $state(0);
	let allowedIPs = $state('');
	let websocketTimeout = $state(0);
	let socketPath = $state('');
	let timeoutSec = $state(0);
	let sshKeyPath = $state('');
	let logLevel = $state('');
	let logFormat = $state('');
	let logOutput = $state('');

	async function loadConfig() {
		loading = true;
		error = null;
		try {
			config = await api.getConfig();
			serverHost = config.server.host;
			serverPort = config.server.port;
			allowedIPs = config.security.allowed_ips.join(', ');
			websocketTimeout = config.security.websocket_timeout_minutes;
			socketPath = config.multipass.socket_path;
			timeoutSec = config.multipass.default_timeout_seconds;
			sshKeyPath = config.multipass.ssh_key_path;
			logLevel = config.logging.level;
			logFormat = config.logging.format;
			logOutput = config.logging.output;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load config';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadConfig();
	});

	async function handleSave() {
		saving = true;
		error = null;
		try {
			const ips = allowedIPs
				.split(',')
				.map((ip) => ip.trim())
				.filter((ip) => ip !== '');

			await api.updateConfig({
				server: {
					host: serverHost,
					port: serverPort
				},
				security: {
					allowed_ips: ips,
					websocket_timeout_minutes: websocketTimeout
				},
				multipass: {
					socket_path: socketPath,
					default_timeout_seconds: timeoutSec,
					ssh_key_path: sshKeyPath
				},
				logging: {
					level: logLevel,
					format: logFormat,
					output: logOutput
				}
			});
			notifications.success('Configuration saved. Restart required for changes to take effect.');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save config';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Settings - CloudPass</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-2xl font-bold text-gray-900">Settings</h2>
			<p class="mt-1 text-sm text-gray-500">Configure CloudPass settings</p>
		</div>
		<div class="flex gap-3">
			<Button variant="secondary" onclick={loadConfig}>Refresh</Button>
			<Button variant="primary" onclick={handleSave} disabled={saving}>
				{saving ? 'Saving...' : 'Save Changes'}
			</Button>
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
	{:else if config}
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-4">Server</h3>
				<div class="space-y-4">
					<div>
						<label for="server-host" class="block text-sm font-medium text-gray-700 mb-1"
							>Host</label
						>
						<input
							id="server-host"
							type="text"
							bind:value={serverHost}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="0.0.0.0"
						/>
					</div>
					<div>
						<label for="server-port" class="block text-sm font-medium text-gray-700 mb-1"
							>Port</label
						>
						<input
							id="server-port"
							type="number"
							bind:value={serverPort}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="8080"
						/>
					</div>
				</div>
			</Card>

			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-4">Security</h3>
				<div class="space-y-4">
					<div>
						<label for="allowed-ips" class="block text-sm font-medium text-gray-700 mb-1">
							Allowed IPs (comma-separated)
						</label>
						<input
							id="allowed-ips"
							type="text"
							bind:value={allowedIPs}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="192.168.1.0/24, 10.0.0.0/8"
						/>
						<p class="mt-1 text-xs text-gray-500">Leave empty to allow all IPs</p>
					</div>
					<div>
						<label for="ws-timeout" class="block text-sm font-medium text-gray-700 mb-1">
							WebSocket Timeout (minutes)
						</label>
						<input
							id="ws-timeout"
							type="number"
							bind:value={websocketTimeout}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="30"
						/>
					</div>
				</div>
			</Card>

			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-4">Multipass</h3>
				<div class="space-y-4">
					<div>
						<label for="socket-path" class="block text-sm font-medium text-gray-700 mb-1"
							>Socket Path</label
						>
						<input
							id="socket-path"
							type="text"
							bind:value={socketPath}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="/var/run/multipass_socket"
						/>
					</div>
					<div>
						<label for="timeout-sec" class="block text-sm font-medium text-gray-700 mb-1">
							Default Timeout (seconds)
						</label>
						<input
							id="timeout-sec"
							type="number"
							bind:value={timeoutSec}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="300"
						/>
					</div>
					<div>
						<label for="ssh-key-path" class="block text-sm font-medium text-gray-700 mb-1">
							SSH Key Path (for terminal access)
						</label>
						<input
							id="ssh-key-path"
							type="text"
							bind:value={sshKeyPath}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
							placeholder="~/.cloudpass/multipass_id_rsa (auto-detected if empty)"
						/>
						<p class="mt-1 text-xs text-gray-500">
							Leave empty to use default path (~/.cloudpass/multipass_id_rsa)
						</p>
					</div>
				</div>
			</Card>

			<Card>
				<h3 class="text-sm font-medium text-gray-500 mb-4">Logging</h3>
				<div class="space-y-4">
					<div>
						<label for="log-level" class="block text-sm font-medium text-gray-700 mb-1">Level</label
						>
						<select
							id="log-level"
							bind:value={logLevel}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						>
							<option value="debug">Debug</option>
							<option value="info">Info</option>
							<option value="warn">Warn</option>
							<option value="error">Error</option>
						</select>
					</div>
					<div>
						<label for="log-format" class="block text-sm font-medium text-gray-700 mb-1"
							>Format</label
						>
						<select
							id="log-format"
							bind:value={logFormat}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						>
							<option value="console">Console</option>
							<option value="json">JSON</option>
						</select>
					</div>
					<div>
						<label for="log-output" class="block text-sm font-medium text-gray-700 mb-1"
							>Output</label
						>
						<select
							id="log-output"
							bind:value={logOutput}
							class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-gray-500"
						>
							<option value="stdout">Stdout</option>
							<option value="file">File</option>
							<option value="syslog">Syslog</option>
						</select>
					</div>
				</div>
			</Card>
		</div>
	{/if}
</div>
