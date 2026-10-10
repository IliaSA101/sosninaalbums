// Действующий режим боковых панелей (inspector/panels.ts → panelMode) из store, edit и placesel:
// один на Stage, EditRail, Inspector и DetailsPanel.
import { store } from '../../base/core/store.svelte'
import { gallery } from '../../panels/layouts/gallery.svelte'
import { edit } from '../text/edit.svelte'
import { placesel } from '../places/placesel.svelte'
import { panelMode, type PanelMode } from './panels'

class Panels {
  mode = $derived<PanelMode>(
    panelMode({
      spread: store.spread?.id ?? null,
      gallery: !!gallery.cur,
      textSpread: edit.sel?.spread ?? null,
      refs: placesel.selected.length,
      group: placesel.isGroup,
      photo: placesel.selected.length === 1 && !!store.spread?.photos.includes(placesel.selected[0].ref),
      photoMissing: placesel.selected.length === 1 && store.missing.includes(placesel.selected[0].ref),
    }),
  )
}

export const panels = new Panels()
