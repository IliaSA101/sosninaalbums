<script lang="ts">
  import { onMount } from 'svelte'
  import TopBar from './shell/menu/TopBar.svelte'
  import SpreadBar from './editor/spread/SpreadBar.svelte'
  import LayoutGallery from './panels/gallery/LayoutGallery.svelte'
  import MissingBar from './panels/library/MissingBar.svelte'
  import Stage from './editor/canvas/Stage.svelte'
  import Timeline from './panels/timeline/Timeline.svelte'
  import Library from './panels/library/Library.svelte'
  import DetailsPanel from './panels/details/DetailsPanel.svelte'
  import Overlay from './base/drag/Overlay.svelte'
  import CopyGridDialog from './panels/layouts/CopyGridDialog.svelte'
  import ExportDialog from './shell/export/ExportDialog.svelte'
  import FsferaDone from './shell/export/FsferaDone.svelte'
  import AutoAskDialog from './shell/ask/AutoAskDialog.svelte'
  import DropAskDialog from './shell/ask/DropAskDialog.svelte'
  import AutoBuildDialog from './shell/dialogs/AutoBuildDialog.svelte'
  import HotkeysHelp from './shell/menu/HotkeysHelp.svelte'
  import Guide from './shell/guide/Guide.svelte'
  import Spot from './shell/guide/Spot.svelte'
  import { guide } from './shell/guide/guidestate.svelte'
  import StartDialog from './shell/dialogs/StartDialog.svelte'
  import LevelsDialog from './shell/dialogs/LevelsDialog.svelte'
  import BackupsDialog from './shell/dialogs/BackupsDialog.svelte'
  import { backupsUi } from './shell/dialogs/backups.svelte'
  import NamesDialog from './panels/names/NamesDialog.svelte'
  import VigDialog from './panels/vignette/VigDialog.svelte'
  import VigTplSave from './panels/vignette/VigTplSave.svelte'
  import DecorDialog from './panels/decor/DecorDialog.svelte'
  import { vigDialog } from './panels/vignette/vigsession.svelte'
  import { namesUi } from './panels/names/names.svelte'
  import PhotoMenu from './editor/photo/PhotoMenu.svelte'
  import PhotoPreview from './editor/photo/PhotoPreview.svelte'
  import { store } from './base/core/store.svelte'
  import { gridAct } from './panels/layouts/gridact.svelte'
  import { dropAsk } from './base/drag/dropgrid.svelte'
  import { gallery } from './panels/gallery/gallery.svelte'
  import { placesel } from './editor/places/placesel.svelte'
  import { canvas } from './editor/canvas/canvas.svelte'
  import { run } from './shell/menu/commands'
  import { help } from './shell/menu/help.svelte'
  import { isTextField, keyAction, preventsDefault } from './shell/menu/keys'
  import { zoneOf } from './editor/places/groupsel'
  import { bgmode } from './editor/backdrop/bgmode.svelte'
  import { prefs } from './base/prefs/prefs.svelte'
  import { modes } from './base/prefs/view.svelte'

  onMount(() => {
    store.init()
  })

  // Зона последнего нажатия: Ctrl+A и Del действуют по ней (библиотека или холст с лентой альбома).
  function zonePress(e: PointerEvent) {
    const z = zoneOf(e.target instanceof Element ? e.target : null)
    if (z) store.zone = z
  }

  // Пробел листает раскладки «Авто»: у сетки их нет, у обложки (032) тоже.
  const noAutoLayouts = () => !!store.spread?.grid || store.onCover

  // Открыт вопрос ручного листа (автораскладка, броски, копия сетки): под ним F1 и Ctrl+N/O/E молчат.
  const askOpen = () => !!gridAct.autoAsk || !!dropAsk.cur || !!gridAct.copyGrid

  // F1 — справочник функций (033). В WebView2 он открыл бы справку браузера, поэтому гасится всегда.
  // Поверх стартового окна и окна уровней справочник открывается; под другими окнами и сам справочник — нет.
  function f1(e: KeyboardEvent): boolean {
    if (e.key !== 'F1' || e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return false
    e.preventDefault()
    const other =
      help.open ||
      backupsUi.open ||
      namesUi.open ||
      !!vigDialog.id ||
      !!store.autoDraft ||
      store.exportOpen ||
      !!store.exportMemo ||
      askOpen()
    if (!guide.open && !other) guide.openFresh()
    return true
  }

  // Окна, под которыми клавиши приложения молчат (Esc закрывает само окно).
  const underDialog = () =>
    guide.open || prefs.startOpen || prefs.compare || backupsUi.open || namesUi.open || !!vigDialog.id || !!store.autoDraft
  // Цель события бывает не Element (document/window, синтетическое событие) — тогда null.
  const targetEl = (e: Event) => (e.target instanceof Element ? e.target : null)
  const isCheckbox = (t: Element | null) => !!t?.matches("input[type='checkbox']")

  function keys(e: KeyboardEvent) {
    const t = targetEl(e)
    if (f1(e)) return
    // Пока открыта справка, клавиши не работают; Esc её закрывает (если фокус ушёл из окна).
    if (help.open) {
      if (e.key === 'Escape') help.open = false
      return
    }
    // Стартовое окно и окно уровней: клавиши под ними молчат (Esc закрывает само окно).
    if (underDialog()) return
    // Хоткеи глушим только при вводе текста/чисел (в том числе в contenteditable); select и ползунок их не блокируют.
    const act = keyAction(e, {
      inTextField: isTextField(t),
      onCheckbox: isCheckbox(t),
      galleryOpen: !!gallery.cur,
      hasSelection: store.selectedIds.length > 0,
      gridSpread: noAutoLayouts(),
      placeSelected: placesel.selected.length > 0 || placesel.texts.length > 0,
      stickerSelected: placesel.selected.length === 0 && placesel.texts.length === 0 && placesel.stickers.length > 0,
      layerable: placesel.canLayer,
      zone: store.zone,
      bgMode: !!bgmode.cur,
      big: modes.big,
      modal: store.exportOpen || !!store.exportMemo || askOpen(),
      can: prefs.can,
    })
    if (!act) return
    if (preventsDefault(act)) e.preventDefault()
    run(act, e)
  }

  // Отпустили Пробел: коротко и без панорамы — следующая (с Shift — предыдущая) раскладка; у сетки — нет.
  function keyup(e: KeyboardEvent) {
    if (e.key !== ' ') return
    const step = canvas.spaceUp()
    if (step) run(step > 0 ? 'nextLayout' : 'prevLayout')
  }
</script>

<svelte:window onkeydown={keys} onkeyup={keyup} onpointerdowncapture={zonePress} />

<main>
  <TopBar />
  <div class="work">
    <div class="col">
      <SpreadBar />
      {#if gallery.cur}
        <LayoutGallery />
      {/if}
      <MissingBar />
      <Stage />
      <Timeline />
      <Library />
    </div>
    <DetailsPanel />
  </div>
</main>

<Overlay />
<CopyGridDialog />
<ExportDialog />
<FsferaDone />
<AutoAskDialog />
<DropAskDialog />
<AutoBuildDialog />
<StartDialog />
<Guide />
<HotkeysHelp />
<LevelsDialog />
<BackupsDialog />
<NamesDialog />
<VigDialog />
<VigTplSave />
<DecorDialog />
<PhotoMenu />
<PhotoPreview />
<Spot />

<style>
  main {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }
  /* Рабочая область под шапкой: колонка с разворотом и лентами | панель деталей */
  .work {
    flex: 1;
    min-height: 0;
    display: flex;
  }
  .col {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
</style>
