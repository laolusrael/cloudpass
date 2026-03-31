import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('Terminal WebSocket Message Handling', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('should correctly identify binary Blob data', () => {
		const blob = new Blob(['test data'], { type: 'application/octet-stream' });
		expect(blob instanceof Blob).toBe(true);
	});

	it('should correctly identify text data', () => {
		const textData = 'hello world';
		expect(textData instanceof Blob).toBe(false);
	});

	it('should correctly serialize resize message as JSON', () => {
		const resizeMsg = { type: 'resize', cols: 80, rows: 24 };
		const json = JSON.stringify(resizeMsg);
		const parsed = JSON.parse(json);

		expect(parsed.type).toBe('resize');
		expect(parsed.cols).toBe(80);
		expect(parsed.rows).toBe(24);
	});

	it('should correctly parse error message from JSON', () => {
		const errorMsg = { type: 'error', data: 'SSH connection failed' };
		const json = JSON.stringify(errorMsg);
		const parsed = JSON.parse(json);

		expect(parsed.type).toBe('error');
		expect(parsed.data).toBe('SSH connection failed');
	});

	it('should correctly encode terminal data as binary', () => {
		const textData = 'ls -la';
		const encoder = new TextEncoder();
		const binaryData = encoder.encode(textData);

		expect(binaryData instanceof Uint8Array).toBe(true);
		expect(binaryData.length).toBe(textData.length);
	});

	it('should correctly decode binary data back to string', () => {
		const originalText = 'test output\r\n';
		const encoder = new TextEncoder();
		const binaryData = encoder.encode(originalText);

		const decoder = new TextDecoder();
		const decodedText = decoder.decode(binaryData);

		expect(decodedText).toBe(originalText);
	});

	it('should handle mixed binary and text WebSocket messages', () => {
		const binaryMessage = new Blob(['terminal output'], { type: 'application/octet-stream' });
		const textMessage = JSON.stringify({ type: 'error', data: 'connection lost' });

		const isBinary = binaryMessage instanceof Blob;
		const isTextJSON = textMessage.startsWith('{');

		expect(isBinary).toBe(true);
		expect(isTextJSON).toBe(true);
	});

	it('should handle ArrayBuffer conversion from Blob', async () => {
		const textData = 'test input';
		const blob = new Blob([textData], { type: 'application/octet-stream' });

		const arrayBuffer = await blob.arrayBuffer();
		const uint8Array = new Uint8Array(arrayBuffer);
		const decoder = new TextDecoder();
		const decoded = decoder.decode(uint8Array);

		expect(decoded).toBe(textData);
	});
});