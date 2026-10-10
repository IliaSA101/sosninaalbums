<script lang="ts">
  import { onMount } from 'svelte'
  import TopBar from './shell/menu/TopBar.svelte'
  import SpreadBar from './editor/spread/SpreadBar.svelte'
  import LayoutGallery from './panels/layouts/LayoutGallery.svelte'
  import MissingBar from './panels/library/MissingBar.svelte'
  import Stage from './editor/canvas/Stage.svelte'
  import Timeline from './panels/timeline/Timeline.svelte'
  import Library from './panels/library/Library.svelte'
  import DetailsPanel from './panels/details/DetailsPanel.svelte'
  import Overlay from './base/drag/Overlay.svelte'
  import CopyGridDialog from './panels/layouts/CopyGridDialog.svelte'
  import ExportDialog from './shell/export/ExportDialog.svelte'
  import AutoAskDialog from './shell/dialogs/AutoAskDialog.svelte'
  import HotkeysHelp from './shell/menu/HotkeysHelp.svelte'
  import StartDialog from './shell/dialogs/StartDialog.svelte'
  import LevelsDialog from './shell/dialogs/LevelsDialog.svelte'
  import BackupsDialog from './shell/dialogs/BackupsDialog.svelte'
  import { backupsUi } from './shell/dialogs/backups.svelte'
  import PhotoMenu from './editor/photo/PhotoMenu.svelte'
  import PhotoPreview from './editor/photo/PhotoPreview.svelte'
  import { store } from './base/core/store.svelte'
  import { gridAct } from './panels/layouts/gridact.svelte'
  import { gallery } from './panels/layouts/gallery.svelte'
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
    const z = zoneOf(e.target as Element | null)
    if (z) store.zone = z
  }

  function keys(e: KeyboardEvent) {
    const t = e.target as HTMLElement
    // Пока открыта справка, клавиши не работают; Esc её закрывает (если фокус ушёл из окна).
    if (help.open) {
      if (e.key === 'Escape') help.open = false
      if (e.key === 'F1') e.preventDefault()
      return
    }
    // Стартовое окно и окно уровней: клавиши под ними молчат (Esc закрывает само окно).
    if (prefs.startOpen || prefs.compare || backupsUi.open) return
    // Хоткеи глушим только при вводе текста/чисел (в том числе в contenteditable); select и ползунок их не блокируют.
    const act = keyAction(e, {
      inTextField: isTextField(t),
      onCheckbox: t.matches("input[type='checkbox']"),
      galleryOpen: !!gallery.cur,
      hasSelection: store.selectedIds.length > 0,
      gridSpread: !!store.spread?.grid,
      placeSelected: placesel.selected.length > 0 || placesel.texts.length > 0,
      layerable: placesel.canLayer,
      zone: store.zone,
      bgMode: !!bgmode.cur,
      big: modes.big,
      modal: store.exportOpen || !!gridAct.autoAsk || !!gridAct.copyGrid,
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
<AutoAskDialog />
<HotkeysHelp />
<StartDialog />
<LevelsDialog />
<BackupsDialog />
<PhotoMenu />
<PhotoPreview />

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
