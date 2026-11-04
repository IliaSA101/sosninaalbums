// Живой показ цвета (109, 6): пока курсор на образце окна цвета или тянут оттенок, слой холста рисует этот цвет,
// а не цвет из View. Ничего не пишет в store, не зовёт Go и не трогает историю. Образец — fadeLive и adjlive.
// Окно знает только ключи (строки): кто какой ключ читает — решает место (таблица в design 109 §6.1).
// Цвет узора, стикера и рамочки рисует Go картинкой: там показ — подмена i-й краски в адресе, после паузы
// наведения, чтобы проход мыши через 8 образцов не давал 8 запросов.
import { normalizeHex } from './palette'

interface Live {
  keys: string[]
  hex: string
  ink?: number // номер краски для адресов Go; нет — просто CSS-цвет
}

// Пауза наведения перед запросом картинки у Go.
export const INK_DELAY = 120

// Адрес Go с i-й краской hex ('#rrggbb'). Меняется только значение i; нет i=, пустой список или номер вне него —
// адрес как был. Остальные параметры и их порядок не трогаются.
export function withInk(url: string, ink: number, hex: string): string {
  const h = normalizeHex(hex)
  const q = url.indexOf('?')
  if (!h || q < 0 || !Number.isInteger(ink) || ink < 0) return url
  const m = /([?&])i=([^&#]*)/.exec(url.slice(q))
  if (!m || m[2] === '') return url
  const list = m[2].split(',')
  if (ink >= list.length) return url
  list[ink] = h.slice(1)
  const at = q + m.index + m[1].length
  return url.slice(0, at) + 'i=' + list.join(',') + url.slice(at + 2 + m[2].length)
}

class ColorLive {
  cur = $state.raw<Live | null>(null) // CSS-цвет, сразу
  inkCur = $state.raw<Live | null>(null) // краска для адресов Go, после паузы
  private timer: ReturnType<typeof setTimeout> | undefined
  // Очередь коммитов: показ держится до ответа Go, старый цвет не мелькает. Как FadeLive.commit.
  private chain: Promise<unknown> = Promise.resolve()
  private pending = 0

  // keys — кого показ касается; ink — номер краски (для узора, стикера, рамочки).
  set(keys: string[], hex: string, ink?: number) {
    const h = normalizeHex(hex)
    if (!h || keys.length === 0) return
    const live: Live = { keys, hex: h, ink }
    this.cur = live
    this.stopTimer()
    if (ink === undefined) {
      this.inkCur = null
      return
    }
    this.timer = setTimeout(() => {
      this.timer = undefined
      if (this.cur === live) this.inkCur = live
    }, INK_DELAY)
  }

  // Показ снимается, только когда нет коммитов в пути: иначе слой мигнёт старым цветом.
  clear() {
    if (this.pending > 0) return
    this.stopTimer()
    this.cur = null
    this.inkCur = null
  }

  private stopTimer() {
    clearTimeout(this.timer)
    this.timer = undefined
  }

  // Живой CSS-цвет ключа или fallback из View.
  color(key: string, fallback: string): string {
    const c = this.cur
    return c && c.keys.includes(key) ? c.hex : fallback
  }

  // Стиль (текст) с живым цветом ключа; нет показа — тот же объект.
  style<T extends { color: string }>(key: string, st: T): T {
    const c = this.cur
    return c && c.keys.includes(key) ? { ...st, color: c.hex } : st
  }

  // Адрес Go с подменённой краской, если показ про этот ключ и пауза прошла.
  inkUrl(key: string, url: string): string {
    const c = this.inkCur
    return c && c.ink !== undefined && c.keys.includes(key) ? withInk(url, c.ink, c.hex) : url
  }

  // Правка в очередь; показ снимается после последней (окно зовёт onpreview(null) по её промису).
  commit(run: () => unknown): Promise<unknown> {
    this.pending++
    const p = this.chain.then(run, run)
    this.chain = p.catch(() => {})
    return this.chain.finally(() => {
      this.pending--
      this.clear()
    })
  }
}

export const colorLive = new ColorLive()

// Цвета двух страниц листа с учётом показа: фон листа (`bg:<лист>:spread|left|right`) или цвет страниц альбома
// (`album` — только страницы без своего цвета, bgSet[i] === '').
export function pageBg(sheet: string, bg: [string, string], bgSet: [string, string]): [string, string] {
  const c = colorLive.cur
  if (!c || c.ink !== undefined) return bg
  const own = (scope: string) => c.keys.includes(`bg:${sheet}:${scope}`)
  const pick = (i: 0 | 1) =>
    own('spread') || own(i === 0 ? 'left' : 'right') || (c.keys.includes('album') && bgSet[i] === '') ? c.hex : bg[i]
  return [pick(0), pick(1)]
}
