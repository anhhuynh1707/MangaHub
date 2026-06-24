import { test, expect } from '@playwright/test'

// The backend API base (the browser app talks to it via VITE_API_URL). The test
// also calls it directly to create a deterministic manga, so we don't depend on
// the flaky MangaDex seed.
const API_URL = process.env.E2E_API_URL || 'http://localhost:8080'

// One fresh user walks through every core feature, in order.
test('user journey: register → login → library → progress → review → chat', async ({ page, request }) => {
  const unique = Date.now()
  const username = `e2e_${unique}`
  const password = 'e2epass123'
  const mangaId = `e2e-manga-${unique}`
  const mangaTitle = `E2E Test Manga ${unique}`
  let token = '' // captured after login, reused for API setup + cleanup

  // ── 1. Register ──────────────────────────────────────────────────
  await test.step('register a new account', async () => {
    await page.goto('/auth')
    await page.getByTestId('tab-register').click()
    // Wait for the register form to finish its enter animation before filling,
    // otherwise the still-exiting login form steals the fill (AnimatePresence).
    await expect(page.getByTestId('submit-register')).toBeVisible()
    await page.getByPlaceholder('Your username').fill(username)
    await page.getByPlaceholder('Your password', { exact: true }).fill(password)
    await page.getByPlaceholder('Repeat your password').fill(password)
    await page.getByTestId('submit-register').click()

    // New flow: no auto-login — we land on the Sign In tab with username prefilled.
    await expect(page.getByTestId('submit-login')).toBeVisible()
    await expect(page.getByPlaceholder('Your username')).toHaveValue(username)
  })

  // ── 2. Login ─────────────────────────────────────────────────────
  await test.step('log in', async () => {
    await page.getByPlaceholder('Your password', { exact: true }).fill(password)
    await page.getByTestId('submit-login').click()

    // Lands on the Browse (home) page.
    await expect(page).toHaveURL(/\/$/)
    await expect(page.getByRole('heading', { name: 'Browse Manga' })).toBeVisible()
  })

  // ── 3. Seed a manga via the API (deterministic test data) ─────────
  await test.step('create a manga via API', async () => {
    token = (await page.evaluate(() => {
      const raw = localStorage.getItem('mangahub-auth')
      return raw ? (JSON.parse(raw).state.token as string) : ''
    })) as string
    expect(token).toBeTruthy()

    const res = await request.post(`${API_URL}/manga`, {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        id: mangaId,
        title: mangaTitle,
        author: 'E2E Author',
        genres: ['Action'],
        status: 'ongoing',
        total_chapters: 24,
        description: 'Created by the Playwright E2E journey test.',
      },
    })
    expect(res.status()).toBe(201)
  })

  // ── 4. Add the manga to the library ──────────────────────────────
  await test.step('add manga to library', async () => {
    await page.goto(`/manga/${mangaId}`)
    await expect(page.getByRole('heading', { name: mangaTitle })).toBeVisible()

    await page.getByRole('button', { name: 'Add to Library' }).click()
    // Once added, the in-library controls (Remove) appear.
    await expect(page.getByRole('button', { name: 'Remove' })).toBeVisible()
  })

  // ── 5. Update reading progress ───────────────────────────────────
  await test.step('increment chapter on the library page', async () => {
    await page.goto('/library')
    await expect(page.getByText(mangaTitle)).toBeVisible()
    await expect(page.getByText('Ch. 0')).toBeVisible()

    await page.getByTestId('chapter-increment').click()
    await expect(page.getByText('Ch. 1')).toBeVisible()
  })

  // ── 6. Leave a review ────────────────────────────────────────────
  await test.step('write a review', async () => {
    await page.goto(`/manga/${mangaId}`)
    await page.getByRole('button', { name: 'Write a review' }).click()

    await page.getByTestId('rating-star-8').click()
    await page.getByPlaceholder('Share your thoughts about this manga…')
      .fill('A solid read — the E2E test enjoyed it.')
    await page.getByRole('button', { name: 'Submit Review' }).click()

    // Either the confirmation banner or the rendered review appears.
    await expect(page.getByText(/Review submitted|enjoyed it/)).toBeVisible()
  })

  // ── 7. Join chat ─────────────────────────────────────────────────
  await test.step('join chat and connect', async () => {
    await page.goto('/chat')
    await expect(page.getByText('Connected')).toBeVisible({ timeout: 10_000 })
  })

  // ── 8. Clean up the test data ────────────────────────────────────
  // The manga is referenced by foreign keys (no cascade) from the library
  // entry, the review, AND activity-feed rows (add-to-library and review each
  // log an activity with manga_id). All must be removed before the manga.
  await test.step('clean up test data via API', async () => {
    const auth = { headers: { Authorization: `Bearer ${token}` } }

    // Remove the library entry.
    await request.delete(`${API_URL}/users/library/${mangaId}`, auth)

    // Delete any reviews this user left on the manga.
    const reviewsRes = await request.get(`${API_URL}/manga/${mangaId}/reviews`)
    const reviews = (await reviewsRes.json())?.data?.reviews ?? []
    for (const r of reviews) {
      await request.delete(`${API_URL}/reviews/${r.id}`, auth)
    }

    // Clear this user's activity feed (its rows reference manga_id).
    await request.delete(`${API_URL}/feed/clear`, auth)

    // Finally delete the manga — now free of references.
    const del = await request.delete(`${API_URL}/manga/${mangaId}`, auth)
    expect(del.status()).toBe(200)

    // Verify it's gone.
    const check = await request.get(`${API_URL}/manga/${mangaId}`)
    expect(check.status()).toBe(404)
  })
})
