import { describe, it, expect } from 'vitest';
import {
	validateSelectedFile,
	formatFileSize,
	validateUploadSettings,
	validateImageSizeMB
} from '../src/lib/validation/upload';

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

describe('validateUploadSettings', () => {
	it('accepts valid settings', () => {
		expect(validateUploadSettings(100, '/home/ubuntu/uploads')).toBeNull();
	});

	it('rejects out-of-range sizes', () => {
		expect(validateUploadSettings(0, '/x')).toContain('between 1 and 1024');
		expect(validateUploadSettings(1025, '/x')).toContain('between 1 and 1024');
		expect(validateUploadSettings(1.5, '/x')).toContain('whole number');
		expect(validateUploadSettings(NaN, '/x')).toContain('whole number');
	});

	it('rejects bad default paths', () => {
		expect(validateUploadSettings(100, 'relative')).toContain('absolute');
		expect(validateUploadSettings(100, '/a/../b')).toContain('..');
	});

	it('validates the optional staging dir', () => {
		expect(validateUploadSettings(100, '/x', '')).toBeNull();
		expect(validateUploadSettings(100, '/x', '/var/staging')).toBeNull();
		expect(validateUploadSettings(100, '/x', 'C:\\staging')).toBeNull();
		expect(validateUploadSettings(100, '/x', 'relative/staging')).toContain('absolute');
	});
});

describe('validateImageSizeMB', () => {
	it('accepts values within 1 MB and 100 GB', () => {
		expect(validateImageSizeMB(10240)).toBeNull();
		expect(validateImageSizeMB(1)).toBeNull();
	});

	it('rejects out-of-range values', () => {
		expect(validateImageSizeMB(0)).toContain('between 1 and 102400');
		expect(validateImageSizeMB(102401)).toContain('between 1 and 102400');
		expect(validateImageSizeMB(1.5)).toContain('whole number');
		expect(validateImageSizeMB('big')).toContain('whole number');
	});
});
