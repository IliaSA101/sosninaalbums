import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// happy-dom не считает раскладку, поэтому проверяем сами правила в тексте компонента (как menulist.style.test.ts).
// QA 0.26: в плавающей карточке (200 px) подписи «Прозрачность», «Масштаб, %», «Наклон, °» резались многоточием.
const css = readFileSync(new URL('./TiltRow.svelte', import.meta.url), 'utf8').split('<style>')[1] ?? ''

// Тело правила по точному селектору.
function body(selector: string): string {
  const esc = selector.replace(/[.*+?^${}()|[\]\\:]/g, '\\$&')
  return new RegExp(`(?:^|\\})\\s*${esc}\\s*\\{([^}]*)\\}`, 'm').exec(css)?.[1] ?? ''
}

describe('TiltRow: подпись в плавающей карточке', () => {
  it('подпись не сжимается и не обрезается: без overflow и ellipsis в общем правиле', () => {
    const lbl = body('.lbl')
    expect(lbl).toMatch(/flex:\s*none/)
    expect(lbl).not.toMatch(/overflow|ellipsis/)
  })

  it('ячейка подписи не уже самой подписи: число и ⊗ при нехватке места уходят строкой ниже', () => {
    expect(body('.cell')).toMatch(/flex:\s*1 0 auto/)
  })

  it('панель деталей: ячейка 104 px, как прежде, — дорожки с одного места; подпись там при нужде обрезается', () => {
    expect(body('.line .cell')).toMatch(/flex:\s*0 0 104px/)
    expect(body('.line .lbl')).toMatch(/text-overflow:\s*ellipsis/)
    expect(body('.line .num')).toMatch(/width:\s*64px/)
  })
})
