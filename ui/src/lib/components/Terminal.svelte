<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';

	interface Props {
		instanceName: string;
		wsUrl?: string;
	}

	let { instanceName, wsUrl = '' }: Props = $props();

	let terminalContainer: HTMLDivElement;
	let terminal: Terminal | null = null;
	let fitAddon: FitAddon | null = null;
	let ws: WebSocket | null = null;
	let connected = $state(false);
	let error = $state('');

	function getWsUrl(): string {
		if (wsUrl) return wsUrl;
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		return `${protocol}//${host}/api/instances/${instanceName}/terminal`;
	}

	function connect() {
		if (ws && ws.readyState === WebSocket.OPEN) {
			return;
		}

		ws = new WebSocket(getWsUrl());

		ws.onopen = () => {
			connected = true;
			error = '';
			terminal?.write('\r\n\x1b[32mConnected to ' + instanceName + '\x1b[0m\r\n$ ');
		};

		ws.onmessage = async (event) => {
			if (event.data instanceof Blob) {
				const buf = await event.data.arrayBuffer();
				terminal?.write(new Uint8Array(buf));
			} else {
				try {
					const msg = JSON.parse(event.data);
					if (msg.type === 'error') {
						error = msg.data;
						connected = false;
						return;
					}
				} catch {
					// Plain text - write to terminal
					terminal?.write(event.data);
				}
			}
		};

		ws.onclose = () => {
			if (!error) {
				terminal?.write('\r\n\x1b[33mDisconnected\x1b[0m\r\n');
			}
			connected = false;
		};

		ws.onerror = () => {
			error = 'Connection error';
			connected = false;
		};
	}

	function handleResize() {
		fitAddon?.fit();
		if (terminal && fitAddon) {
			const { cols, rows } = terminal;
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ type: 'resize', cols, rows }));
			}
		}
	}

	onMount(() => {
		terminal = new Terminal({
			cursorBlink: true,
			fontSize: 14,
			fontFamily: 'Menlo, Monaco, "Courier New", monospace',
			theme: {
				background: '#1a1a1a',
				foreground: '#f0f0f0',
				cursor: '#f0f0f0'
			},
			allowProposedApi: true
		});

		fitAddon = new FitAddon();
		terminal.loadAddon(fitAddon);
		terminal.open(terminalContainer);
		fitAddon.fit();

		terminal.onData((data) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(new TextEncoder().encode(data));
			}
		});

		connect();

		window.addEventListener('resize', handleResize);
		const resizeObserver = new ResizeObserver(handleResize);
		resizeObserver.observe(terminalContainer);
	});

	onDestroy(() => {
		window.removeEventListener('resize', handleResize);
		ws?.close();
		terminal?.dispose();
	});
</script>

<div class="flex flex-col h-full">
	<div class="flex items-center justify-between px-4 py-2 bg-gray-100 border-b">
		<span class="text-sm font-medium text-gray-700">
			Terminal: {instanceName}
		</span>
		<div class="flex items-center gap-2">
			{#if connected}
				<span class="text-xs text-green-600">Connected</span>
			{:else if error}
				<span class="text-xs text-red-600">{error}</span>
			{/if}
			<button
				class="px-3 py-1 text-xs bg-gray-700 text-white rounded hover:bg-gray-600"
				onclick={connect}
			>
				Reconnect
			</button>
		</div>
	</div>
	<div bind:this={terminalContainer} class="flex-1 overflow-hidden"></div>
</div>

<style>
	:global(.xterm) {
		padding: 8px;
	}
	:global(.xterm-viewport) {
		overflow-y: auto !important;
	}
</style>
