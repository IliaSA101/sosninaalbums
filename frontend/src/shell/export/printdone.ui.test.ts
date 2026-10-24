import { describe, expect, it } from 'vitest'

// Окно «Готово» типографии (081): заголовок, путь на сайте и кнопка приходят в плане из Go, UI их только рисует.
// DOM в тестах нет — проверяем исходник окна.
const files = import.meta.glob('/src/shell/export/PrintDone.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>
const done = files['/src/shell/export/PrintDone.svelte'] ?? ''

describe('окно «Готово» типографии', () => {
  it('done window title and site from plan: тексты из плана, своих названий типографий нет', () => {
    expect(done).toContain('title={plan?.done')
    expect(done).toContain('{plan.site}')
    expect(done).toContain('{plan.siteButton}')
    const own = done
      .replace(/<script[\s\S]*?<\/script>/, '')
      .replace(/<style[\s\S]*$/, '')
      .match(/Фотосфер|Фабрик/g)
    expect(own).toBeNull()
  })
  it('кнопка сайта: Фабрика — OpenFabrika, иначе OpenFsfera; «Открыть папку» и «Готово» остаются', () => {
    expect(done).toMatch(/plan\?\.kind === 'fabrika' \? Api\.OpenFabrika\(\) : Api\.OpenFsfera\(\)/)
    expect(done).toContain('Api.ShowInExplorer()')
    expect(done).toContain('Готово</button>')
  })
  it('закрытие сбрасывает памятку, сводку и план', () => {
    const close = done.match(/function close\(\) \{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    expect(close).toContain('jobs.exportMemo = null')
    expect(close).toContain('jobs.exportNote =')
    expect(close).toContain('jobs.exportPrint = null')
  })
})
