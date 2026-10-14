<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { edit } from '../../editor/text/edit.svelte'
  import { parseExportRange } from './exportrange'
  import {
    DPI_PRESETS,
    QUALITIES,
    buildOptions,
    coverAvailable,
    estimateBytes,
    exportWarnings,
    formatSize,
    loadPrefs,
    sampleName,
    savePrefs,
    sizeSummary,
    type ExportPrefs,
    type ExportScope,
  } from './exportopts'
  import Modal from '../../base/ui/Modal.svelte'
  import { isSheet } from '../../base/geom/sheet'

  // Окно «Экспорт» (спека 009 U8, design §6.4): что выгрузить, как резать, качество и dpi.
  // Открыто, пока store.exportOpen; дальше всё ведёт Go (вопросы, диалог папки, прогресс).
  const spreads = $derived(store.view.spreads)
  const settings = $derived(store.view.settings)
  // Лист виньетки (031): страниц нет — «По страницам» и «л/п» не предлагаются, файл на лист.
  const sheet = $derived(isSheet(settings))

  let scope = $state<ExportScope>('all')
  let text = $state('')
  let prefs = $state<ExportPrefs>(loadPrefs())
  let busy = $state(false)
  let field = $state<HTMLInputElement>()

  // Каждое открытие — заново: выбор из localStorage, диапазон «Все».
  $effect(() => {
    if (!store.exportOpen) return
    untrack(() => {
      scope = 'all'
      text = ''
      prefs = loadPrefs()
      busy = false
    })
  })

  const current = $derived(store.current + 1)
  const parsed = $derived(parseExportRange(text, spreads.length, sheet))
  // Пункты обложки — только если она есть (и это не лист-виньетка); «только обложка» гасит выбор разворотов.
  const hasCover = $derived(coverAvailable(store.view))
  const only = $derived(hasCover && prefs.coverOnly)
  const bad = $derived(!only && scope === 'range' && !!parsed.error)
  const can = $derived(!busy && !bad)

  const album = $derived(settings.dpi)
  const albumIsPreset = $derived(DPI_PRESETS.some((d) => d.v === album))
  // Выбранное dpi в числе; 0 в выборе — «как в альбоме».
  const dpiNow = $derived(prefs.dpi > 0 ? prefs.dpi : album)
  const unit = $derived(sheet ? 'лист' : prefs.pages ? 'страницу' : 'разворот')
  // Итог по выбранному объёму (U22) и предупреждения из данных альбома (U8).
  const summary = $derived(
    sizeSummary(settings, dpiNow, prefs.quality, prefs.pages, unit, {
      scope,
      spreads: spreads.length,
      units: parsed.error ? null : parsed.units,
      cover: hasCover && (prefs.cover || only),
      only,
    }),
  )
  const warn = $derived(exportWarnings(store.view, store.missing.length))

  function setPrefs(p: Partial<ExportPrefs>) {
    prefs = { ...prefs, ...p }
    savePrefs(prefs)
  }
  // Кнопка, совпадающая с dpi альбома, остаётся «из альбома» (0): смена настроек альбома её подхватит.
  const setDpi = (v: number) => setPrefs({ dpi: v === album ? 0 : v })

  function pick(s: ExportScope) {
    scope = s
    if (s === 'range') queueMicrotask(() => field?.focus())
  }

  function close() {
    store.exportOpen = false
  }

  async function go() {
    if (!can) return
    busy = true
    await edit.idle() // набранное в тексте должно попасть в файл
    const ok = await store.runExport(
      buildOptions(scope, current, text, sheet ? { ...prefs, pages: false } : prefs, hasCover, store.onCover),
    )
    busy = false
    if (ok) close()
  }
  const enter = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      void go()
    }
  }
</script>

{#if store.exportOpen}
  <Modal title="Экспорт" onclose={close} wide>
    {#if hasCover}
      <div class="group">
        <span class="cap">Обложка</span>
        <div class="opts">
          <label
            ><input
              type="checkbox"
              checked={prefs.cover || only}
              disabled={only}
              onchange={(e) => setPrefs({ cover: e.currentTarget.checked })}
            />Обложка <span class="hint">(00.jpg)</span></label
          >
          <label
            ><input type="checkbox" checked={prefs.coverOnly} onchange={(e) => setPrefs({ coverOnly: e.currentTarget.checked })} />Только
            обложка</label
          >
        </div>
      </div>
    {/if}

    <div class="group" class:off={only} role="radiogroup" aria-label="Что выгрузить">
      <span class="cap">{sheet ? 'Листы' : 'Развороты'}</span>
      <div class="opts">
        <label
          ><input type="radio" name="ex-scope" disabled={only} checked={scope === 'all'} onchange={() => pick('all')} />Все ({spreads.length})</label
        >
        <label
          ><input type="radio" name="ex-scope" disabled={only} checked={scope === 'current'} onchange={() => pick('current')} />Текущий ({store.onCover &&
          hasCover
            ? 'обложка'
            : current})</label
        >
        <label class="inline"
          ><input type="radio" name="ex-scope" disabled={only} checked={scope === 'range'} onchange={() => pick('range')} />Диапазон:
          <input
            type="text"
            class="ranges"
            class:bad
            placeholder={sheet ? '3-5, 8' : '3-5, 8, 7п'}
            aria-label="Диапазон экспорта"
            aria-invalid={bad}
            disabled={only}
            bind:this={field}
            bind:value={text}
            onfocus={() => (scope = 'range')}
            onkeydown={enter}
          /></label
        >
      </div>
    </div>
    {#if scope === 'range' && !only}
      {#if parsed.error}<p class="err">{parsed.error}</p>{/if}
      <p class="hint">
        {sheet ? '3-5, 8 — номера листов.' : '3-5, 8 — развороты; 7л, 7п — левая или правая страница.'}
      </p>
    {/if}

    {#if !sheet}
      <div class="group" role="radiogroup" aria-label="Вид">
        <span class="cap">Вид</span>
        <div class="opts">
          <label
            ><input type="radio" name="ex-pages" checked={!prefs.pages} onchange={() => setPrefs({ pages: false })} />Разворотами
            <span class="hint">{sampleName(current, spreads.length, false)}</span></label
          >
          <label
            ><input type="radio" name="ex-pages" checked={prefs.pages} onchange={() => setPrefs({ pages: true })} />По страницам
            <span class="hint">{sampleName(current, spreads.length, true)}</span></label
          >
        </div>
      </div>
    {/if}

    <div class="group" role="radiogroup" aria-label="Качество">
      <span class="cap">Качество</span>
      <div class="opts">
        {#each QUALITIES as q (q.v)}
          <button
            class="chip"
            class:on={prefs.quality === q.v}
            role="radio"
            aria-checked={prefs.quality === q.v}
            onclick={() => setPrefs({ quality: q.v })}
          >
            <b>{q.v}</b>
            <span>{q.label}</span>
            <small>{formatSize(estimateBytes(settings, dpiNow, q.v, prefs.pages))}</small>
          </button>
        {/each}
      </div>
    </div>

    <div class="group" role="radiogroup" aria-label="Разрешение">
      <span class="cap">Dpi</span>
      <div class="opts">
        {#if !albumIsPreset}
          <button class="chip" class:on={prefs.dpi === 0} role="radio" aria-checked={prefs.dpi === 0} onclick={() => setPrefs({ dpi: 0 })}>
            <b>{album}</b>
            <span>из альбома</span>
          </button>
        {/if}
        {#each DPI_PRESETS as d (d.v)}
          <button class="chip" class:on={dpiNow === d.v} role="radio" aria-checked={dpiNow === d.v} onclick={() => setDpi(d.v)}>
            <b>{d.v}</b>
            <span>{d.label}</span>
          </button>
        {/each}
      </div>
    </div>
    <p class="hint size">{summary}.</p>
    {#if warn}<p class="warn">{warn}.</p>{/if}

    <div class="actions">
      <button class="ghost" onclick={close}>Отмена</button>
      <button class="primary" data-autofocus disabled={!can} onclick={go}>Выгрузить</button>
    </div>
  </Modal>
{/if}

<style>
  /* Окно шире обычного: строки «Развороты» и «Вид» помещаются в одну строку */
  :global(.box.wide):has(.opts) {
    width: 560px;
  }
  /* Подпись строки — левая колонка 64 px, варианты — правая; при переносе держатся левого края вариантов. */
  .group {
    display: grid;
    grid-template-columns: 64px 1fr;
    align-items: center;
    gap: 0 14px;
    margin-bottom: 12px;
  }
  .opts {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 14px;
    min-width: 0;
  }
  .cap {
    color: var(--muted);
    font-size: 12px;
  }
  label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }
  input[type='radio'],
  input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }
  .group.off {
    opacity: 0.5;
  }
  .inline {
    flex-wrap: wrap;
  }
  .ranges {
    width: 140px;
    height: 26px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .ranges.bad {
    border-color: #c0392b;
  }
  .err {
    margin: -6px 0 4px 78px;
    color: #e5645a;
    font-size: 12px;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    margin: 0;
  }
  p.hint {
    margin: -6px 0 12px 78px;
  }
  p.hint.size {
    margin: -4px 0 0;
  }
  p.warn {
    margin: 8px 0 0;
    color: var(--text);
    font-size: 12px;
  }
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
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
