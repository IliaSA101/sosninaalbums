// Свёрнутые группы карточек «Фото», «Разворот», «Обложка» (спека 124, 0.26-124-2): настройка окна, не проекта —
// в .salbum и в undo не попадает. Отдельно для панели деталей ('d') и плавающей карточки ('f'): они не видны
// вместе, и разумное свёрнутое в 200 px и в 400 px разное. Свой ключ localStorage, как у align и placesel.
import { getContext, setContext } from 'svelte'

export type GroupHost = 'd' | 'f'

// Ключи групп: чужие id из хранилища отбрасываются, поэтому список закрытый.
export const GROUP_IDS = [
  'photo.frame',
  'photo.tone',
  'photo.shape',
  'photo.name',
  'photo.place',
  'photo.about',
  'spread.vig',
  'spread.bg',
  'spread.caps',
  'cover.layout',
  'cover.spine',
  'cover.bg',
  'cover.caps',
  'cover.size',
] as const
export type GroupId = (typeof GROUP_IDS)[number]

const KEY = 'cardGroups'

export const isGroupId = (x: unknown): x is GroupId => GROUP_IDS.includes(x as GroupId)
export const isGroupHost = (x: unknown): x is GroupHost => x === 'd' || x === 'f'

type Folded = Record<GroupHost, GroupId[]>

// Хранилище → свёрнутые: мусор, не-массивы и чужие id отбрасываются, повторы схлопываются; порядок — как в GROUP_IDS.
export function normGroups(x: unknown): Folded {
  const src = x && typeof x === 'object' ? (x as Record<string, unknown>) : {}
  const pick = (v: unknown): GroupId[] => (Array.isArray(v) ? GROUP_IDS.filter((id) => v.includes(id)) : [])
  return { d: pick(src.d), f: pick(src.f) }
}

export class CardGroups {
  private folded = $state<Folded>({ d: [], f: [] })

  // Один раз при старте. Нет значения, не JSON или нет хранилища — всё открыто.
  constructor() {
    try {
      this.folded = normGroups(JSON.parse(localStorage.getItem(KEY) ?? 'null'))
    } catch {
      // Всё открыто.
    }
  }

  private save() {
    try {
      localStorage.setItem(KEY, JSON.stringify(this.folded))
    } catch {
      // Без хранилища свёрнутое не запомнится.
    }
  }

  isOpen(h: GroupHost, id: GroupId): boolean {
    return !this.folded[h].includes(id)
  }

  // Щелчок по заголовку; возвращает, открыта ли группа теперь.
  toggle(h: GroupHost, id: GroupId): boolean {
    const to = !this.isOpen(h, id)
    this.set(h, id, to)
    return to
  }

  // Раскрыть с записью: F1 «Показать», тур, подсветка «Наклон», удачный бросок фото на фон. false — уже была открыта.
  open(h: GroupHost, id: GroupId): boolean {
    if (this.isOpen(h, id)) return false
    this.set(h, id, true)
    return true
  }

  // Всё открыто (по умолчанию).
  reset() {
    this.folded = { d: [], f: [] }
    this.save()
  }

  private set(h: GroupHost, id: GroupId, open: boolean) {
    const rest = this.folded[h].filter((g) => g !== id)
    const next = open ? rest : [...rest, id]
    this.folded = { ...this.folded, [h]: GROUP_IDS.filter((g) => next.includes(g)) }
    this.save()
  }
}

export const cardGroups = new CardGroups()

// Контекст карточки (0.26-124-4): хост групп и «узкая» (плавающая 200 px). Ставит Inspector по docked; вне карточки
// (тесты, чужие места) — панель деталей и не узкая. Геттеры, чтобы читать актуальное значение, а не снимок.
interface CardCtx {
  readonly host: GroupHost
  readonly narrow: boolean
}
const CTX = Symbol('card')

// Пара «ключ, значение» контекста: provideCard кладёт её через setContext, тесты — через опцию context при render.
export function cardContext(docked: () => boolean): [symbol, CardCtx] {
  return [
    CTX,
    {
      get host() {
        return docked() ? 'd' : 'f'
      },
      get narrow() {
        return !docked()
      },
    },
  ]
}

export function provideCard(docked: () => boolean): void {
  const [key, ctx] = cardContext(docked)
  setContext<CardCtx>(key, ctx)
}

// Вызывать при создании компонента: getContext вне инициализации бросает.
export function useCard(): CardCtx {
  return getContext<CardCtx | undefined>(CTX) ?? { host: 'd', narrow: false }
}

// Удачный бросок фото на фон (0.26-124-23): счётчик, по которому группа с зоной «Фото фона» понимает, что
// её надо оставить открытой и запомнить. Шлёт dropact после ответа Go.
class DropLanded {
  n = $state(0)
  scope = $state('')
  spread = $state('')
  notify(spread: string, scope: string) {
    this.spread = spread
    this.scope = scope
    this.n++
  }
}
export const dropLanded = new DropLanded()
