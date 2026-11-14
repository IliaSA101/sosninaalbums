import { Api, store } from '../../base/core/store.svelte'
import { SHEET_PRESETS } from '../../base/geom/sheet'
import { edit } from '../../editor/text/edit.svelte'
import { backupsUi } from '../dialogs/backups.svelte'
import { DRAFT_NOTE, hadDraft } from './draft'
import { pickedDims, type ProductKind, type SizePick } from './newproduct'

// Выбор нового изделия в стартовом окне; «Файл ▸ Новый проект: виньетка-лист…» открывает окно с уже выбранным листом.
class NewDoc {
  kind = $state<ProductKind>('album')
  pick = $state<SizePick>(0)
  w = $state<number | null>(SHEET_PRESETS[0].w)
  h = $state<number | null>(SHEET_PRESETS[0].h)

  dims = $derived(this.kind === 'sheet' ? pickedDims(this.pick, this.w, this.h) : null)
  // Можно создавать: альбом всегда, лист — при годном размере.
  ok = $derived(this.kind === 'album' || this.dims !== null)

  choose(pick: SizePick) {
    this.pick = pick
    if (pick !== 'custom') {
      this.w = SHEET_PRESETS[pick].w
      this.h = SHEET_PRESETS[pick].h
    }
  }

  // Идёт создание: второй щелчок по «создать», пока Go отвечает, не должен создать проект ещё раз.
  busy = $state(false)

  // Создаёт выбранное; true — проект заменён (окно можно закрывать).
  async create(): Promise<boolean> {
    if (this.busy) return false
    this.busy = true
    try {
      if (this.kind === 'album') return await this.replace(() => Api.NewProject('album'))
      const d = this.dims
      return d ? await this.replace(() => Api.NewVignetteSheet(d.w, d.h)) : false
    } finally {
      this.busy = false
    }
  }

  // Замена проекта новым (меню, стартовое окно, лист): Go прежний черновик убирает в резервные копии молча —
  // говорим об этом тостом с «Открыть копии» (U4). Черновик определяем до замены.
  // Отказ Go (черновик занят другой программой — вернул прежнее состояние) тостом не затираем: после замены черновика уже нет.
  // Сначала ждём набранный текст (087-5), вызов Go — внутри замены: иначе он уйдёт раньше.
  async replace(next: () => Promise<unknown>): Promise<boolean> {
    await edit.idle()
    const draft = hadDraft(store.view)
    const ok = await store.replaceState(next())
    if (ok && draft && !hadDraft(store.view)) store.notify(DRAFT_NOTE, 'info', { label: 'Открыть копии', run: () => void backupsUi.show() })
    return ok
  }

  // Меню «Файл ▸ Новый проект: виньетка-лист…»: окно открывается на листе 20×30.
  showSheet() {
    this.kind = 'sheet'
  }
}

export const newDoc = new NewDoc()
