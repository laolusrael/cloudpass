import { describe, it, expect } from 'vitest';
import { validateSelectedFile, formatFileSize } from '../src/lib/validation/upload';

function sizedFile(size: number): File {
	const blob = new Blob([new Uint8Array(size)]);
	return new File([blob], 'seed.txt', { type: 'text/plain' });
}

describe('validateSelectedFile', () => {
	it('accepts a file within the limit', () => {
		expect(validateSelectedFile(sizedFile(100), 100)).toBeNull();
	});

	it('rejects an empty file', () => {
		expect(validateSelectedFile(sizedFile(0), 100)).toBe('The selected file is empty.');
	});

	it('rejects a file over the limit', () => {
		const result = validateSelectedFile(sizedFile(1024 * 1024 + 1), 1);
		expect(result).toBe('The selected file exceeds the maximum of 1 MB.');
	});
});

describe('formatFileSize', () => {
	it('formats bytes, kilobytes, and megabytes', () => {
		expect(formatFileSize(512)).toBe('512 B');
		expect(formatFileSize(2048)).toBe('2.0 KB');
		expect(formatFileSize(3 * 1024 * 1024)).toBe('3.0 MB');
	});
});
