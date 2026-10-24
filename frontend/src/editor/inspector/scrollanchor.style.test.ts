import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// happy-dom не прокручивает и не якорит прокрутку, поэтому проверяем сами правила в тексте компонентов.
// QA 0.26: бросок фото на свёрнутый «Фон», панель прокручена ниже группы — браузер при раскрытии группы сам
// сдвигал прокрутку на её высоту (149 → 319), зона уходила выше видимой области.
const style = (url: string) => readFileSync(new URL(url, import.meta.url), 'utf8').split('<style>')[1] ?? ''

function body(css: string, selector: string): string {
  const esc = selector.replace(/[.*+?^${}()|[\]\\:]/g, '\\$&')
  return new RegExp(`(?:^|\\})\\s*${esc}\\s*\\{([^}]*)\\}`, 'm').exec(css)?.[1] ?? ''
}

describe('прокрутка карточек: без якорения браузера', () => {
  it('панель деталей (.scroll) не якорит прокрутку', () => {
    expect(body(style('../../panels/details/DetailsPanel.svelte'), '.scroll')).toMatch(/overflow-anchor:\s*none/)
  })
  it('плавающая карточка (.insp) не якорит прокрутку', () => {
    expect(body(style('./Inspector.svelte'), '.insp')).toMatch(/overflow-anchor:\s*none/)
  })
})
