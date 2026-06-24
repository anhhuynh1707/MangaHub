import { test, expect } from '@playwright/test'

// Not a test — a screenshot generator for the README. Run with:
//   npm run screenshots
// Writes PNGs into the repo-root img/ directory. It registers a demo user and,
// if the manga catalog is empty (no MangaDex seed), creates a demo manga so the
// detail/library pages still render. Best results come from a backend with the
// real seeded catalog (cover art).
const API_URL = process.env.E2E_API_URL || 'http://localhost:8080'
const shot = (name: string) => `../img/${name}.png`

test('capture README screenshots', async ({ page, request }) => {
  const username = `demo_${Date.now()}`
  const password = 'demo12345'

  // Auth page (logged out)
  await page.goto('/auth')
  await page.screenshot({ path: shot('auth') })

  // Register, then log in.
  await page.getByTestId('tab-register').click()
  await expect(page.getByTestId('submit-register')).toBeVisible()
  await page.getByPlaceholder('Your username').fill(username)
  await page.getByPlaceholder('Your password', { exact: true }).fill(password)
  await page.getByPlaceholder('Repeat your password').fill(password)
  await page.getByTestId('submit-register').click()
  // Wait for the login form to finish entering (AnimatePresence) before filling.
  await expect(page.getByTestId('submit-login')).toBeVisible()
  await page.getByPlaceholder('Your password', { exact: true }).fill(password)
  await page.getByTestId('submit-login').click()
  await expect(page.getByRole('heading', { name: 'Browse Manga' })).toBeVisible()

  const token = (await page.evaluate(() => {
    const raw = localStorage.getItem('mangahub-auth')
    return raw ? (JSON.parse(raw).state.token as string) : ''
  })) as string

  // Ensure there's at least one manga to screenshot.
  await page.waitForTimeout(800)
  let mangaId = await page.locator('a[href^="/manga/"]').first().getAttribute('href').catch(() => null)
  if (!mangaId) {
    mangaId = `demo-manga-${Date.now()}`
    await request.post(`${API_URL}/manga`, {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        id: mangaId,
        title: 'Demo Manga',
        author: 'Demo Author',
        genres: ['Action', 'Adventure'],
        status: 'ongoing',
        total_chapters: 42,
        description: 'A demo entry created so the README screenshots render.',
      },
    })
    await page.reload()
    await page.waitForTimeout(600)
    mangaId = `/manga/${mangaId}`
  }
  await page.screenshot({ path: shot('browse') })

  // Manga detail — open it and add it to the library.
  await page.goto(mangaId.startsWith('/') ? mangaId : `/manga/${mangaId}`)
  await page.waitForTimeout(600)
  const addBtn = page.getByRole('button', { name: 'Add to Library' })
  if (await addBtn.isVisible().catch(() => false)) await addBtn.click()
  await page.waitForTimeout(300)
  await page.screenshot({ path: shot('manga-detail') })

  // Library
  await page.goto('/library')
  await page.waitForTimeout(600)
  await page.screenshot({ path: shot('library') })

  // Chat
  await page.goto('/chat')
  await page.getByText('Connected').waitFor({ timeout: 10_000 }).catch(() => {})
  await page.screenshot({ path: shot('chat') })

  // Feed + Profile
  await page.goto('/feed')
  await page.waitForTimeout(500)
  await page.screenshot({ path: shot('feed') })

  await page.goto('/profile')
  await page.waitForTimeout(500)
  await page.screenshot({ path: shot('profile') })
})
