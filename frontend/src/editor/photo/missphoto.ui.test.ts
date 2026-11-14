import { describe, expect, it } from 'vitest'

// Заглушка «файла нет» в крупном просмотре и на месте разворота (088). DOM в тестах нет — проверяем исходники.
const files = import.meta.glob('/src/editor/photo/{PhotoPreview,Cell}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>
const preview = files['/src/editor/photo/PhotoPreview.svelte'] ?? ''
const cell = files['/src/editor/photo/Cell.svelte'] ?? ''
const markup = (s: string) => s.replace(/<script[\s\S]*?<\/script>/, '').replace(/<style[\s\S]*$/, '')

describe('MissPhoto: подпись', () => {
  const miss = import.meta.glob('/src/base/ui/MissPhoto.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
    string,
    string
  >
  const src = miss['/src/base/ui/MissPhoto.svelte'] ?? ''
  it('подпись только если задана; на мелкой плитке скрыта (@container), подсказка остаётся', () => {
    expect(src).toContain('{#if label}<span class="lbl">{label}</span>{/if}')
    expect(src).toMatch(/\.lbl \{\s*display: none;/)
    expect(src).toMatch(/@container \(min-width: 90px\) \{\s*\.lbl \{\s*display: block;/)
    expect(src).toContain('aria-label={tip}')
    expect(src).toContain('title={tip}')
  })
  it('Cell и PhotoPreview подпись не задают — там заглушка как раньше', () => {
    expect(markup(cell)).not.toMatch(/<MissPhoto[^>]*label=/)
    expect(markup(preview)).not.toMatch(/<MissPhoto[^>]*label=/)
  })
})

describe('PhotoPreview: файла нет', () => {
  it('заглушка при сбое загрузки или store.missing', () => {
    expect(preview).toContain('let failed = $state(false)')
    expect(preview).toMatch(/const gone = \$derived\(failed \|\| \(!!id && isGone\(id, store\.missing, store\.unread\)\)\)/)
    expect(markup(preview)).toMatch(/\{#if gone\}\s*<div class="gone"[^>]*><MissPhoto/)
  })
  it('ошибка загрузки обеих картинок (прямой и повёрнутой) → onFail: failed и autocheck.soon(true)', () => {
    const onFail = preview.match(/function onFail\(\) \{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    expect(onFail).toContain('failed = true')
    expect(onFail).toContain('autocheck.soon(true)')
    expect(markup(preview).match(/onerror=\{onFail\}/g)).toHaveLength(2)
  })
  it('смена id сбрасывает сбой и размер прошлого фото', () => {
    const eff = preview.match(/\$effect\(\(\) => \{\s*if \(id\) \{[\s\S]*?\n {2}\}\)/)?.[0] ?? ''
    expect(eff).toContain('nat = { w: 0, h: 0 }')
    expect(eff).toContain('failed = false')
  })
})

describe('не читается (117): причина из store.unread доходит до подсказки', () => {
  it('Cell и PhotoPreview передают store.unread в missTip', () => {
    expect(cell).toContain('missTip(id, store.missingNames, store.unread)')
    expect(preview).toContain("missTip(id, { [id]: photo?.name ?? '' }, store.unread)")
  })
})

describe('Cell: повтор миниатюры', () => {
  it('адрес несёт метку lost.retryOf: после снятия причины «не читается» запрос идёт заново мимо кэша', () => {
    expect(markup(cell)).toContain('src={thumbUrl(id, lost.retryOf(id))}')
  })
})

describe('Cell: файла нет', () => {
  it('missing → MissPhoto вместо img', () => {
    expect(cell).toContain('const missing = $derived(isGone(id, store.missing, store.unread))')
    expect(markup(cell)).toMatch(/\{#if missing\}\s*<MissPhoto[\s\S]*?\{:else\}/)
  })
  it('onerror миниатюры → autocheck.soon(true)', () => {
    expect(markup(cell)).toContain('onerror={() => autocheck.soon(true)}')
  })
})
