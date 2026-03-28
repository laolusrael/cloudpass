import { test, expect } from '@playwright/test';

test.describe('CloudPass UI', () => {
	test('homepage loads successfully', async ({ page }) => {
		await page.goto('/');
		await expect(page).toHaveTitle(/CloudPass/);
	});

	test('instances page shows heading', async ({ page }) => {
		await page.goto('/instances');
		await expect(page.getByRole('heading', { name: /instances/i })).toBeVisible();
	});

	test('networks page shows heading', async ({ page }) => {
		await page.goto('/networks');
		await expect(page.getByRole('heading', { name: /networks/i })).toBeVisible();
	});

	test('create instance page loads', async ({ page }) => {
		await page.goto('/instances/new');
		await expect(page.getByRole('heading', { name: /create instance/i })).toBeVisible();
	});

	test('navigation between pages works', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('link', { name: /instances/i })).toBeVisible();
		await expect(page.getByRole('link', { name: /networks/i })).toBeVisible();
	});
});
