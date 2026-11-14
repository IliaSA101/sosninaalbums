// Тексты окна «Альбом из новой версии» (112, design §5). Чистые функции: вид и вызовы — NewerAskDialog.svelte.
import type { NewerInfo } from '../../base/core/types'

export interface NewerText {
  title: string
  lead: string // что случилось
  next: string // что делать
  close: string
  download: string
  open: string // подпись «Открыть в 0.26»; пусто — кнопки нет (версии на компьютере нет)
}

// Номера «0.25» / «0.25.1» → [major, minor, patch]; не номер — null. Сравнение по числам, не по строкам («0.9» < «0.10»).
function parts(v: string): [number, number, number] | null {
  const m = /^(\d+)\.(\d+)(?:\.(\d+))?$/.exec(v)
  return m ? [Number(m[1]), Number(m[2]), Number(m[3] ?? 0)] : null
}

// Версия из файла действительно новее своей. Неизвестная, непонятная или не новее (файл новее по формату, а
// записанная версия — нет: сборка разработчика, правка руками) — не новее: номера в тексте только запутают.
// Своя версия неизвестна — верим файлу.
function isNewer(need: string, mine: string): boolean {
  const n = parts(need)
  if (!n) return false
  const m = parts(mine)
  if (!m) return true
  for (let k = 0; k < 3; k++) if (n[k] !== m[k]) return n[k] > m[k]
  return false
}

// Откуда файл: «сохранён в Sosnina Albums 0.26, а эта программа — 0.25»; версия в файле неизвестна — без номеров.
function saved(i: NewerInfo): string {
  const what = i.draft ? 'Черновик' : `«${i.name}»`
  if (!isNewer(i.need, i.mine)) return `${what} сохранён более новой версией программы.`
  const mine = i.mine ? `, а эта программа — ${i.mine}` : ''
  return `${what} сохранён в Sosnina Albums ${i.need}${mine}. Открыть его может версия ${i.need} или новее.`
}

export function newerAskText(i: NewerInfo): NewerText {
  const lead = i.draft ? `${saved(i)} Здесь он не открывается и не изменится.` : saved(i)
  const next = i.have
    ? i.draft
      ? `Версия ${i.have} уже есть на компьютере. Эта программа закроется, запустится ${i.have}.`
      : `Версия ${i.have} уже есть на компьютере. Эта программа закроется, альбом откроется в ${i.have}.`
    : 'Скачайте новую версию — альбомы и настройки сохранятся.'
  return {
    title: i.draft ? 'Черновик из новой версии' : 'Альбом из новой версии',
    lead,
    next,
    close: 'Закрыть',
    download: 'Скачать',
    open: i.have ? `Открыть в ${i.have}` : '',
  }
}
