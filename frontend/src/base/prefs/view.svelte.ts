// Режимы вида (спека 003): крупный просмотр и панель деталей. Это настройки
// окна, а не проекта: в .salbum и в undo не попадают, живут в localStorage.

import type { Variety } from '../core/types'

// card — сворачиваемая карточка выделенного в панели деталей (028 Б), не раздел Section.
export type SectionName = 'card' | 'info' | 'settings' | 'grid'
type Sections = Record<SectionName, boolean>

const KEY = 'viewMode'

class ViewMode {
  big = $state(false)
  details = $state(false)
  // Безопасная зона (спека 005): пунктир на Settings.safe мм от края.
  safe = $state(false)
  // Линейки (спека 010): по умолчанию включены, видны на уровне «Полный».
  rulers = $state(true)
  sections = $state<Sections>({ card: true, info: true, settings: false, grid: true })
  // Окно автосборки (027): сколько разворотов собирали в прошлый раз (null — ещё не собирали) и «не больше на разворот».
  autoSpreads = $state<number | null>(null)
  autoMaxPer = $state(64)
  // 034: среднее «фото на разворот» и якорь — какое поле трогали последним (null/'spreads' — прежнее поведение, N).
  autoPer = $state<number | null>(null)
  autoBy = $state<'spreads' | 'per'>('spreads')
  // 041: «Разнообразие раскладок» окна автосборки; его же берёт «Разнообразить раскладки» (там «Меньше» = «Обычное»).
  autoVariety = $state<Variety>(1)
  // Уровень «Разнообразить раскладок»: «Меньше» там считается «Обычным» (decisions 0.11 №25).
  get varyLevel(): Variety {
    return this.autoVariety === 2 ? 2 : 1
  }
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
      this.rulers = v.rulers !== false
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
    const { autoSpreads: sp, autoMaxPer: mp, autoPer: per, autoBy: by, autoVariety: va } = v
    if (va === 0 || va === 1 || va === 2) this.autoVariety = va
    if (Number.isInteger(sp) && (sp as number) > 0) this.autoSpreads = sp as number
    if (Number.isInteger(mp) && (mp as number) >= 1 && (mp as number) <= 64) this.autoMaxPer = mp as number
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
          rulers: this.rulers,
          sections: this.sections,
          autoSpreads: this.autoSpreads,
          autoMaxPer: this.autoMaxPer,
          autoPer: this.autoPer,
          autoBy: this.autoBy,
          autoVariety: this.autoVariety,
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

  toggleRulers() {
    this.rulers = !this.rulers
    this.save()
  }

  // Запомнить выбор окна автосборки после «Собрать».
  rememberAuto(spreads: number, maxPer: number, per: number, by: 'spreads' | 'per', variety: Variety = this.autoVariety) {
    this.autoSpreads = spreads
    this.autoMaxPer = maxPer
    this.autoVariety = variety
    this.autoPer = Number.isFinite(per) ? Math.max(1, Math.min(64, per)) : null
    this.autoBy = by
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
