// Узор на фоне (048): чистые решения для слоя, карточки и вызовов Go. Картинку рисует Go (/media/gen),
// здесь только размер запроса и перевод единиц; ключи, цвета и зажим — у Go.
import type { BackdropScope, PatternSpec, PatternView, SpreadView } from '../../base/core/types'

// Масштаб в Go — доля 0.25…4 (0 — 1), в карточке — проценты 50…200.
export const SCALE_MIN = 50
export const SCALE_MAX = 200

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v))

// Доля → проценты ползунка; нет поля (0 в модели) — 100 %, мусор тоже.
export function scalePct(scale: number | undefined): number {
  return scale && Number.isFinite(scale) ? clamp(Math.round(scale * 100), SCALE_MIN, SCALE_MAX) : 100
}

// Проценты ползунка → доля для Go, зажатая окном ползунка.
export function pctScale(pct: number): number {
  return Number.isFinite(pct) ? clamp(Math.round(pct), SCALE_MIN, SCALE_MAX) / 100 : 1
}

// Ширина PNG узора в px: ширина области на экране × dpr, шаг 256 (зум не просит новую картинку на каждый пиксель),
// не больше 4096 (предел /media/gen). Мусор и крошечное — минимальный шаг.
export const PX_STEP = 256
export const PX_MAX = 4096
export function patternPx(cssWidth: number, dpr: number): number {
  const w = cssWidth * (dpr > 0 ? dpr : 1)
  if (!Number.isFinite(w) || w <= 0) return PX_STEP
  return clamp(Math.ceil(w / PX_STEP) * PX_STEP, PX_STEP, PX_MAX)
}

// Ширина PNG узора слоя: доля области в листе × ширина листа НА ЭКРАНЕ (в зуме холста Stage уже передаёт её с зумом,
// поэтому на 348 % запрос крупнее, чем на 100 %). Нет ширины листа в мм — минимальный шаг.
export function layerPx(areaW: number, sheetW: number, onScreenW: number, dpr: number): number {
  return patternPx(sheetW > 0 ? (areaW / sheetW) * onScreenW : 0, dpr)
}

// Адрес картинки: url от Go всегда с «?», px дописывает интерфейс.
export const patternSrc = (url: string, px: number): string => `${url}${url.includes('?') ? '&' : '?'}px=${px}`

export const patternOf = (s: Pick<SpreadView, 'patterns'> | undefined, scope: BackdropScope): PatternView | undefined =>
  s?.patterns?.find((p) => p.scope === scope)

// Цвета узора для пикеров: столько, сколько пикеров у генератора (Go дополнил их цветами по умолчанию).
export const inksOf = (p: Pick<PatternView, 'pat' | 'inks'>): string[] => (p.pat.inks ?? []).slice(0, p.inks)

// Узор для SetPattern: прежний с правкой; ключ генератора Go не принимает, но шлём как есть (он берёт слот).
export function patched(p: Pick<PatternView, 'pat'>, patch: Partial<PatternSpec>): PatternSpec {
  return { ...p.pat, ...(p.pat.inks ? { inks: [...p.pat.inks] } : {}), ...patch }
}

// Новая «перестановка» случайного узора: другое число, не 0 (0 в модели — «по умолчанию»).
export function newSeed(prev: number | undefined, rnd: () => number = Math.random): number {
  for (let i = 0; i < 8; i++) {
    const n = Math.floor(rnd() * 0x100000000) >>> 0
    if (n !== 0 && n !== (prev ?? 0)) return n
  }
  return ((prev ?? 0) + 1) >>> 0 || 1
}

// Сессия merge для Go: [A-Za-z0-9-], ≤ 32 знаков; стрелки ползунка в одном фокусе — один шаг отмены.
export const patSession = (t = Date.now()) => `pt-${t.toString(36)}`

// Название узора в карточке: пустое имя — генератор из новой версии программы.
export const PATTERN_NEWER = 'Нужна новая версия программы'

// Подпись области, когда узоров на листе два (левая и правая страницы).
export function patternCaption(scope: BackdropScope, count: number, labels: Record<BackdropScope, string>): string {
  return count > 1 ? `Узор — ${labels[scope].toLowerCase()}` : 'Узор'
}

// Ключ «этот же узор той же области»: новый объект SpreadView после ответа Go (тот же id, scope, генератор) его не меняет.
export const patKey = (spread: string, p: Pick<PatternView, 'scope' | 'pat'>): string => `${spread}|${p.scope}|${p.pat.kind}`

// Сессия merge ползунков узора: живёт от фокуса до ухода фокуса и не рвётся от того, что после каждого ответа Go
// приходит новый View (иначе каждая стрелка — отдельный шаг отмены). Другой узор или область — сессия не та.
export class PatSession {
  private id = ''
  private key = ''
  begin(key: string, t = Date.now()) {
    this.id = patSession(t)
    this.key = key
  }
  end() {
    this.id = ''
    this.key = ''
  }
  // Идентификатор для вызова Go; '' — сессии нет или она про другой узор.
  merge(key: string): string {
    return this.id !== '' && this.key === key ? this.id : ''
  }
}
