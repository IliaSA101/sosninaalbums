import { describe, expect, it } from 'vitest'

// Плитка ленты при пропавшем файле (088): DOM в тестах нет — проверяем исходник Thumb.
const files = import.meta.glob('/src/panels/library/Thumb.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>
const src = files['/src/panels/library/Thumb.svelte'] ?? ''
const body = src.replace(/<script[\s\S]*?<\/script>/, '').replace(/<style[\s\S]*$/, '')

describe('Thumb: файл пропал', () => {
  it('lost → MissPhoto вместо img; иначе img с onerror → lost.mark', () => {
    const branch = body.match(/\{#if lost\.has\(p\.id\)\}([\s\S]*?)\{:else\}([\s\S]*?)\{\/if\}/)
    expect(branch).not.toBeNull()
    expect(branch?.[1]).toContain('<MissPhoto')
    expect(branch?.[1]).not.toContain('<img')
    expect(branch?.[2]).toContain('<img')
    expect(branch?.[2]).toContain('onerror={() => lost.mark(p.id)}')
  })
  it('число вхождений (098) и звёзды стоят вне ветки заглушки — остаются при пропаже файла', () => {
    const tail = body.slice(body.indexOf('{/if}', body.indexOf('{:else}')))
    expect(tail).toContain('uses > 0')
    expect(tail).toContain('class="check"')
    expect(tail).toContain('mark.rating > 0')
    expect(tail).toContain('class="stars"')
  })
  it('значок ленты (098-6, 0.23-q8): 1 → ✓, 2 и больше → число, 0 → значка нет; бледность — по store.used', () => {
    expect(src).toContain('store.useCount(p.id)')
    expect(body).toContain('{#if uses > 0}')
    expect(body).toContain("{uses === 1 ? '✓' : uses}</span>")
    expect(body).toContain('class:used={store.used.has(p.id)}')
  })
  it('значок ленты: правило по значению — 1 даёт ✓, 2 и 3 дают число, 0 не даёт значка', () => {
    const m = body.match(/\{#if uses > 0\}<span[^>]*>\{(uses === 1 \? '✓' : uses)\}<\/span>\{\/if\}/)
    expect(m).not.toBeNull()
    const sign = new Function('uses', `return ${m?.[1]}`) as (uses: number) => string | number
    expect(sign(1)).toBe('✓')
    expect(sign(2)).toBe(2)
    expect(sign(3)).toBe(3)
    expect(sign(12)).toBe(12)
    const shown = (uses: number) => (uses > 0 ? sign(uses) : null)
    expect(shown(0)).toBeNull()
  })
  it('подпись «нет файла» под значком; адрес миниатюры несёт метку повтора lost.retryOf', () => {
    expect(body).toMatch(/<MissPhoto[^>]*label="нет файла"/)
    expect(body).toContain('thumbUrl(p.id, lost.retryOf(p.id))')
  })
  it('рамка заглушки: выбранная — акцентная, как у img', () => {
    expect(src).toContain('.thumb.selected .gone')
  })
})
