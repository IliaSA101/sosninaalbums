import { describe, expect, it } from 'vitest'

// Страж имён файлов: Windows и macOS не различают регистр. Пара `MyColors.svelte` + `mycolors.svelte.ts` на Linux
// живёт, а импорт `./mycolors.svelte` там находит компонент — svelte-check и сборка exe падают (0.24).
describe('имена файлов без коллизий по регистру', () => {
  const paths = Object.keys(import.meta.glob('/src/**/*'))

  // Чистая проверка: список путей → список нарушений (тестируется и на заведомо плохом наборе).
  function collisions(list: string[]): string[] {
    const bad: string[] = []
    const byLower = new Map<string, string>()
    for (const p of list) {
      const prev = byLower.get(p.toLowerCase())
      if (prev !== undefined && prev !== p) bad.push(`${prev} и ${p} — одно имя без учёта регистра`)
      byLower.set(p.toLowerCase(), p)
    }
    // Модуль состояния `x.svelte.ts` против компонента `X.svelte`: импорт `./x.svelte` разрешится в компонент.
    for (const p of list) {
      const m = /^(.*\.svelte)\.(?:ts|js)$/.exec(p)
      if (!m) continue
      const comp = list.find((q) => q !== m[1] && q.toLowerCase() === m[1].toLowerCase())
      if (comp) bad.push(`${p}: импорт './${m[1].split('/').pop()}' на нечувствительной к регистру ФС найдёт ${comp}`)
    }
    return bad
  }

  it('в frontend/src нет двух путей, равных без учёта регистра', () => {
    expect(collisions(paths), 'переименовать модуль состояния (не компонент)').toEqual([])
  })

  it('страж ловит пару компонента и модуля состояния', () => {
    const bad = collisions(['/src/a/MyColors.svelte', '/src/a/mycolors.svelte.ts'])
    expect(bad).toHaveLength(1)
    expect(collisions(['/src/a/Foo.ts', '/src/a/foo.ts'])).toHaveLength(1)
    expect(collisions(['/src/a/Foo.svelte', '/src/a/foostate.svelte.ts'])).toEqual([])
  })

  it('страж видит файлы', () => {
    expect(paths).toContain('/src/base/core/platform.ts')
    expect(paths.length).toBeGreaterThan(100)
  })
})
