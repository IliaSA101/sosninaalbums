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
  import CollectDialog from './shell/collect/CollectDialog.svelte'
  import PrintDone from './shell/export/PrintDone.svelte'
  import AutoAskDialog from './shell/ask/AutoAskDialog.svelte'
  import DropAskDialog from './shell/ask/DropAskDialog.svelte'
  import AutoBuildDialog from './shell/autobuild/AutoBuildDialog.svelte'
  import KeysDialog from './shell/hotkeys/KeysDialog.svelte'
  import Guide from './shell/guide/Guide.svelte'
  import Spot from './shell/guide/Spot.svelte'
  import { guide } from './shell/guide/guidestate.svelte'
  import StartDialog from './shell/dialogs/StartDialog.svelte'
  import OpenProgress from './shell/dialogs/OpenProgress.svelte'
  import LevelsDialog from './shell/dialogs/LevelsDialog.svelte'
  import BackupsDialog from './shell/dialogs/BackupsDialog.svelte'
  import { backupsUi } from './shell/dialogs/backups.svelte'
  import PacksDialog from './shell/dialogs/PacksDialog.svelte'
  import { packsUi } from './shell/dialogs/packs.svelte'
  import NamesDialog from './panels/names/NamesDialog.svelte'
  import VigDialog from './panels/vignette/VigDialog.svelte'
  import VigTplSave from './panels/vignette/VigTplSave.svelte'
  import DecorDialog from './panels/decor/DecorDialog.svelte'
  import { vigDialog } from './panels/vignette/vigsession.svelte'
  import { namesUi } from './panels/names/names.svelte'
  import PhotoMenu from './editor/photo/PhotoMenu.svelte'
  import PhotoPreview from './editor/photo/PhotoPreview.svelte'
  import { store } from './base/core/store.svelte'
  import { autocheck } from './panels/folders/autocheck'
  import { jobs } from './base/core/jobs.svelte'
  import { gridAct } from './panels/layouts/gridact.svelte'
  import { dropAsk } from './base/drag/dropgrid.svelte'
  import { gallery } from './panels/gallery/gallery.svelte'
  import { placesel } from './editor/places/placesel.svelte'
  import { canvas } from './editor/canvas/canvas.svelte'
  import { run, runKey } from './shell/menu/commands'
  import { keysUi } from './shell/hotkeys/keysui.svelte'
  import { guardsBrowser, isTextField, isWidget, keyCommand, preventsDefault, type DialogState } from './shell/menu/keys'
  import { menuOn } from './shell/menu/menuctx'
  import { eventChord, isSpace } from './base/keys/chord'
  import { modalStack } from './base/modal/modalstack.svelte'
  import { zoneOf } from './editor/places/groupsel'
  import { bgmode } from './editor/backdrop/bgmode.svelte'
  import { prefs } from './base/prefs/prefs.svelte'
  import { modes } from './base/prefs/view.svelte'

  onMount(() => {
    store.init()
    // Сверка папок по фокусу и таймеру (088): удалённые в Проводнике файлы уходят из ленты, новые появляются.
    autocheck.start(window, document)
    return () => autocheck.stop()
  })

  // Зона последнего нажатия: Ctrl+A и Del действуют по ней (библиотека или холст с лентой альбома).
  function zonePress(e: PointerEvent) {
    const z = zoneOf(e.target instanceof Element ? e.target : null)
    if (z) store.zone = z
  }

  // Пробел листает раскладки «Авто»: у сетки их нет, у обложки (032) тоже.
  const noAutoLayouts = () => !!store.spread?.grid || store.onCover

  // Открыт вопрос ручного листа (автораскладка, броски, копия сетки): под ним F1 молчит.
  const askOpen = () => !!gridAct.autoAsk || !!dropAsk.cur || !!gridAct.copyGrid

  // Открыто окно фоновой работы Go: экспорт, его памятка, сбор в папку (069) или «Открываем альбом» (073: F1 не открывает справочник под ним).
  const jobsOpen = () => jobs.exportOpen || jobs.collectOpen || !!jobs.exportMemo || jobs.openShown

  // F1 — справочник функций (033). В WebView2 он открыл бы справку браузера, поэтому гасится всегда.
  // Поверх стартового окна и окна уровней справочник открывается; под другими окнами и сам справочник — нет.
  function f1(e: KeyboardEvent): boolean {
    if (eventChord(e) !== 'F1') return false
    e.preventDefault()
    const other =
      keysUi.open || backupsUi.open || packsUi.open || namesUi.open || !!vigDialog.id || !!store.autoDraft || jobsOpen() || askOpen()
    if (!guide.open && !other) guide.openFresh()
    return true
  }

  // Окно открыто: отмена и повтор — только под окнами, что сами правят альбом (Modal undo), под остальными клавиши молчат (074-18, 074-28).
  const dialogState = (): DialogState | undefined => (modalStack.any() ? (modalStack.topUndo() ? 'undo' : 'shut') : undefined)
  // Цель события бывает не Element (document/window, синтетическое событие) — тогда null.
  const targetEl = (e: Event) => (e.target instanceof Element ? e.target : null)
  const isCheckbox = (t: Element | null) => !!t?.matches("input[type='checkbox']")

  function keys(e: KeyboardEvent) {
    const t = targetEl(e)
    if (f1(e)) return
    // Клавиши глушим при вводе текста/чисел (в том числе в contenteditable); ползунку, select и списку остаются их стрелки.
    const id = keyCommand(e, {
      inTextField: isTextField(t),
      widget: isWidget(t),
      popup: !!t?.closest('[data-barpop]'),
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
      dialog: dialogState(),
      can: prefs.can,
      menuOn,
    })
    if (!id) {
      // Перезагрузка и «назад» страницы гасит страж. Удержанный Пробел команду не повторяет (автоповтор — только у команд
      // с флагом), но и странице не достаётся.
      if (guardsBrowser(e) || (e.repeat && isSpace(e) && !isTextField(t) && !isCheckbox(t))) e.preventDefault()
      return
    }
    if (preventsDefault(id, e)) e.preventDefault()
    runKey(id, e)
  }

  // Отпустили Пробел: коротко и без панорамы — следующая (с Shift — предыдущая) раскладка; у сетки — нет.
  function keyup(e: KeyboardEvent) {
    if (!isSpace(e)) return
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
<CollectDialog />
<PrintDone />
<AutoAskDialog />
<DropAskDialog />
<AutoBuildDialog />
<StartDialog />
<Guide />
<KeysDialog />
<LevelsDialog />
<BackupsDialog />
<PacksDialog />
<NamesDialog />
<VigDialog />
<VigTplSave />
<DecorDialog />
<PhotoMenu />
<PhotoPreview />
<Spot />
<!-- Последним: окно хода открытия (073) лежит над стартовым окном -->
<OpenProgress />

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
