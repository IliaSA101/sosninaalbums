// Окно «Горячие клавиши» (074): вид и доли блоков — настройка окна, как прочие настройки вида (view.svelte.ts).
// Хранилище → память с проверкой: мусор даёт значения по умолчанию.

// Карта крупно (как вариант А макета) или список крупно (как Б).
export type KeysMode = 'map' | 'list'

// Доли блоков: ra — высота карты в «Карте крупно» (null — по размеру клавиш), rw — ширина списка и rh — высота карты
// в «Списке крупно». Пределы в пикселях держит keyslayout.ts.
export interface KeysSplit {
  ra: number | null
  rw: number
  rh: number
}

export const DEF_KEYS_SPLIT: Readonly<KeysSplit> = { ra: null, rw: 0.27, rh: 0.62 }

const isFrac = (x: unknown): x is number => typeof x === 'number' && x > 0.05 && x < 0.95

export const loadKeysMode = (x: unknown): KeysMode => (x === 'list' ? 'list' : 'map')

export function loadKeysSplit(x: unknown): KeysSplit {
  const o = x && typeof x === 'object' ? (x as Record<string, unknown>) : {}
  return {
    ra: isFrac(o.ra) ? o.ra : DEF_KEYS_SPLIT.ra,
    rw: isFrac(o.rw) ? o.rw : DEF_KEYS_SPLIT.rw,
    rh: isFrac(o.rh) ? o.rh : DEF_KEYS_SPLIT.rh,
  }
}
