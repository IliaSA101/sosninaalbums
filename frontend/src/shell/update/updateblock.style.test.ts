import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const css = readFileSync(new URL('./UpdateBlock.svelte', import.meta.url), 'utf8').split('<style>')[1] ?? ''

// happy-dom раскладку не считает: проверяем, что у списка есть предел высоты и свой скролл (1024×700; размеры — замером в браузере).
describe('UpdateBlock: стили', () => {
  it('список «Что нового» ограничен по высоте и прокручивается сам, кнопки окна остаются видны', () => {
    const notes = /\.notes\s*\{([^}]*)\}/.exec(css)?.[1] ?? ''
    expect(notes).toMatch(/max-height:\s*min\(\d+px,\s*\d+vh\)/)
    expect(notes).toMatch(/overflow-y:\s*auto/)
  })
})
