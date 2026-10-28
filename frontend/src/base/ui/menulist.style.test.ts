import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// happy-dom не применяет стили компонента и :hover, поэтому проверяем сами правила в тексте компонента.
const css = readFileSync(new URL('./MenuList.svelte', import.meta.url), 'utf8').split('<style>')[1] ?? ''

// Тело правила по точному селектору (с учётом списка селекторов через запятую).
function body(selector: string): string {
  const esc = selector.replace(/[.*+?^${}()|[\]\\:]/g, '\\$&')
  const m = new RegExp(`(?:^|[,}\\s])${esc}\\s*(?:,[^{]*)?\\{([^}]*)\\}`, 'm').exec(css)
  return m?.[1] ?? ''
}

describe('MenuList: одна подсветка (регресс QA 0.24)', () => {
  it('пункт под мышью без .cur фон не меняет: глобальное button:hover из style.css перебито', () => {
    expect(body('button:hover:not(:disabled)')).toMatch(/background:\s*transparent/)
  })

  it('.cur держит свой фон и под курсором', () => {
    expect(body('button.cur:hover:not(:disabled)')).toMatch(/background:\s*#38383c/)
  })
})
