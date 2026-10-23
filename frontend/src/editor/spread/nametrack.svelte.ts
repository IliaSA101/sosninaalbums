// Цель ввода имени под курсором на развороте (037): подпись или заглушка с полями имени — курсор text и подсказка.
// Заглушки мышь не ловят, поэтому цель ищем по точке (мм), а не по элементу.
import { drag } from '../../base/drag/drag.svelte'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { tip } from '../../base/ui/tip.svelte'
import { pageAt } from '../../base/geom/sheet'
import type { SpreadView } from '../../base/core/types'
import { placesel } from '../places/placesel.svelte'
import { clip } from '../clip/clip.svelte'
import { nameEdit } from '../captions/nameedit.svelte'
import { nameTargetAt, takesNameTarget, targetKey, type NameTarget } from '../captions/nametarget'
import { edit } from '../text/edit.svelte'
import { emptyField } from '../canvas/field'

// Что нужно от Spread.svelte: его данные меняются, поэтому — геттер.
export interface NameTrackCtx {
  spread: SpreadView
  mini: boolean
  root: HTMLElement | undefined
  bgEdit: boolean
}

// Создавать при инициализации компонента: внутри — $effect подсказки.
export class NameTrack {
  named = $state<NameTarget | null>(null)
  // Невидимая рамка цели, от неё встаёт подсказка (bind:this в SpreadOverlays).
  hint = $state<HTMLElement>()

  constructor(private readonly ctx: () => NameTrackCtx) {
    $effect(() => {
      const el = this.hint
      if (!el) return
      tip.showAt(el, 'Двойной щелчок — ввести имя')
      return () => tip.hideFor(el)
    })
  }

  // Цель по точке не знает, что выше (свободный текст, фото) и открыт ли редактор: решают признаки элемента.
  takes(target: EventTarget | null): boolean {
    const el = target as Element | null
    return takesNameTarget({
      can: prefs.can('captions'),
      bgEdit: this.ctx().bgEdit,
      editing: edit.editing,
      empty: emptyField(el),
      onNamed: !!el?.closest?.('.hit.caption.named'),
    })
  }

  // Страница под курсором: «+ Место» встаёт на неё; заодно — цель ввода имени.
  track = (e: PointerEvent) => {
    const { spread, mini } = this.ctx()
    if (mini || store.spread?.id !== spread.id) return
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    const x = ((e.clientX - r.left) / r.width) * spread.w
    placesel.page = pageAt(spread, x)
    // Курсор над листом, мм: Ctrl+V берёт страницу под ним (099). Нулевой размер — точки нет.
    clip.hover = r.width > 0 && r.height > 0 ? { x, y: ((e.clientY - r.top) / r.height) * spread.h } : null
    const t =
      drag.active || nameEdit.cur || !this.takes(e.target) ? null : nameTargetAt(spread, x, ((e.clientY - r.top) / r.height) * spread.h)
    const next = t?.who.length ? t : null
    if ((next && targetKey(next)) !== (this.named && targetKey(this.named))) this.named = next
  }

  leave = () => {
    clip.hover = null
    this.named = null
  }

  // Двойной щелчок по подписи по шаблону или заглушке: ввод имени вместо «вписать» (Stage.dbl его не увидит).
  dbl = (e: MouseEvent) => {
    const { spread, mini, root } = this.ctx()
    if (mini || store.spread?.id !== spread.id || !root || !this.takes(e.target)) return
    if (nameEdit.dbl(spread, e, root)) {
      e.stopPropagation()
      this.named = null
    }
  }
}
