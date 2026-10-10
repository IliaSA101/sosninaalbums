// Режимы вида (спека 003): крупный просмотр и панель деталей. Это настройки
// окна, а не проекта: в .salbum и в undo не попадают, живут в localStorage.

export type SectionName = 'info' | 'settings' | 'grid'
type Sections = Record<SectionName, boolean>

const KEY = 'viewMode'

class ViewMode {
  big = $state(false)
  details = $state(false)
  // Безопасная зона (спека 005): пунктир на 8 мм от края.
  safe = $state(false)
  // Линейки (спека 010): по умолчанию включены, видны на уровне «Полный».
  rulers = $state(true)
  sections = $state<Sections>({ info: true, settings: false, grid: true })
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

  save() {
    try {
      localStorage.setItem(
        KEY,
        JSON.stringify({ big: this.big, details: this.details, safe: this.safe, rulers: this.rulers, sections: this.sections }),
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
