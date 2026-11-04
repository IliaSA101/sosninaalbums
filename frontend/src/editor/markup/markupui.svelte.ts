// Состояние разметки только для экрана (032, 066): прозрачность не сохраняется ни в проект, ни в localStorage —
// разметка нужна на время вёрстки. Флажок «Вид ▸ Разметка типографии» живёт в modes (настройка окна).
import { clampOpacity, nextRev, OPACITY_DEF, type MarkupKind } from './markup'

class MarkupUi {
  opacity = $state<Record<MarkupKind, number>>({ cover: OPACITY_DEF, spreads: OPACITY_DEF })
  // Номер в адресе картинки: адрес один на вид, поэтому смена файла (key) сбивает кэш этим числом.
  rev = $state<Record<MarkupKind, number>>({ cover: 0, spreads: 0 })
  // Какой отпечаток файла (GuideView.key) видели в последний раз: подложек на экране несколько, а bump нужен один раз.
  private seen: Record<MarkupKind, string | null> = { cover: null, spreads: null }

  setOpacity(kind: MarkupKind, v: number) {
    this.opacity[kind] = clampOpacity(v)
  }

  // Другой файл (выбрали, убрали, Undo, открыли альбом) — картинку надо перечитать. Сравниваем key, а не имя:
  // одноимённые файлы из разных папок имеют разный key; "" — файла нет.
  track(kind: MarkupKind, key: string, viewRev: number) {
    if (this.seen[kind] === key) return
    this.seen[kind] = key
    this.bump(kind, viewRev)
  }

  bump(kind: MarkupKind, viewRev: number) {
    this.rev[kind] = nextRev(this.rev[kind], viewRev)
  }
}

export const markupUi = new MarkupUi()
