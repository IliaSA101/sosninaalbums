// Контекст меню из живого состояния программы (спека 010). Читают его строка меню (MenuBar, через $derived)
// и диспетчер клавиш: команда без клавиши по умолчанию срабатывает, пока её пункт виден и не серый (074-8).
import { store } from '../../base/core/store.svelte'
import { jobs } from '../../base/core/jobs.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { clip } from '../../editor/clip/clip.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { binsel } from '../../panels/bins/binsel.svelte'
import { align } from '../../panels/layouts/align.svelte'
import { anyVaries, varyTargets } from '../../panels/gallery/variants'
import { canMakeVignette, isVignette } from '../../panels/vignette/vignette'
import { isCover } from '../../base/geom/sheet'
import { splitMissing } from '../../panels/library/missing'
import { buildMenu, enabledCmds, type MenuCtx } from './menu'

export function menuCtx(): MenuCtx {
  return {
    can: prefs.can,
    level: prefs.level,
    recent: store.recent,
    spreads: store.sheets.length, // лист обложки (032) тоже можно выгрузить
    hasSpread: !!store.spread,
    canUndo: store.view.canUndo,
    canRedo: store.view.canRedo,
    zone: store.zone,
    libraryPhotos: store.visible.length,
    anySelection:
      store.selected.size > 0 ||
      spreadsel.size > 0 ||
      placesel.refs.length > 0 ||
      placesel.texts.length > 0 ||
      placesel.stickers.length > 0 ||
      !!edit.sel,
    deletable: placesel.refs.length > 0 || placesel.texts.length > 0 || placesel.stickers.length > 0 || !!edit.sel,
    flippable: placesel.selected.some((b) => b.photo),
    allGray: placesel.allGray,
    sticker: !!placesel.soloSticker,
    varyable: anyVaries(store.view.spreads, varyTargets(spreadsel.ordered(store.view.spreads.map((s) => s.id)), store.spread?.id)),
    vigable: canMakeVignette(store.spread, prefs.can),
    decorable: !!store.spread && !isCover(store.spread),
    vigSetup: isVignette(store.spread),
    layerable: placesel.canLayer,
    copyable: clip.canCopy,
    pasteable: clip.canPaste,
    missing: splitMissing(store.missing, store.view.libPhotos).photos.length, // свои из библиотеки папкой не ищутся (048)
    collecting: !!jobs.collecting,
    exporting: !!jobs.exporting,
    big: modes.big,
    details: modes.details,
    safe: modes.safe,
    markup: modes.markup,
    faceGuide: modes.faceGuide,
    rulers: modes.rulers,
    snap: placesel.snap,
    assocCan: store.assoc.can,
    assocOn: store.assoc.on,
    binShown: binsel.active,
    alignCount: align.ids.length,
    alignTo: align.to,
  }
}

// Пункт меню команды виден на уровне и не серый. Меню строится на нажатие: пара десятков пунктов, и только
// когда клавиша совпала с командой без своих условий.
export const menuOn = (id: string): boolean => enabledCmds(buildMenu(menuCtx())).has(id)
