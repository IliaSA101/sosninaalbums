<script lang="ts">
  import { untrack } from 'svelte'
  import { Api, store } from '../../base/core/store.svelte'
  import { jobs } from '../../base/core/jobs.svelte'
  import type { PrintPlan } from '../../base/core/types'
  import { edit } from '../../editor/text/edit.svelte'
  import { fileCount } from '../../panels/library/missing'
  import { parseExportRange, rangeSummary } from './exportrange'
  import ExportRun from './ExportRun.svelte'
  import ExportGroup from './ExportGroup.svelte'
  import ExportQuality from './ExportQuality.svelte'
  import PrintGroup from './PrintGroup.svelte'
  import {
    activePlan,
    buildOptions,
    canExport,
    coverAvailable,
    exportWarnings,
    limitWarn,
    PRINT_DPI,
    loadPrefs,
    planStale,
    printQuality,
    sampleName,
    savePrefs,
    sizeSummary,
    type ExportPrefs,
    type ExportScope,
  } from './exportopts'
  import Modal from '../../base/modal/Modal.svelte'
  import { isSheet } from '../../base/geom/sheet'

  // Окно «Экспорт» (спека 009 U8, design §6.4): что выгрузить, как резать, качество и dpi.
  // Открыто, пока jobs.exportOpen; дальше всё ведёт Go (вопросы, диалог папки, прогресс).
  // Пока выгрузка идёт (jobs.exporting), вместо настроек — ход (ExportRun); крестик окно прячет, выгрузка продолжается (073).
  const spreads = $derived(store.view.spreads)
  const settings = $derived(store.view.settings)
  // Лист виньетки (031): страниц нет — «По страницам» и «л/п» не предлагаются, файл на лист.
  const sheet = $derived(isSheet(settings))

  let scope = $state<ExportScope>('all')
  let text = $state('')
  let prefs = $state<ExportPrefs>(loadPrefs())
  let busy = $state(false)
  let field = $state<HTMLInputElement>()

  // Готовность к выгрузке для типографий (047, 081): Go считает при открытии окна и при каждой правке альбома.
  // null — ответа ещё нет; [] — для страницы нет ни одной типографии. Номер запроса отсекает запоздавший ответ.
  let plans = $state<PrintPlan[] | null>(null)
  let planReq = 0
  $effect(() => {
    void store.view.rev
    if (!jobs.exportOpen) return
    const my = ++planReq
    Api.PrintPlans()
      .then((p) => {
        if (!planStale(my, planReq)) plans = (p ?? []) as PrintPlan[]
      })
      .catch(() => {
        if (!planStale(my, planReq)) plans = [] // без планов окно работает как обычно
      })
  })
  // Каждое открытие — заново: выбор из localStorage, диапазон «Все».
  $effect(() => {
    if (!jobs.exportOpen) return
    untrack(() => {
      scope = 'all'
      text = ''
      prefs = loadPrefs()
      busy = false
      plans = null // планы прошлого открытия не показываем и не берём в расчёт
    })
  })

  // Режим печати включён: у страницы есть план этой типографии (формат из её таблицы, не лист 031) и флажок выбран.
  const plan = $derived(activePlan(plans, prefs.print))
  const fs = $derived(!!plan)

  const current = $derived(store.current + 1)
  const parsed = $derived(parseExportRange(text, spreads.length, sheet))
  // Пункты обложки — только если она есть (и это не лист-виньетка); «только обложка» гасит выбор разворотов.
  // В режиме печати выбор разворотов, вида и dpi серый: выгружается всё разворотами в 300 dpi (Go игнорирует эти поля).
  const hasCover = $derived(coverAvailable(store.view))
  const only = $derived(hasCover && prefs.coverOnly && !fs)
  const scopeNow = $derived(fs ? 'all' : scope)
  const pagesNow = $derived(!fs && prefs.pages)
  const bad = $derived(!only && !fs && scope === 'range' && !!parsed.error)
  // Пока планы не пришли, а типография выбрана, выгрузка закрыта (блок неизвестен).
  const can = $derived(canExport({ busy, bad, printPref: prefs.print, plans }))

  const album = $derived(settings.dpi)
  // Выбранное dpi в числе; 0 в выборе — «как в альбоме».
  const dpiNow = $derived(fs ? PRINT_DPI : prefs.dpi > 0 ? prefs.dpi : album)
  // Качество: у типографии со своим (Фабрика — 100) выбор окна серый и показывает его.
  const qualityNow = $derived(printQuality(plan, prefs.quality))
  const unit = $derived(sheet ? 'лист' : pagesNow ? 'страницу' : 'разворот')
  // Итог по выбранному объёму (U22) и предупреждения из данных альбома (U8).
  const summary = $derived(
    sizeSummary(settings, dpiNow, qualityNow, pagesNow, unit, {
      scope: scopeNow,
      spreads: spreads.length,
      units: parsed.error ? null : parsed.units,
      cover: hasCover && (fs || prefs.cover || only),
      only,
      print: fs,
    }),
  )
  // Пропавшие — по файлам: копии одного файла — одно фото (098).
  const warn = $derived(exportWarnings(store.view, fileCount(store.missing)))
  const limit = $derived(limitWarn(store.view, fs))

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
    jobs.exportOpen = false
  }

  async function go() {
    if (!can) return
    busy = true
    const printPlan = plan // план этого запуска — для окна «Готово» (заголовок, сайт, папка)
    await edit.idle() // набранное в тексте должно попасть в файл
    jobs.exportPrint = printPlan
    // Окно остаётся: при успехе в нём пойдёт ход (jobs.exporting), при отказе Go — прежние настройки.
    await jobs.runExport(buildOptions(scope, current, text, sheet ? { ...prefs, pages: false } : prefs, hasCover, store.onCover, printPlan))
    busy = false
  }
  const enter = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      void go()
    }
  }
</script>

{#if jobs.exportOpen}
  <Modal title="Экспорт" onclose={close} wide>
    {#if jobs.exporting}
      <ExportRun />
    {:else}
      <PrintGroup plans={plans ?? []} pref={prefs.print} {plan} {hasCover} setPref={(v) => setPrefs({ print: v })} />

      {#if hasCover}
        <ExportGroup cap="Обложка" off={fs}>
          <label
            ><input
              type="checkbox"
              checked={fs || prefs.cover || only}
              disabled={only || fs}
              onchange={(e) => setPrefs({ cover: e.currentTarget.checked })}
            />Обложка <span class="hint">(00.jpg)</span></label
          >
          <label
            ><input
              type="checkbox"
              checked={prefs.coverOnly && !fs}
              disabled={fs}
              onchange={(e) => setPrefs({ coverOnly: e.currentTarget.checked })}
            />Только обложка</label
          >
        </ExportGroup>
      {/if}

      <ExportGroup cap={sheet ? 'Листы' : 'Развороты'} off={only || fs} role="radiogroup" label="Что выгрузить">
        <label
          ><input type="radio" name="ex-scope" disabled={only || fs} checked={scopeNow === 'all'} onchange={() => pick('all')} />Все ({spreads.length})</label
        >
        <label
          ><input
            type="radio"
            name="ex-scope"
            disabled={only || fs}
            checked={scopeNow === 'current'}
            onchange={() => pick('current')}
          />Текущий ({store.onCover && hasCover ? 'обложка' : current})</label
        >
        <label class="inline"
          ><input
            type="radio"
            name="ex-scope"
            disabled={only || fs}
            checked={scopeNow === 'range'}
            onchange={() => pick('range')}
          />Диапазон:
          <input
            type="text"
            class="ranges"
            class:bad
            placeholder={sheet ? '3-5, 8' : '3-5, 8, 7п'}
            aria-label="Диапазон экспорта"
            aria-invalid={bad}
            disabled={only || fs}
            bind:this={field}
            bind:value={text}
            onfocus={() => (scope = 'range')}
            onkeydown={enter}
          /></label
        >
      </ExportGroup>
      {#if scope === 'range' && !only && !fs}
        {#if parsed.error}<p class="err">{parsed.error}</p>{:else}<p class="hint" role="status">
            {rangeSummary(parsed.units, sheet, hasCover && prefs.cover)}
          </p>{/if}
        <p class="hint">
          {sheet ? '3-5, 8 — номера листов.' : '3-5, 8 — развороты; 7л, 7п — левая или правая страница.'}
        </p>
      {/if}

      {#if !sheet}
        <ExportGroup cap="Вид" off={fs} role="radiogroup" label="Вид">
          <label
            ><input type="radio" name="ex-pages" checked={!pagesNow} disabled={fs} onchange={() => setPrefs({ pages: false })} />Разворотами
            <span class="hint">{sampleName(current, spreads.length, false)}</span></label
          >
          <label
            ><input type="radio" name="ex-pages" checked={pagesNow} disabled={fs} onchange={() => setPrefs({ pages: true })} />По страницам
            <span class="hint">{sampleName(current, spreads.length, true)}</span></label
          >
        </ExportGroup>
      {/if}

      <ExportQuality
        {prefs}
        {settings}
        {dpiNow}
        {pagesNow}
        {fs}
        {qualityNow}
        qualityLocked={(plan?.quality ?? 0) > 0}
        {setPrefs}
        {setDpi}
      />
      <p class="hint size">{summary}.</p>
      {#if limit}<p class="warn">{limit}.</p>{/if}
      {#if warn}<p class="warn">{warn}.</p>{/if}

      <div class="actions">
        <button class="ghost" onclick={close}>Отмена</button>
        <button class="primary" data-autofocus disabled={!can} onclick={go}>Выгрузить</button>
      </div>
    {/if}
  </Modal>
{/if}

<style>
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
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
