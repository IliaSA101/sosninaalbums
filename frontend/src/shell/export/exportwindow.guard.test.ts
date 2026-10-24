import { describe, expect, it } from 'vitest'

// Окно «Экспорт» во время хода (073). DOM в тестах нет (компоненты не монтируются), поэтому разметку и глобальные
// правила стиля проверяем по исходникам: именно так пропал крестик — общий класс `.run` попал под правило сбора.
const files = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const src = (name: string): string => files[`/src/${name}`] ?? ''

const classesIn = (text: string): Set<string> => {
  const out = new Set<string>()
  for (const m of text.matchAll(/class="([^"]*)"/g)) for (const c of m[1].split(/\s+/)) if (c) out.add(c)
  for (const m of text.matchAll(/class:([\w-]+)/g)) out.add(m[1])
  return out
}
// Классы в :global(...:has(.x)) и :global(...):has(.x) — правило действует на всё приложение.
const hasTargets = (text: string): string[] => {
  const css = text.replace(/^[\s\S]*?<style[^>]*>/, '')
  const out: string[] = []
  for (const m of css.matchAll(/:global\([^)]*:has\(\.([\w-]+)\)|:global\([^)]*\):has\(\.([\w-]+)\)/g)) out.push(m[1] ?? m[2])
  return out
}

describe('глобальные :has(.класс) не задевают чужие окна', () => {
  // Правила `:global(.box.wide):has(.x)` меняют только ширину окна с `wide`; совпадения ниже проверены глазами:
  // .opts в CollectDialog и главном окне PacksDialog — не wide; .guide в редакторе — направляющие вне окон.
  const ok = new Set(['.opts|ExportGroup.svelte', '.guide|Guide.svelte'])
  it('класс из правила встречается в разметке только у владельца правила', () => {
    const bad: string[] = []
    for (const [path, text] of Object.entries(files)) {
      for (const cls of hasTargets(text)) {
        if (ok.has(`.${cls}|${path.split('/').pop()}`)) continue
        for (const [other, otherText] of Object.entries(files)) {
          if (other !== path && classesIn(otherText.replace(/<style[\s\S]*$/, '')).has(cls)) bad.push(`${path}: .${cls} есть и в ${other}`)
        }
      }
    }
    expect(bad).toEqual([])
  })
  it('страж видит правило сбора и свои классы окон', () => {
    expect(hasTargets(src('shell/collect/CollectDialog.svelte'))).toContain('collect-run')
    expect(classesIn(src('shell/export/ExportRun.svelte')).has('exrun')).toBe(true)
    expect(classesIn(src('shell/collect/CollectDialog.svelte')).has('collect-run')).toBe(true)
  })
  it('крестик скрывает только окно сбора: у хода выгрузки правила на .x нет', () => {
    const hidesX = Object.entries(files).filter(([, t]) => /:has\([^)]*\)\s*\.x\b/.test(t))
    expect(hidesX.map(([p]) => p)).toEqual(['/src/shell/collect/CollectDialog.svelte', '/src/shell/dialogs/OpenProgress.svelte'])
    expect(src('shell/export/ExportRun.svelte')).not.toMatch(/\.x\b/)
  })
})

describe('разметка хода выгрузки', () => {
  const dialog = src('shell/export/ExportDialog.svelte')
  const modal = src('base/modal/Modal.svelte')
  const run = src('shell/export/ExportRun.svelte')
  it('при jobs.exporting вместо настроек — ExportRun, окно обёрнуто в Modal с onclose = close', () => {
    expect(dialog).toMatch(/<Modal title="Экспорт" onclose=\{close\}[^>]*>\s*\{#if jobs\.exporting\}\s*<ExportRun \/>\s*\{:else\}/)
  })
  it('close только прячет окно (exportOpen = false), выгрузку не трогает', () => {
    expect(dialog).toMatch(/function close\(\) \{\s*jobs\.exportOpen = false\s*\}/)
  })
  it('Modal рисует крестик .x с onclose безусловно', () => {
    expect(modal).toMatch(/<button class="x" onclick=\{onclose\}/)
    expect(modal).not.toMatch(/\{#if[^}]*\}\s*<button class="x"/)
  })
  it('«Остановить» зовёт jobs.stopExport, подпись «Останавливаем…» по jobs.exportStopping', () => {
    expect(run).toContain('onclick={() => jobs.stopExport()}')
    expect(run).toContain("stopping ? 'Останавливаем…' : 'Остановить'")
    expect(run).toContain('jobs.exportStopping')
  })
})
