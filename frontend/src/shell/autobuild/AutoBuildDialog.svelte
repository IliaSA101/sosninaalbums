<script lang="ts">
  import { untrack } from 'svelte'
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import { store } from '../../base/core/store.svelte'
  import type { SigState } from '../../base/core/types'
  import { modes } from '../../base/prefs/view.svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import AutoLayouts from './AutoLayouts.svelte'
  import AutoPhotos from './AutoPhotos.svelte'
  import AutoRangeRow from './AutoRangeRow.svelte'
  import AutoTiles from './AutoTiles.svelte'
  import AutoMode from '../autoalbum/AutoMode.svelte'
  import AutoVariant from './AutoVariant.svelte'
  import { dropsFn, planFn, previewFn, savedOf, sigsFn } from './autoapi'
  import { ALBUM_BUILD, albumKept } from '../autoalbum/autoalbum'
  import { goBuild } from '../autoalbum/autogo'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { AutoDialog } from './autobuild.svelte'
  import { albumAfter, arrow, sameText, seriesText, spreadsBefore } from './autobuild'
  import { unitOf } from '../../base/geom/sheet'

  // Окно «Автосборка» (027 U2, 034): сколько разворотов собрать — числом N или средним «фото на разворот»
  // (поля связаны). Открыто, пока store.autoDraft; фото выбраны заранее (store.openAuto).
  let ui = $state<AutoDialog | null>(null)
  // Семейства — часть функции «Варианты раскладки»: без неё фильтра нет и в сборку уходит «Все».
  const famOn = $derived(prefs.can('variants'))
  // 078: строка «Ч/б развороты» — часть функции «Ч/б и тон фото»; без неё в Go уходит gray 0
  const grayOn = $derived(prefs.can('adjust'))

  // Каждое открытие — новое состояние: N, среднее, диапазон и серии берутся из памяти окна, ответ Go запрашивается заново.
  $effect(() => {
    const d = store.autoDraft
    untrack(() => {
      ui = d
        ? new AutoDialog(
            d.ids,
            planFn(() => modes.autoVariety),
            savedOf(d, famOn, grayOn),
            previewFn,
            { grayOn, sigs: sigsFn, drops: dropsFn },
          )
        : null
      typing = null
      ui?.start()
    })
    return () => untrack(() => ui?.dispose())
  })

  // Ход подготовки подписей кадров (078-12): «Сверяю кадры: done из total…»
  $effect(() => EventsOn('sigs:progress', (s: SigState) => ui?.ph.onProgress(s)))

  const close = () => (store.autoDraft = null)

  // Пересборка уже ждёт ответа: второй Enter не уходит
  const go = async () => ui && !store.autoWaiting && goBuild(ui, { famOn, grayOn, unit })

  // Какое поле печатают (общее с карточками AutoTiles): ↑↓ сбрасывают набранное, показ берёт фактическое значение.
  let typing = $state<'n' | 'per' | null>(null)

  // ↑↓ в полях — шаг 1 (в «среднем» — показанное целое на 1, N хотя бы на 1), PgUp/PgDn — 5; на ползунке — только PgUp/PgDn.
  function step(t: HTMLElement, d: number, k: number): boolean {
    if (!ui || !d) return false
    if (t.id === 'auto-per') {
      for (let i = 0; i < k; i++) ui.stepPer(d > 0 ? 1 : -1)
      typing = null
    } else if (t.id === 'auto-n') {
      ui.setSpreads(ui.n + d * k)
      typing = null
    } else if (k === 5 && t.matches("input[type='range']")) ui.setSpreads(ui.n + d * k)
    else return false
    return true
  }

  // Enter собирает отовсюду в окне (Modal.onkey), кроме кнопок и «Дополнительно» (там он делает своё).
  function key(e: KeyboardEvent) {
    if (!ui) return
    if (!(e.target instanceof HTMLElement)) return
    const t = e.target
    if (e.key === 'Enter' && !t.closest('button, summary')) {
      e.preventDefault()
      void go()
    } else if (step(t, ...arrow(e.key))) e.preventDefault()
  }

  const unit = $derived(unitOf(store.view.settings))
  const empty = $derived(!!ui?.plan && ui.plan.photos === 0)
  // 107: в режиме альбома якоря остаются на месте, новые развороты встают между ними
  const after = $derived(
    !ui || !store.autoDraft
      ? ''
      : store.autoDraft.from === 'album'
        ? ui.plan?.album
          ? albumAfter(albumKept(ui.plan.album), ui.n, unit)
          : ''
        : albumAfter(spreadsBefore(store.autoDraft.again, store.view.spreads.length, store.lastAuto), ui.n, unit, ui.plan?.reuse ?? 0),
  )
  const top = $derived(ui?.plan ? Math.max(1, ui.plan.maxSpreads) : Math.max(1, ui?.n ?? 1))
  const low = $derived(ui?.plan ? Math.max(1, ui.plan.minSpreads) : 1)
  // Ползунок всегда показывает N: пока min двигался вместе с value, браузер прижимал значение к старому
  // пределу, а Svelte value не переписывал (оно для него не менялось). Пишем явно после смены пределов.
  let range = $state<HTMLInputElement>()
  $effect(() => {
    if (range && ui) {
      void [low, top]
      range.value = String(ui.n)
    }
  })
</script>

{#if store.autoDraft && ui}
  {@const d = store.autoDraft}
  <Modal title="Автосборка" onclose={close} onkey={key} width={860}>
    <div class="body">
      <AutoMode {d} plan={ui.plan} />

      <AutoTiles {ui} {unit} {empty} {low} {top} bind:typing />

      <input
        id="auto-range"
        bind:this={range}
        type="range"
        min={low}
        max={top}
        value={ui.n}
        step="1"
        disabled={empty}
        aria-label="Число {unit.plural[2]}"
        oninput={(e) => ui?.setSpreads(e.currentTarget.valueAsNumber)}
      />
      <p class="after">{after}</p>

      <AutoRangeRow {ui} {unit} {empty} />

      <AutoPhotos ph={ui.ph} plan={ui.plan} {unit} {empty} {grayOn} />

      <AutoLayouts {ui} {famOn} {unit} {empty} />

      <details>
        <summary>Дополнительно</summary>
        <label class="more">
          <input type="checkbox" checked={ui.series} onchange={(e) => ui?.setSeries(e.currentTarget.checked)} />
          Не разрывать серии съёмки (папки, время)
          {#if ui.series && ui.plan && !ui.pending && seriesText(ui.plan.cuts)}
            <span class="cuts">· {seriesText(ui.plan.cuts)}</span>
          {/if}
        </label>
      </details>

      <div class="actions">
        <AutoVariant ph={ui.ph} off={empty || store.autoWaiting} note={sameText(store.autoSame, ui.ph.variant)} />
        <button class="ghost" onclick={close}>Отмена</button>
        <button class="primary" disabled={!ui.canBuild || store.autoWaiting} onclick={go}>
          {store.autoWaiting ? 'Пересобираю…' : ui.ph.again ? 'Пересобрать' : ui.ph.album ? ALBUM_BUILD : 'Собрать'}
        </button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;
  }
  input[type='range'] {
    width: 100%;
    margin: 0; /* поля по умолчанию (2 px) выпирали за край окна */
  }
  .after {
    margin: 0;
    min-height: 18px;
    font-size: 13px;
  }
  summary {
    cursor: pointer;
    color: var(--muted);
    font-size: 12px;
  }
  .more {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
  }
  .cuts {
    color: var(--muted);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
  }
</style>
