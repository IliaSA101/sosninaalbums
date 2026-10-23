<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { edit } from '../text/edit.svelte'
  import { photoui } from './photoui.svelte'
  import { straighten } from '../tilt/straighten.svelte'
  import { tiltmode } from '../tilt/tiltmode.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { clip } from '../clip/clip.svelte'
  import { menuContext, photoMenu, sheetMenu } from './photomenu'
  import type { ActId } from './photoacts'
  import type { LayerOp } from '../places/minipanel'
  import { relinkFolder } from '../../panels/library/relink'
  import { decor } from '../../panels/decor/decor.svelte'
  import { sticker } from '../sticker/sticker.svelte'
  import { copyNames, reveal } from './fileact'
  import { scopeLabel, scopesFor } from '../backdrop/backdrop'
  import { setBackdrop } from '../backdrop/backdropact'
  import { coverLabel, coverScopes } from '../cover/cover'
  import { isCover, isSheet } from '../../base/geom/sheet'
  import MenuList from '../../base/ui/MenuList.svelte'
  import type { MenuNode } from '../../base/ui/menulist'
  import type { BackdropScope } from '../../base/core/types'
  import type { Pt } from '../clip/clip'
  import { goUse } from '../photoprops/gouse.svelte'
  import { UseList } from '../photoprops/uselist.svelte'
  import { elsewhere, lineAt, USE_ROW, useRows } from '../photoprops/usemenu'
  import { untrack } from 'svelte'

  const sheet = $derived(isSheet(store.view.settings))
  // На обложке (032) фон ставят в области её раскладки, названия — сторонки; в листе (031) — только «Весь лист».
  const cover = $derived(isCover(store.spread))
  const scopes = $derived(cover ? coverScopes(store.cover) : scopesFor(sheet))
  const label = (sc: BackdropScope) => (cover ? coverLabel(store.cover, sc) : scopeLabel(sc, sheet))

  // Меню правого щелчка по фото (спека 008, 108): своё, работает и в браузере. Дерево пунктов — photo/photomenu.ts,
  // список с подменю сбоку и клавишами — base/ui/MenuList. Esc и щелчок мимо закрывают.
  // Выделены одни стикеры (048) — меню стикера: «Копировать», «Вставить», «Заменить…», «Зеркало», «Слой», «Удалить».
  // Одни надписи (099) — своё меню по его образцу; пустой лист (099) — только «Вставить».
  // Одиночная надпись выделена через edit.sel, а не placesel.texts: считается, лишь когда ни мест, ни стикеров.
  const texts = $derived(
    placesel.texts.length ||
      (placesel.selected.length === 0 && placesel.stickers.length === 0 && edit.sel?.kind === 'text' && edit.sel.spread === store.spread?.id
        ? 1
        : 0),
  )
  // «Где ещё стоит ▸» (спека 106, 108): одно фото на месте; вхождения от Go грузятся при открытии меню, текущее место
  // в списке не показываем («ещё» — кроме этого).
  const useList = new UseList()
  const useBox = $derived(
    photoui.menu?.kind === 'photo' && placesel.selected.length === 1 && texts + placesel.selectedStickers.length === 0
      ? placesel.selected[0]
      : undefined,
  )
  const useId = $derived(useBox?.photo ?? '')
  const useLines = $derived(elsewhere(useList.lines, useBox && store.spread ? { spread: store.spread.id, pick: useBox.photo } : undefined))
  $effect(() => {
    const id = useId
    untrack(() => (id ? useList.load(id, () => photoui.close()) : useList.reset()))
    return () => useList.reset()
  })
  const items = $derived(
    photoui.menu?.kind === 'sheet'
      ? sheetMenu(clip.canPaste, prefs.can, keymap.hint.bind(keymap))
      : photoMenu(
          {
            ...menuContext(store.spread, placesel.selected, store.missing, texts, placesel.selectedStickers.length, {
              copy: clip.canCopy,
              paste: clip.canPaste,
            }),
            scopes: scopes.map((scope) => ({ scope, label: label(scope) })),
            usesSub: useRows(useLines, 'Больше нигде не стоит'),
          },
          prefs.can,
          keymap.hint.bind(keymap),
        ),
  )

  // Выделение пропало (правка, смена разворота) — пунктам не на что опираться.
  $effect(() => {
    if (photoui.menu && items.length === 0) photoui.close()
  })

  const photos = () => placesel.selected.filter((b) => b.photo).map((b) => b.photo)

  // Копирование имён и проводник — общие с меню миниатюры (fileact.ts); у холста пропавшие — store.missing.
  const gone = (id: string) => store.missing.includes(id)

  // Выбор пункта: меню закрывается с сохранением выделения, действие получает первое выделенное фото,
  // точку щелчка (мм) для «Вставить» и параметр пункта подменю.
  function pick(n: MenuNode) {
    const first = photos()[0]
    const at = photoui.menu?.at
    const line = n.id === USE_ROW ? lineAt(useLines, n.arg) : null
    photoui.close(true)
    if (line) goUse(line)
    else ACTIONS[n.id as ActId]?.(first, at, n.arg)
  }

  // «Заменить…» у стикера: то же окно «Стикеры», что и в карточке.
  function replaceSticker() {
    const t = sticker.target()
    if (t) decor.show('sticker', t.spread, 'spread', t.id)
  }

  // «Рамочка…» (079): окно «Рамочки» с выделенными местами в «Куда» (меню уже закрыто с сохранением выделения).
  function openBorder() {
    const s = store.spread
    if (s) decor.show('frame', s.id, 'spread')
  }

  // «На фон ▸»: параметр — область (лист или обложка).
  function toBackdrop(first: string, scope: string | undefined) {
    const s = store.spread
    if (s && first && scope) setBackdrop(s.id, scope as BackdropScope, first)
  }

  // Действие пункта; first — первое выделенное фото (у части пунктов оно не нужно), at — точка щелчка, arg — параметр
  // пункта подменю («Слой»: операция, «На фон»: область).
  const ACTIONS: Partial<Record<ActId, (first: string, at?: Pt, arg?: string) => void>> = {
    preview: (first) => (photoui.preview = first),
    copy: () => void copyNames(photos(), gone),
    clipCopy: () => clip.copy(),
    clipPaste: (_first, at) => clip.paste(at),
    explorer: (first) => void reveal(first),
    relink: () => void relinkFolder(),
    fitPhoto: () => placesel.arrange('fitPhoto'),
    turnCw: () => placesel.turn(1),
    turnCcw: () => placesel.turn(-1),
    straighten: (first) => straighten.startPhoto(first),
    tilt: () => tiltmode.toggle(),
    flipH: () => placesel.flip('h'),
    flipV: () => placesel.flip('v'),
    gray: () => placesel.gray(),
    contain: () => placesel.setFit(true),
    cover: () => placesel.setFit(false),
    rect: () => placesel.setShape('rect'),
    rounded: () => placesel.setShape('round'),
    oval: () => placesel.setShape('oval'),
    border: openBorder,
    resetFrame: (first) => void store.run(Api.ResetFrame(first)),
    caption: (first) => edit.editCaption(first),
    layer: (_first, _at, arg) => placesel.layer(arg as LayerOp),
    backdrop: (first, _at, arg) => toBackdrop(first, arg),
    replace: replaceSticker,
    // Одиночная надпись (099) живёт в edit.sel, не в placesel: удаляется своим путём (как Del).
    delete: () => (texts === 1 && placesel.texts.length === 0 ? edit.removeSelected() : placesel.deleteSelected()),
  }
</script>

{#if photoui.menu && items.length > 0}
  <MenuList {items} at={{ x: photoui.menu.x, y: photoui.menu.y }} onpick={pick} onclose={() => photoui.close()} />
{/if}
