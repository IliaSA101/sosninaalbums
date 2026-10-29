import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// Порядок окон в стеке — по порядку монтирования (modalstack): на первом запуске стартовое окно и «Альбом из новой
// версии» появляются за один проход, и второе должно лечь сверху (0.25-n2ui).
describe('порядок в App.svelte', () => {
  it('NewerAskDialog смонтирован после StartDialog', () => {
    const app = readFileSync(new URL('../../App.svelte', import.meta.url), 'utf8')
    expect(app.indexOf('<StartDialog />')).toBeGreaterThan(-1)
    expect(app.indexOf('<NewerAskDialog />')).toBeGreaterThan(app.indexOf('<StartDialog />'))
  })
})
