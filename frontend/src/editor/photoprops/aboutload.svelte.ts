import type { app, model } from '../../../wailsjs/go/models'

// Сведения о снимке для карточки (084): загрузка с отбрасыванием поздних ответов и чистые решения.

export type About = app.PhotoAbout

// Вхождений в строке показываем не больше (decisions №6): остальные — «ещё N».
export const USES_MAX = 6

type Use = Pick<model.PhotoUse, 'sheet' | 'label'>

export interface UseLink {
  sheet: number
  link: string // «Разворот 3» — текст ссылки
  kind: string // «место» — после ссылки обычным текстом
}

// «Разворот 3 — место» → ссылка «Разворот 3» и вид «место». Без тире — вся подпись ссылкой.
export function splitUse(u: Use): UseLink {
  const i = u.label.indexOf(' — ')
  if (i < 0) return { sheet: u.sheet, link: u.label, kind: '' }
  return { sheet: u.sheet, link: u.label.slice(0, i), kind: u.label.slice(i + 3) }
}

export function usesShown(uses: readonly Use[], max = USES_MAX): { shown: UseLink[]; more: number } {
  return { shown: uses.slice(0, max).map(splitUse), more: Math.max(0, uses.length - max) }
}

// Фото из ленты для раздела панели деталей: панель открыта и в ленте выделено ровно одно фото. То же фото,
// что уже показано в карточке выделенного (cardId), второй раз не выводим. Зону последнего нажатия не смотрим:
// выделение из справочника или с клавиатуры её не меняет.
export function libPhotoOf(selected: readonly string[], detailsOpen: boolean, cardId = ''): string {
  return detailsOpen && selected.length === 1 && selected[0] !== cardId ? selected[0] : ''
}

// Загрузчик: номер запроса отбрасывает ответ, пришедший после более нового. Смена фото сразу очищает
// данные (чужие сведения не показываем), перезапрос того же фото (правка альбома) держит прежние до ответа.
export class AboutLoader {
  data = $state.raw<About | null>(null)
  private req = 0
  private cur = ''
  private key = ''

  constructor(private readonly fetch: (id: string) => Promise<About>) {}

  // Условие перезапроса: меняется фото или ревизия альбома (правка). Тот же ключ — запроса нет.
  sync(id: string, rev: number): Promise<void> {
    const k = `${id}@${rev}`
    if (k === this.key) return Promise.resolve()
    this.key = k
    return this.load(id)
  }

  async load(id: string): Promise<void> {
    const my = ++this.req
    if (id !== this.cur) {
      this.cur = id
      this.data = null
    }
    if (!id) return
    try {
      const a = await this.fetch(id)
      if (my === this.req) this.data = a
    } catch {
      // Сведения не стоят тоста: блока просто нет.
      if (my === this.req) this.data = null
    }
  }
}
