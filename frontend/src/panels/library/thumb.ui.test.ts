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
  it('✓ «Уже в альбоме» и звёзды стоят вне ветки заглушки — остаются при пропаже файла', () => {
    const tail = body.slice(body.indexOf('{/if}', body.indexOf('{:else}')))
    expect(tail).toContain('store.used.has(p.id)')
    expect(tail).toContain('class="check"')
    expect(tail).toContain('mark.rating > 0')
    expect(tail).toContain('class="stars"')
  })
  it('подпись «нет файла» под значком; адрес миниатюры несёт метку повтора lost.retryOf', () => {
    expect(body).toMatch(/<MissPhoto[^>]*label="нет файла"/)
    expect(body).toContain('thumbUrl(p.id, lost.retryOf(p.id))')
  })
  it('рамка заглушки: выбранная — акцентная, как у img', () => {
    expect(src).toContain('.thumb.selected .gone')
  })
})
