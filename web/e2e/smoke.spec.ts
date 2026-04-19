import { fileURLToPath } from 'node:url'

import { expect, test } from '@playwright/test'

const sampleFile = fileURLToPath(new URL('../../testdata/sample.txt', import.meta.url))

test('processes a document end-to-end in the browser', async ({ page }) => {
  await page.goto('/')

  await expect(
    page.getByRole('heading', {
      name: '将 AI 生成的稿件润色为自然、像人写的文档。',
    }),
  ).toBeVisible()

  await page.getByTestId('workspace-mode-en').click()
  await page.getByTestId('workspace-file-input').setInputFiles(sampleFile)
  await expect(page.getByText('sample.txt 已就绪,可以上传。')).toBeVisible()

  await page.getByTestId('workspace-primary-action').click()

  await expect(page.getByTestId('workspace-status-pill')).toHaveText(/已完成/, {
    timeout: 180_000,
  })

  await expect(page.getByTestId('workspace-download-txt')).toBeVisible()
  await expect(page.getByTestId('workspace-download-docx')).toBeVisible()

  const previewBody = page.getByTestId('workspace-preview-body')
  await expect(previewBody.locator('p').first()).toBeVisible()

  const previewText = ((await previewBody.textContent()) ?? '').trim()
  expect(previewText.length).toBeGreaterThan(80)
  expect(previewText).not.toContain('[INPUT TEXT]')

  await expect(page.getByTestId('workspace-preview-meta')).toContainText('已处理')

  await page.getByRole('tab', { name: '卡片评审' }).click()

  const reviewCards = page.getByTestId('workspace-review-card')
  await expect(reviewCards.first()).toBeVisible()
  await expect(reviewCards.first()).toContainText('原文')
  await expect(reviewCards.first()).toContainText('润色后')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByTestId('workspace-download-txt').click(),
  ])
  expect(download.suggestedFilename()).toMatch(/\.txt$/)
})
