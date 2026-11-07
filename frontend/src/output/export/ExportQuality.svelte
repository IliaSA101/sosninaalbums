<script lang="ts">
  import type { app } from '../../../wailsjs/go/models'
  import ExportGroup from './ExportGroup.svelte'
  import { DPI_PRESETS, QUALITIES, type ExportPrefs } from './exportopts'

  // Разделы «Качество» и «Dpi» окна «Экспорт» (вынесено из ExportDialog, 075-11).
  // dpiNow — выбранное dpi в числе (в режиме печати 300); album — dpi альбома; qualityNow — качество в числе (у плана со своим — оно);
  // per — объём файла по качествам из ответа Go (пусто, пока ответа нет);
  // qualityLocked — качество задаёт типография (серое, 081); setPrefs/setDpi сохраняют выбор в localStorage.
  let {
    prefs,
    album,
    dpiNow,
    fs,
    qualityNow,
    per,
    qualityLocked,
    setPrefs,
    setDpi,
  }: {
    prefs: ExportPrefs
    album: number
    dpiNow: number
    fs: boolean
    qualityNow: number
    per: app.ExportSizeQ[]
    qualityLocked: boolean
    setPrefs: (p: Partial<ExportPrefs>) => void
    setDpi: (v: number) => void
  } = $props()

  const albumIsPreset = $derived(DPI_PRESETS.some((d) => d.v === album))
</script>

<ExportGroup cap="Качество" off={qualityLocked} role="radiogroup" label="Качество">
  {#each QUALITIES as q (q.v)}
    <button
      class="chip"
      class:on={qualityNow === q.v}
      role="radio"
      aria-checked={qualityNow === q.v}
      disabled={qualityLocked}
      onclick={() => setPrefs({ quality: q.v })}
    >
      <b>{q.v}</b>
      <span>{q.label}</span>
      <small>{per.find((x) => x.quality === q.v)?.text ?? '\u00a0'}</small>
    </button>
  {/each}
</ExportGroup>

<ExportGroup cap="Dpi" off={fs} role="radiogroup" label="Разрешение">
  {#if !albumIsPreset}
    <button
      class="chip"
      class:on={prefs.dpi === 0 && !fs}
      role="radio"
      aria-checked={prefs.dpi === 0 && !fs}
      disabled={fs}
      onclick={() => setPrefs({ dpi: 0 })}
    >
      <b>{album}</b>
      <span>из альбома</span>
    </button>
  {/if}
  {#each DPI_PRESETS as d (d.v)}
    <button class="chip" class:on={dpiNow === d.v} role="radio" aria-checked={dpiNow === d.v} disabled={fs} onclick={() => setDpi(d.v)}>
      <b>{d.v}</b>
      <span>{d.label}</span>
    </button>
  {/each}
</ExportGroup>

<style>
  .chip {
    display: inline-flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
    height: auto;
    min-height: 30px;
    min-width: 84px;
    padding: 4px 8px;
    line-height: 1.25;
  }
  .chip.on {
    border-color: var(--accent);
    color: var(--accent);
  }
  .chip b {
    font-weight: 600;
  }
  .chip span,
  .chip small {
    color: var(--muted);
    font-size: 11px;
  }
</style>
