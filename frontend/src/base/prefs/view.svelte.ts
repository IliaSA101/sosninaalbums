// Режимы вида (спека 003): «Во весь экран» и панель деталей. Это настройки
// окна, а не проекта: в .salbum и в undo не попадают, живут в localStorage.

import type { Family, Variety } from '../core/types'
import { normFamilies } from './families'
import { DEF_KEYS_SPLIT, loadKeysMode, loadKeysSplit, type KeysMode, type KeysSplit } from './keyswin'

// card — сворачиваемая карточка выделенного в панели деталей (028 Б), не раздел Section.
export type SectionName = 'card' | 'photo' | 'info' | 'settings' | 'grid'
type Sections = Record<SectionName, boolean>

const KEY = 'viewMode'

// Память «Фото на разворот» окна автосборки (051): «Авто» или свой диапазон [от, до].
export type AutoMem = 'auto' | [number, number]

// Размер миниатюр ленты (053): пределы общие у ползунка и Ctrl+колеса.
export const THUMB_MIN = 70
export const THUMB_MAX = 220
export const THUMB_DEF = 110

// Размер плиток окон оформления (133): по одному числу на вид окна. Пределы и умолчания знает
// panels/decorbrowse/tilesize.ts; здесь только память — целые px, мусор не загружается.
export const DECOR_TILE_KEYS = ['bg', 'sticker', 'frame', 'spread'] as const
export type DecorTileKey = (typeof DECOR_TILE_KEYS)[number]
export type DecorTiles = Partial<Record<DecorTileKey, number>>

// Порядок ленты фото (075): по имени (как от Go) или по времени съёмки.
export type LibSort = 'name' | 'shot'

const isPer = (x: unknown): x is number => Number.isInteger(x) && (x as number) >= 1 && (x as number) <= 64

// Хранилище → память плиток окон оформления: только известные виды и целые числа.
export function loadDecorTile(x: unknown): DecorTiles {
  const out: DecorTiles = {}
  if (!x || typeof x !== 'object') return out
  for (const k of DECOR_TILE_KEYS) {
    const n = (x as Record<string, unknown>)[k]
    if (Number.isInteger(n) && (n as number) > 0) out[k] = n as number
  }
  return out
}

// Хранилище → память: autoRange главнее; прежнее «не больше m» (autoMaxPer): 64 или нет → «Авто», меньшее m — «от min(3, m) до m».
// Мусор — «Авто».
export function loadRange(v: Record<string, unknown>): AutoMem {
  const r = v.autoRange
  if (Array.isArray(r) && r.length === 2 && isPer(r[0]) && isPer(r[1]) && r[0] <= r[1]) return [r[0], r[1]]
  if (isPer(v.autoMaxPer) && v.autoMaxPer < 64) return [Math.min(3, v.autoMaxPer), v.autoMaxPer]
  return 'auto'
}

// Уровень 0…3 (похожие, ч/б); всё остальное — 0.
export const norm03 = (x: unknown): number => (x === 1 || x === 2 || x === 3 ? x : 0)

class ViewMode {
  big = $state(false)
  details = $state(false)
  // Безопасная зона (спека 005): пунктир на Settings.safe мм от края.
  safe = $state(false)
  // Разметка типографии (066): подложка обложки и разворотов; по умолчанию включена, файл остаётся при выключении.
  markup = $state(true)
  // Силуэт под лицо (130): на виньетке, пока тянут кадр; по умолчанию включён.
  faceGuide = $state(true)
  // Линейки (спека 010): по умолчанию включены, видны на уровне «Полный».
  rulers = $state(true)
  sections = $state<Sections>({ card: true, photo: true, info: true, settings: false, grid: true })
  // Окно автосборки (027): сколько разворотов собирали в прошлый раз (null — ещё не собирали).
  autoSpreads = $state<number | null>(null)
  // 051: «Фото на разворот» — «Авто» или свой [от, до] (вместо прежнего «не больше»); не разрывать серии съёмки.
  autoRange = $state<AutoMem>('auto')
  autoSeries = $state(true)
  // 034: среднее «фото на разворот» и якорь — какое поле трогали последним (null/'spreads' — прежнее поведение, N).
  autoPer = $state<number | null>(null)
  autoBy = $state<'spreads' | 'per'>('spreads')
  // 041: «Разнообразие раскладок» окна автосборки; его же берёт «Разнообразить раскладки» (там «Меньше» = «Обычное»).
  autoVariety = $state<Variety>(1)
  // 065: разрешённые семейства автосборки; `[]` — «Все». Тот же выбор берёт «Разнообразить раскладки».
  autoFamilies = $state<Family[]>([])
  // 078: «Убирать похожие кадры» и «Ч/б развороты», 0…3; возвращённые кадры (keep) и вариант сборки не запоминаются.
  autoSimilar = $state(0)
  autoGray = $state(0)
  // Уровень «Разнообразить раскладок»: «Меньше» там считается «Обычным» (decisions 0.11 №25).
  get varyLevel(): Variety {
    return this.autoVariety === 2 ? 2 : 1
  }
  // 053: размер миниатюр ленты, целые px; настройка окна, как высота ленты.
  thumbSize = $state(THUMB_DEF)
  // 133: размер плиток окон «Фоны», «Стикеры», «Рамочки», «Развороты»; нет ключа — умолчание вида.
  decorTile = $state<DecorTiles>({})
  // 075: порядок ленты («Фильтр ▸ Порядок»); настройка окна, фильтр «Сбросить» её не трогает.
  libSort = $state<LibSort>('name')
  // 058: последняя типография в списке форматов «Настроек альбома» ("" — «Общие»).
  presetGroup = $state('')
  // 074: окно «Горячие клавиши» — вид (карта или список крупно) и доли блоков.
  keysMode = $state<KeysMode>('map')
  keysSplit = $state<KeysSplit>({ ...DEF_KEYS_SPLIT })
  // Запрос «прокрутить панель к разделу»: счётчик, чтобы повторный запрос тоже срабатывал.
  reveal = $state<{ name: SectionName; n: number } | null>(null)

  constructor() {
    this.load()
  }

  load() {
    try {
      const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
      if (!v || typeof v !== 'object') return
      this.big = v.big === true
      this.details = v.details === true
      this.safe = v.safe === true
      this.markup = v.markup !== false
      this.faceGuide = v.faceGuide !== false
      this.rulers = v.rulers !== false
      const t = v.thumbSize
      if (Number.isInteger(t) && t >= THUMB_MIN && t <= THUMB_MAX) this.thumbSize = t
      this.decorTile = loadDecorTile(v.decorTile)
      if (v.libSort === 'name' || v.libSort === 'shot') this.libSort = v.libSort
      if (typeof v.presetGroup === 'string') this.presetGroup = v.presetGroup
      this.keysMode = loadKeysMode(v.keysMode)
      this.keysSplit = loadKeysSplit(v.keysSplit)
      this.loadAuto(v)
      const s = v.sections
      if (s && typeof s === 'object') {
        for (const k of Object.keys(this.sections) as SectionName[]) {
          if (typeof s[k] === 'boolean') this.sections[k] = s[k]
        }
      }
    } catch {
      // Без хранилища режимы просто выключены.
    }
  }

  // Память окна автосборки; мусор в хранилище — прежние значения.
  private loadAuto(v: Record<string, unknown>) {
    const { autoSpreads: sp, autoPer: per, autoBy: by, autoVariety: va, autoSeries: se, autoFamilies: fa } = v
    if (va === 0 || va === 1 || va === 2) this.autoVariety = va
    this.autoFamilies = normFamilies(fa)
    if (Number.isInteger(sp) && (sp as number) > 0) this.autoSpreads = sp as number
    this.autoRange = loadRange(v)
    if (typeof se === 'boolean') this.autoSeries = se
    this.autoSimilar = norm03(v.autoSimilar)
    this.autoGray = norm03(v.autoGray)
    if (typeof per === 'number' && per >= 1 && per <= 64) this.autoPer = per
    if (by === 'spreads' || by === 'per') this.autoBy = by
  }

  save() {
    try {
      localStorage.setItem(
        KEY,
        JSON.stringify({
          big: this.big,
          details: this.details,
          safe: this.safe,
          markup: this.markup,
          faceGuide: this.faceGuide,
          rulers: this.rulers,
          thumbSize: this.thumbSize,
          decorTile: this.decorTile,
          libSort: this.libSort,
          presetGroup: this.presetGroup,
          keysMode: this.keysMode,
          keysSplit: this.keysSplit,
          sections: this.sections,
          autoSpreads: this.autoSpreads,
          autoRange: this.autoRange,
          autoSeries: this.autoSeries,
          autoPer: this.autoPer,
          autoBy: this.autoBy,
          autoVariety: this.autoVariety,
          autoFamilies: this.autoFamilies,
          autoSimilar: this.autoSimilar,
          autoGray: this.autoGray,
        }),
      )
    } catch {
      // Без хранилища режимы не запомнятся.
    }
  }

  toggleBig() {
    this.big = !this.big
    this.save()
  }

  toggleSafe() {
    this.safe = !this.safe
    this.save()
  }

  toggleMarkup() {
    this.markup = !this.markup
    this.save()
  }

  toggleFaceGuide() {
    this.faceGuide = !this.faceGuide
    this.save()
  }

  toggleRulers() {
    this.rulers = !this.rulers
    this.save()
  }

  // Запомнить выбор окна автосборки после «Собрать».
  rememberAuto(
    spreads: number | null,
    range: AutoMem,
    per: number | null,
    by: 'spreads' | 'per',
    variety: Variety = this.autoVariety,
    series: boolean = this.autoSeries,
    families: readonly Family[] = this.autoFamilies,
  ) {
    this.autoSpreads = spreads
    this.autoRange = range
    this.autoSeries = series
    this.autoVariety = variety
    this.autoFamilies = normFamilies(families)
    this.autoPer = per !== null && Number.isFinite(per) ? Math.max(1, Math.min(64, per)) : null
    this.autoBy = by
    this.save()
  }

  // 078: запомнить уровни «похожих» и «ч/б» после «Собрать» (мусор — 0).
  rememberPhotos(similar: number, gray: number) {
    this.autoSimilar = norm03(similar)
    this.autoGray = norm03(gray)
    this.save()
  }

  // Размер миниатюр: округлить, зажать в пределы, запомнить (мусор вместо числа — не трогаем).
  setThumb(n: number) {
    if (!Number.isFinite(n)) return
    this.thumbSize = Math.max(THUMB_MIN, Math.min(THUMB_MAX, Math.round(n)))
    this.save()
  }

  // Размер плиток окна оформления: целое число запоминаем (пределы зажимает вызывающий), мусор — нет.
  setDecorTile(kind: DecorTileKey, n: number) {
    if (!Number.isFinite(n)) return
    const v = Math.round(n)
    if (this.decorTile[kind] === v) return
    this.decorTile = { ...this.decorTile, [kind]: v }
    this.save()
  }

  // Порядок ленты: запомнить (повторный выбор того же — без записи).
  setLibSort(s: LibSort) {
    if (this.libSort === s) return
    this.libSort = s
    this.save()
  }

  // Запомнить типографию списка форматов.
  setPresetGroup(g: string) {
    this.presetGroup = g
    this.save()
  }

  // Окно клавиш (074): вид и доли блоков запоминаются; повтор того же — без записи.
  setKeysMode(m: KeysMode) {
    if (this.keysMode === m) return
    this.keysMode = m
    this.save()
  }

  setKeysSplit(s: KeysSplit) {
    this.keysSplit = { ...s }
    this.save()
  }

  toggleDetails() {
    this.details = !this.details
    this.save()
  }

  toggleSection(name: SectionName) {
    this.sections[name] = !this.sections[name]
    this.save()
  }

  // Открыть панель с раскрытым разделом и прокрутить к нему.
  openSection(name: SectionName) {
    this.details = true
    this.sections[name] = true
    this.reveal = { name, n: (this.reveal?.n ?? 0) + 1 }
    this.save()
  }
}

export const modes = new ViewMode()
