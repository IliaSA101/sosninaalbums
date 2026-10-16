<script lang="ts" module>
  // «Скрыть»: ключ набора пропавших (missingKey). Только в памяти: новое пропавшее фото или новое открытие — полоса снова.
  let hidden = $state('')
  let hiddenOwn = $state('') // то же для полосы про свои из библиотеки (048)
</script>

<script lang="ts">
  import { slide } from 'svelte/transition'
  import { store, Api } from '../../base/core/store.svelte'
  import { unitOf } from '../../base/geom/sheet'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { relinkFolder } from './relink'
  import {
    albumPhotos,
    driveLabel,
    isHidden,
    libMissingText,
    missingKey,
    missingSpots,
    missingText,
    missingWhere,
    showMissing,
    splitMissing,
    spotLabel,
    type DriveHint,
    type Spot,
  } from './missing'

  // Свои из библиотеки (048) — отдельной полосой без «Найти…»: их ищет загрузка того же файла, а не папка.
  const split = $derived(splitMissing(store.missing, store.view.libPhotos))
  const count = $derived(split.photos.length)
  const total = $derived(albumPhotos(store.sheets))
  const unit = $derived(unitOf(store.view.settings))
  const spots = $derived(missingSpots(store.sheets, split.photos))
  const text = $derived(missingText(count, total, missingWhere(spots, unit)))
  const key = $derived(missingKey(split.photos, store.openSeq))
  const ownText = $derived(libMissingText(split.own.length, missingWhere(missingSpots(store.sheets, split.own), unit)))
  const ownKey = $derived(missingKey(split.own, store.openSeq))

  // Список «Показать»: переход к листу и выделение фото; окно закрывается.
  let listOpen = $state(false)
  let listBtn = $state<HTMLButtonElement>()
  function show(s: Spot) {
    listOpen = false
    store.goId(s.sheet)
    store.selectedCell = s.photo
  }

  // Подсказка смены диска: список недостающих меняется — спрашиваем Go заново.
  // Номер запроса отсекает запоздавший ответ.
  let hint = $state<DriveHint | null>(null)
  let req = 0
  $effect(() => {
    const n = count
    const my = ++req
    if (n === 0) {
      hint = null
      return
    }
    Api.SuggestDrive()
      .then((h) => {
        if (my === req) hint = h
      })
      .catch(() => {
        if (my === req) hint = null
      })
  })
  const drive = $derived(driveLabel(hint))

  let busy = $state(false)

  async function relinkDrive() {
    if (!hint || busy) return
    busy = true
    try {
      await store.run(Api.RelinkDrive(hint.from, hint.to))
    } finally {
      busy = false
    }
  }

  // Убрать из альбома все пропавшие фото: места остаются пустыми, Undo возвращает.
  async function removeAll() {
    if (busy || spots.length === 0) return
    busy = true
    try {
      const n = spots.length
      if (await store.run(Api.RemovePhotos(spots.map((s) => s.photo)))) store.notify(`Убрано фото: ${n}`)
    } finally {
      busy = false
    }
  }

  async function findFolder() {
    if (busy) return
    busy = true
    try {
      await relinkFolder()
    } finally {
      busy = false
    }
  }
</script>

{#if showMissing(count) && !isHidden(hidden, key)}
  <div class="bar" role="status" data-guide="missing-bar" transition:slide={{ duration: 150 }}>
    <span class="msg" title={text}><i aria-hidden="true">!</i><span class="txt">{text}</span></span>
    {#if drive}
      <button class="accent" disabled={busy} onclick={relinkDrive} title="Заменить букву диска у недостающих фото">{drive}</button>
    {/if}
    {#if spots.length > 0}
      <button
        bind:this={listBtn}
        aria-haspopup="dialog"
        aria-expanded={listOpen}
        onclick={() => (listOpen = !listOpen)}
        title="Список пропавших фото: выбрать — перейти к фото">Показать ▾</button
      >
    {/if}
    <button disabled={busy} onclick={findFolder} title="Выбрать папку, где лежат эти фото">Найти…</button>
    {#if spots.length > 0}
      <button disabled={busy} onclick={removeAll} title="Убрать пропавшие фото из альбома: места останутся пустыми, Ctrl+Z вернёт"
        >Убрать из альбома</button
      >
    {/if}
    <button class="x ghost" onclick={() => (hidden = key)} title="Скрыть до следующего открытия" aria-label="Скрыть до следующего открытия"
      ><Icon name="x" size={14} /></button
    >
  </div>
  <Popover anchor={listBtn} bind:open={listOpen} label="Пропавшие фото" kind="missing">
    <ul class="list">
      {#each spots as s (s.photo)}
        {@const label = spotLabel(s, unit, store.missingNames)}
        <li><button class="ghost row" title={label} onclick={() => show(s)}>{label}</button></li>
      {/each}
    </ul>
  </Popover>
{/if}

{#if split.own.length > 0 && !isHidden(hiddenOwn, ownKey)}
  <div class="bar" role="status" transition:slide={{ duration: 150 }}>
    <span class="msg" title={ownText}><i aria-hidden="true">!</i><span class="txt">{ownText}</span></span>
    <button
      class="x ghost"
      onclick={() => (hiddenOwn = ownKey)}
      title="Скрыть до следующего открытия"
      aria-label="Скрыть до следующего открытия"><Icon name="x" size={14} /></button
    >
  </div>
{/if}

<style>
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    height: 40px;
    margin: 0 16px 4px;
    padding: 0 12px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel);
    font-size: 13px;
    overflow: hidden;
  }
  .msg {
    min-width: 0;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--accent);
  }
  .txt {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .x {
    width: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    flex: none;
  }
  /* Не больше 10 строк на виду, дальше прокрутка. */
  .list {
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: 280px;
    overflow-y: auto;
    min-width: 220px;
  }
  .row {
    width: 100%;
    max-width: 420px;
    height: 28px;
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Значок «!» без Icon.svelte: общий набор значков правят параллельно. */
  i {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--accent);
    color: #1e1e1e;
    font:
      700 11px/16px 'Segoe UI',
      sans-serif;
    text-align: center;
  }
  .bar button {
    height: 26px;
    padding: 0 10px;
    flex: none;
    white-space: nowrap;
  }
</style>
