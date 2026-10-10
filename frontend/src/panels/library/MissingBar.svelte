<script lang="ts">
  import { slide } from 'svelte/transition'
  import { store, Api } from '../../base/core/store.svelte'
  import { relinkFolder } from './relink'
  import { albumPhotos, driveLabel, missingText, showMissing, type DriveHint } from './missing'

  const count = $derived(store.missing.length)
  const total = $derived(albumPhotos(store.view.spreads))

  // Подсказка смены диска: список недостающих меняется — спрашиваем Go заново.
  // Номер запроса отсекает запоздавший ответ.
  let hint = $state<DriveHint | null>(null)
  let req = 0
  $effect(() => {
    const n = store.missing.length
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

{#if showMissing(count)}
  <div class="bar" role="status" transition:slide={{ duration: 150 }}>
    <span class="msg"><i aria-hidden="true">!</i>{missingText(count, total)}</span>
    {#if drive}
      <button class="accent" disabled={busy} onclick={relinkDrive} title="Заменить букву диска у недостающих фото">{drive}</button>
    {/if}
    <button disabled={busy} onclick={findFolder} title="Выбрать папку, где лежат эти фото">Найти…</button>
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
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--accent);
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
  button {
    height: 26px;
    padding: 0 10px;
  }
</style>
