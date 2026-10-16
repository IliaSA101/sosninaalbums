<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import * as Api from '../../../wailsjs/go/app/App'
  import { model } from '../../../wailsjs/go/models'
  import type { PlanFn } from './autobuild.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import Modal from '../../base/ui/Modal.svelte'
  import { AutoDialog } from './autobuild.svelte'
  import {
    MAX_PER,
    VARIETY_OPTIONS,
    albumAfter,
    anchorField,
    otherField,
    perText,
    planHint,
    rangeText,
    sourceLine,
    unitWord,
  } from './autobuild'
  import { unitOf } from '../../base/geom/sheet'

  // Окно «Автосборка» (027 U2, 034): сколько разворотов собрать — числом N или средним «фото на разворот»
  // (поля связаны). Открыто, пока store.autoDraft; фото выбраны заранее (store.openAuto).
  let ui = $state<AutoDialog | null>(null)
  // Разнообразие на план не влияет (группы те же), но поле контракта заполняем запомненным.
  const autoPlan: PlanFn = (ids, spreads, maxPer) =>
    Api.AutoPlan([...ids], model.AutoOpts.createFrom({ spreads, maxPer, variety: modes.autoVariety }))

  // Каждое открытие — новое состояние: N, среднее и «не больше» берутся из памяти окна, ответ Go запрашивается заново.
  $effect(() => {
    const d = store.autoDraft
    untrack(() => {
      ui = d
        ? new AutoDialog(d.ids, autoPlan, {
            spreads: modes.autoSpreads,
            maxPer: modes.autoMaxPer,
            per: modes.autoPer,
            by: modes.autoBy,
            variety: modes.autoVariety,
          })
        : null
      typing = null
      ui?.start()
    })
  })

  const close = () => (store.autoDraft = null)

  async function go() {
    if (!ui) return
    // Запоминаем и собираем то, что подтвердил Go: N после ответа, а не то, что успели набрать.
    await ui.submit((n, m, per, by, variety) => {
      modes.rememberAuto(n, m, per, by, variety)
      return store.autoBuild(n, m, variety)
    })
  }

  // Пока в поле печатают, в нём остаётся набранное (draft); иначе — фактическое значение, даже при фокусе:
  // ответ Go мог поправить N после открытия, а поле с автофокусом не должно застрять на старом числе.
  // Якорь меняет только действие пользователя (щелчок по карточке, ⇄, ввод); простой фокус — нет:
  // иначе автофокус при открытии и возврат фокуса после окна поверх затирали бы запомненное «среднее».
  let typing = $state<'n' | 'per' | null>(null)
  let draft = $state('')
  const enter = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select()
  // ⇄ переводит ввод во второе поле: оно становится активным, значение пересчитается от первого.
  const swap = () => {
    if (!ui) return
    const to = otherField(ui.anchor) // до activate: после него якорь уже другой
    ui.activate(ui.anchor === 'spreads' ? 'per' : 'spreads')
    document.getElementById(to)?.focus()
  }
  // Щелчок по карточке (в том числе вне поля) делает её поле якорем; ⇄ обрабатывает swap.
  function pick(e: Event) {
    const f = (e.target as Element).closest('label.tile')?.getAttribute('for')
    if (f) ui?.activate(f === 'auto-n' ? 'spreads' : 'per')
  }
  // Поле якоря получает первый фокус (data-autofocus); у второго атрибута нет вовсе — false Svelte пишет строкой.
  const first = (id: 'auto-n' | 'auto-per') => (ui && anchorField(ui.anchor) === id ? '' : undefined)
  const shownPer = () => (ui ? perText(ui.photos, ui.n) : '')

  // Стрелки и PgUp/PgDn: направление и шаг (PgUp/PgDn — 5).
  const arrow = (key: string): [number, number] => [
    key === 'PageUp' || key === 'ArrowUp' ? 1 : key === 'PageDown' || key === 'ArrowDown' ? -1 : 0,
    key.startsWith('Page') ? 5 : 1,
  ]

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

  // Enter собирает отовсюду, кроме кнопок и «Дополнительно» (там он делает своё).
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
  <Modal title="Автосборка" onclose={close} width={540}>
    <div class="body" role="presentation" onkeydown={key}>
      <p class="src">{sourceLine({ ...d, count: d.ids.length }, ui.plan?.photos ?? null)}</p>

      <div class="tiles">
        <div class="pair" role="presentation" onpointerdown={pick} onclick={pick}>
          <label class="tile" for="auto-n" class:anchor={ui.anchor === 'spreads'}>
            <span class="row">
              <input
                id="auto-n"
                class="big"
                type="number"
                min={low}
                max={top}
                step="1"
                value={typing === 'n' ? draft : ui.n}
                disabled={empty}
                data-autofocus={first('auto-n')}
                aria-label={unitWord(ui.n, unit)}
                onfocus={enter}
                onblur={() => (typing = null)}
                oninput={(e) => {
                  typing = 'n'
                  draft = e.currentTarget.value
                  if (draft !== '') ui?.setSpreads(e.currentTarget.valueAsNumber)
                }}
              />
              <span class="cap">{unitWord(ui.n, unit)}</span>
            </span>
            <span class="sub">{unit.one === 'лист' ? '' : `${2 * ui.n} стр.`}</span>
            <span class="rem">{ui.anchor === 'spreads' ? 'Запомню для следующей сборки' : ''}</span>
          </label>

          <button class="link" type="button" disabled={empty} title="Задать другим полем" aria-label="Переключить поле ввода" onclick={swap}
            >⇄</button
          >

          <label
            class="tile"
            for="auto-per"
            class:anchor={ui.anchor === 'per'}
            title={ui.plan && !ui.pending ? planHint(ui.plan, unit) : ''}
          >
            <span class="row">
              <input
                id="auto-per"
                class="big"
                type="text"
                inputmode="decimal"
                autocomplete="off"
                data-autofocus={first('auto-per')}
                value={empty ? '' : typing === 'per' ? draft : shownPer()}
                disabled={empty}
                aria-label="Фото на {unit.one}"
                onfocus={enter}
                onblur={() => (typing = null)}
                oninput={(e) => {
                  typing = 'per'
                  draft = e.currentTarget.value
                  ui?.setPer(Number(draft.replace(',', '.')))
                }}
              />
              <span class="cap">фото на {unit.one}</span>
            </span>
            <span class="sub" role="status">
              {#if ui.error}
                <span class="err">{ui.error}</span>
              {:else if empty}
                Нет фото для сборки
              {:else if ui.pending || !ui.plan}
                Считаю…
              {:else}
                {rangeText(ui.plan)}
              {/if}
            </span>
            <span class="rem">{ui.anchor === 'per' ? 'Запомню для следующей сборки' : ''}</span>
          </label>
        </div>
      </div>

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
      <p class="after">{albumAfter(store.view.spreads.length, ui.n, unit, ui.plan?.reuse ?? 0)}</p>

      <details>
        <summary>Дополнительно</summary>
        <label class="more">
          Не больше
          <input
            type="number"
            min="1"
            max={MAX_PER}
            step="1"
            value={ui.maxPer}
            aria-label="Не больше фото на {unit.one}"
            oninput={(e) => e.currentTarget.value !== '' && ui?.setMaxPer(e.currentTarget.valueAsNumber)}
          />
          фото на {unit.one}
        </label>
        <!-- 041: выбор из трёх — сегменты, как «Разворот / Левая / Правая» в галерее -->
        <div class="more" role="group" aria-label="Разнообразие раскладок">
          Разнообразие раскладок
          <div class="seg">
            {#each VARIETY_OPTIONS as o (o.v)}
              <button type="button" class:on={ui.variety === o.v} aria-pressed={ui.variety === o.v} onclick={() => ui?.setVariety(o.v)}
                >{o.label}</button
              >
            {/each}
          </div>
        </div>
        <p class="hint" role="status">{VARIETY_OPTIONS.find((o) => o.v === ui?.variety)?.hint}</p>
      </details>

      <div class="actions">
        <button class="ghost" onclick={close}>Отмена</button>
        <button class="primary" disabled={!ui.canBuild} onclick={go}>Собрать</button>
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
  .src {
    margin: 0;
    color: var(--muted);
    font-size: 12px;
  }
  /* Поля встают в ряд, пока хватает места; иначе друг под другом (без прокрутки и обрезки) */
  .tiles {
    container-type: inline-size;
  }
  .pair {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    align-items: center;
    gap: 8px;
  }
  @container (max-width: 440px) {
    .pair {
      grid-template-columns: minmax(0, 1fr);
    }
    .link {
      justify-self: center;
      transform: rotate(90deg);
    }
  }
  .tile {
    display: block;
    cursor: pointer; /* вся карточка — цель щелчка: фокус уходит в её поле */
    min-width: 0;
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--panel2);
  }
  .tile:hover {
    border-color: var(--muted);
  }
  .tile.anchor,
  .tile:focus-within {
    border-color: var(--accent);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 8px;
  }
  .cap {
    font-size: 13px;
  }
  .big {
    width: 76px;
    font: inherit;
    font-size: 20px;
    font-weight: 600;
    text-align: center;
    height: 36px;
    box-sizing: border-box;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    padding: 0 4px;
  }
  .big:focus {
    outline: 1px solid var(--accent);
  }
  .sub {
    display: block;
    margin-top: 4px;
    min-height: 18px;
    color: var(--muted);
    font-size: 12px;
  }
  .rem {
    display: block;
    min-height: 14px;
    color: var(--muted);
    font-size: 11px;
  }
  .link {
    padding: 2px 6px;
    background: transparent;
    border: 0;
    color: var(--muted);
    font-size: 18px;
    cursor: pointer;
  }
  .link:hover:not(:disabled) {
    color: var(--text);
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
  .err {
    color: #e5645a;
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
  .more input {
    width: 64px;
  }
  /* Сегменты — как в галерее страницы (GridGallery) */
  .seg {
    display: flex;
    gap: 1px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .seg button {
    height: 24px;
    min-width: 28px;
    padding: 0 8px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .hint {
    margin: 4px 0 0;
    min-height: 16px;
    color: var(--muted);
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 8px;
  }
</style>
