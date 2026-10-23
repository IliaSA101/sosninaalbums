<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { NAME_MAX, nameChanged, normName } from '../../panels/timeline/spreadname'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import InspectorBackdrop from '../backdrop/InspectorBackdrop.svelte'
  import InspectorPattern from '../pattern/InspectorPattern.svelte'
  import CaptionSheet from '../captions/CaptionSheet.svelte'
  import { namesUi } from '../../panels/names/names.svelte'
  import InspectorCover from '../../panels/cover/InspectorCover.svelte'
  import { coverLabel } from '../cover/cover'
  import { isCover, isSheet, unitOf } from '../../base/geom/sheet'
  import { unitWords } from '../captions/unitword'
  import VignettePanel from '../../panels/vignette/VignettePanel.svelte'
  import { canMakeVignette, isVignette } from '../../panels/vignette/vignette'
  import { makeVignette } from '../../panels/vignette/vigact'
  import Icon from '../../base/ui/Icon.svelte'
  import { ALBUM_CARD_EDIT, ALBUM_CARD_HIDE, ALBUM_CARD_NOW, ALBUM_CARD_OFF, ALBUM_CARD_SHOW, albumCard } from '../../panels/decor/albumbg'
  import { decor } from '../../panels/decor/decor.svelte'

  // Карточка «Разворот N», когда ничего не выделено (спека 008, panels.md; 009 D2): название, раздел
  // «Фон» (цвет на весь разворот или по страницам), раскладка «Авто» / «Сетка». «Сетка» закрепляет
  // места, «Авто» на сетке спрашивает (места сбросятся, gridAct.backToAuto).
  // head=false — заголовок рисует сама карточка (docked, cardTitle): сворачиваемая шапка.
  let { head = true }: { head?: boolean } = $props()
  const sp = $derived(store.spread)
  // Виньетка (031, решение 80): вместо «Авто / Ручная» — раздел «Виньетка» с шаблонами и числами.
  const vsp = $derived(isVignette(sp) ? sp : null)
  // Лист виньетки (031): страниц нет — фон только на весь лист, «По страницам» не предлагается.
  const sheet = $derived(isSheet(store.view.settings))
  const unit = $derived(unitOf(store.view.settings))
  // Обложка (032): своя карточка сверху, названия и «Авто / Ручная» нет, страницы называются сторонками.
  const cover = $derived(isCover(sp))
  const cw = $derived(unitWords(store.view.settings, sp))
  const sides = $derived(
    cover
      ? ([
          { scope: 'left', label: coverLabel(store.cover, 'left'), i: 0 },
          { scope: 'right', label: coverLabel(store.cover, 'right'), i: 1 },
        ] as const)
      : ([
          { scope: 'left', label: 'Левая', i: 0 },
          { scope: 'right', label: 'Правая', i: 1 },
        ] as const),
  )

  // «По страницам» раскрыто вручную; если цвета страниц уже разные, раскрыто всегда (один образец
  // соврал бы). Смена разворота сворачивает.
  let splitOpen = $state(false)
  let splitFor = ''
  $effect(() => {
    if (sp && sp.id !== splitFor) {
      splitFor = sp.id
      splitOpen = false
    }
  })
  const differ = $derived(!!sp && sp.bg[0] !== sp.bg[1])
  const split = $derived(!sheet && (splitOpen || differ))
  const own = $derived(!!sp && (sp.bgSet[0] !== '' || sp.bgSet[1] !== ''))

  // Enter или уход из поля — в Go; Esc возвращает прежнее название и не снимает выделение.
  function commit(el: HTMLInputElement) {
    if (!sp) return
    if (nameChanged(sp.name, el.value)) store.renameSpread(sp.id, normName(el.value))
    else el.value = sp.name ?? ''
  }
  function key(e: KeyboardEvent) {
    const el = e.currentTarget as HTMLInputElement
    if (e.key === 'Enter') el.blur()
    else if (e.key === 'Escape') {
      e.stopPropagation()
      el.value = sp?.name ?? ''
      el.blur()
    }
  }
  // «Сделать виньеткой» (отзыв 14.10): видимый путь вместо одного правого щелчка в ленте; тост с «Вернуть» — в makeVignette.
  const vigable = $derived(canMakeVignette(sp, prefs.can))
  let making = $state(false)
  async function toVignette() {
    if (!sp || making) return
    making = true
    try {
      await makeVignette(sp)
    } finally {
      making = false
    }
  }
  // Фон альбома (098): строка в разделе «Фон» — виден ли он на этом листе и как его скрыть; сам фон выбирают в настройках альбома.
  const albumRow = $derived(albumCard(sp, store.view.albumBg))
  const hideAlbum = (off: boolean) => sp && store.run(Api.SetNoAlbumBg(sp.id, off))
  // ColorPicker зовёт onpick один раз на выбор — один шаг undo. Цвет '' снимает свой цвет.
  const pick = (scope: 'spread' | 'left' | 'right', color: string) => sp && store.run(Api.SetBackground(sp.id, scope, color))
</script>

{#if sp}
  {#if head}<div class="head">{cover ? 'Обложка' : `${unit.cap} ${store.current + 1}`}</div>{/if}
  {#if cover}<InspectorCover />{/if}
  {#if prefs.can('names') && !cover}
    <input
      class="name"
      data-guide="spread-title"
      type="text"
      maxlength={NAME_MAX}
      placeholder="Название"
      aria-label="Название {unit.one}а"
      value={sp.name}
      onchange={(e) => commit(e.currentTarget)}
      onkeydown={key}
    />
  {/if}
  {#if vsp}
    <div class="bg" role="group" aria-label="Виньетка">
      <div class="cap">Виньетка</div>
      <!-- Форма берётся при создании панели: для другого листа — заново -->
      {#key vsp.id}<VignettePanel sp={vsp} />{/key}
    </div>
  {/if}
  <!-- Раздел «Фон» живёт, пока есть цвет или фото на фоне (спека 011); без обоих — не рисуется -->
  {#if prefs.can('bgColor') || prefs.can('bgPhoto')}
    <div class="bg" role="group" aria-label="Фон" data-guide="spread-bg">
      <div class="cap">Фон</div>
      {#if prefs.can('bgColor')}
        {#if !split}
          <div class="row">
            <span class="lbl">{cover ? coverLabel(store.cover, 'spread') : unit.all}</span>
            <ColorPicker value={sp.bg[0]} label="Цвет фона {unit.one}а" onpick={(c) => pick('spread', c)} />
          </div>
        {/if}
        {#if !sheet}
          <button class="link" aria-expanded={split} disabled={differ} onclick={() => (splitOpen = !splitOpen)}>
            <svg class="chev" class:open={split} viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
            По страницам
          </button>
        {/if}
        {#if split}
          {#each sides as s (s.scope)}
            <div class="row">
              <span class="lbl">{s.label}</span>
              <ColorPicker value={sp.bg[s.i]} label="Цвет фона: {s.label.toLowerCase()} страница" onpick={(c) => pick(s.scope, c)} />
              <button
                class="reset"
                disabled={sp.bgSet[s.i] === ''}
                aria-label="{s.label} страница: как в альбоме"
                title="Как в альбоме"
                onclick={() => pick(s.scope, '')}>×</button
              >
            </div>
          {/each}
        {/if}
        <button class="ghost" disabled={!own} onclick={() => pick('spread', '')}>Как в альбоме</button>
      {/if}
      {#if albumRow}
        <div class="album">
          {#if albumRow === 'on'}
            <span class="lbl">{ALBUM_CARD_NOW}</span>
            <!-- Без уровня «Фоны» окно не открываем — как «Выбрать…» в AlbumBgRow -->
            {#if prefs.can('decor')}<button class="ghost mini" onclick={() => decor.showAlbum()}>{ALBUM_CARD_EDIT}</button>{/if}
            <button class="ghost mini" onclick={() => hideAlbum(true)}>{ALBUM_CARD_HIDE}</button>
          {:else}
            <span class="lbl">{ALBUM_CARD_OFF}</span>
            <button class="ghost mini" onclick={() => hideAlbum(false)}>{ALBUM_CARD_SHOW}</button>
          {/if}
        </div>
      {/if}
      <!-- Узор программы (048) — строкой «Узор» над зонами: он лежит под фото фона и от областей зон не зависит -->
      {#if prefs.can('bgPhoto')}<div data-slot="backdrop"><InspectorPattern {sp} /><InspectorBackdrop {sp} {split} /></div>{/if}
    </div>
  {/if}
  <!-- Подписи листа (030): общий стиль и шаблон «к каждому фото»; когда выделять нечего (одни заглушки) — здесь -->
  {#if prefs.can('captions') && (sp.caps || sp.captions.length > 0 || sp.photos.length > 0)}
    <div class="bg" role="group" aria-label="Подписи {cw.gen}" data-guide="sheet-caps">
      <div class="cap">Подписи {cw.gen}</div>
      <CaptionSheet {sp} />
      <button
        class="ghost"
        data-guide="names-btn"
        title="Вставить список фамилий и имён по порядку фото {cw.gen}"
        onclick={() => namesUi.show(sp.id)}>Имена из списка…</button
      >
    </div>
  {/if}
  {#if prefs.can('manual') && !cover && !vsp}
    <div class="seg" role="group" aria-label="Раскладка {unit.one}а" data-guide="mode-toggle">
      <button
        class:on={!sp.grid}
        aria-pressed={!sp.grid}
        onclick={() => sp.grid && gridAct.backToAuto(sp.id)}
        title="Фото раскладываются автоматически">Авто</button
      >
      <button
        class:on={sp.grid}
        aria-pressed={sp.grid}
        onclick={() => !sp.grid && gridAct.setGrid(true)}
        title="Сделать ручной: раскладка перестаёт меняться сама, места можно править">Ручная</button
      >
    </div>
  {/if}
  {#if vigable}
    <button
      class="ghost vig"
      data-guide="vig-make"
      disabled={making}
      title="Учителя и ученики с подписями, шапка с полями; фото {unit.of} садятся в места учеников"
      onclick={toVignette}><Icon name="vignette" size={14} />Сделать виньеткой</button
    >
  {/if}
{/if}

<style>
  .head {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .name {
    width: 100%;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .bg {
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
  }
  .cap {
    font-size: 12px;
    font-weight: 600;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    font-size: 12px;
  }
  .lbl {
    flex: 1;
    color: var(--muted);
  }
  .link {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 4px;
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
    cursor: pointer;
  }
  .link:hover:not(:disabled) {
    color: var(--text);
  }
  .link:disabled {
    cursor: default;
    opacity: 0.6;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .chev {
    width: 10px;
    height: 10px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .ghost {
    width: 100%;
    height: 24px;
    margin-top: 8px;
    padding: 0 6px;
    font-size: 12px;
  }
  .album {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 6px;
    margin-top: 8px;
    font-size: 12px;
  }
  .album .lbl {
    flex-basis: 100%;
  }
  .ghost.mini {
    width: auto;
    margin-top: 0;
    padding: 0 8px;
  }
  .vig {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    width: 100%;
    height: 26px;
    margin-top: 10px;
    font-size: 12px;
    border-color: var(--line);
  }
  .reset {
    width: 22px;
    height: 24px;
    padding: 0;
    font-size: 14px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .seg {
    display: flex;
    margin-top: 10px;
    border-radius: 5px;
    overflow: hidden;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    font-size: 12px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
</style>
