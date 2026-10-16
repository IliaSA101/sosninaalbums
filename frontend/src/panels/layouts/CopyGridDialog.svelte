<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { gridAct } from './gridact.svelte'
  import { parseRanges } from './ranges'
  import { plural } from '../../base/words/plural'
  import { isSheet, unitOf } from '../../base/geom/sheet'
  import { errText } from '../../base/words/errtext'
  import {
    copyTargets,
    loadCopyOpts,
    loadDecorOpts,
    planQuestion,
    saveCopyOpts,
    saveDecorOpts,
    touched,
    type CopySide,
    type CopyWhere,
  } from '../gallery/gridgallery'
  import { planFor, runDuplicate, runFor, type CopyCall } from './gridactions'
  import type { LayoutScope } from '../../base/core/types'
  import Modal from '../../base/ui/Modal.svelte'

  // «Копировать сетку…» (спека 007, design §6): откуда, куда, флажки; перед копированием — PlanCopyGrid
  // и вопрос, если копирование что-то разрушит. Окно открыто, пока gridAct.copyGrid не null.
  // «Копировать оформление…» (049 Ф2) — то же окно (mode 'decor'): без новых разворотов, флажок «С надписями».
  const cg = $derived(gridAct.copyGrid)
  const decor = $derived(cg?.mode === 'decor')
  const spreads = $derived(store.view.spreads)
  const sheet = $derived(isSheet(store.view.settings))
  const unit = $derived(unitOf(store.view.settings))

  let from = $state<LayoutScope>('spread')
  let where = $state<CopyWhere>('spreads')
  let side = $state<CopySide>('spread')
  let text = $state('')
  let count = $state(1)
  let design = $state(true)
  let texts = $state(false) // «С надписями» (только оформление)
  let mirror = $state(false)
  let ask = $state<string | null>(null) // вопрос перед копированием
  let busy = $state(false)

  // Каждое открытие — заново: источник из LayoutBar, флажки из localStorage.
  $effect(() => {
    const c = cg
    if (!c) return
    untrack(() => {
      from = c.from
      where = c.from === 'spread' ? 'spreads' : 'next'
      side = 'spread'
      text = ''
      count = 1
      ask = null
      busy = false
      if (c.mode === 'decor') ({ texts, mirror } = loadDecorOpts())
      else ({ design, mirror } = loadCopyOpts())
    })
  })

  const page = $derived(from !== 'spread')
  // Смена источника: цели «соседняя страница» и «новые развороты» подходят не всем.
  function setFrom(f: LayoutScope) {
    from = f
    if (f === 'spread' && where === 'next') where = 'spreads'
    if (f !== 'spread' && where === 'new') where = 'next'
  }

  const ranges = $derived(where === 'spreads' ? parseRanges(text, spreads.length) : null)
  const ids = $derived(ranges && !ranges.error ? ranges.nums.map((n) => spreads[n - 1].id) : [])
  const targets = $derived(cg ? copyTargets({ src: cg.spread, from, where, side, spreadIds: ids, allIds: spreads.map((s) => s.id) }) : [])
  const n = $derived(cg ? touched(targets, cg.spread, from) : 0)
  const countOk = $derived(Number.isInteger(count) && count >= 1 && count <= 100)
  const can = $derived(!busy && (where === 'new' ? countOk : n > 0))

  function close() {
    gridAct.copyGrid = null
  }
  function remember() {
    if (decor) saveDecorOpts({ texts, mirror })
    else saveCopyOpts({ design, mirror })
  }

  // Зеркало осмысленно только со страницы; «С текстом и фоном» и «С надписями» — флажки разных режимов
  const call = (src: string): CopyCall => ({ decor, src, from, to: targets, design, texts, mirror: page && mirror })

  async function exec() {
    const c = cg
    if (!c) return
    busy = true
    const ok = where === 'new' ? await runDuplicate(c.spread, count, design) : await runFor(call(c.spread), n)
    busy = false
    if (ok) close()
  }

  async function go() {
    const c = cg
    if (!c || !can) return
    if (where === 'new') return exec()
    busy = true
    try {
      const q = planQuestion(await planFor(call(c.spread)))
      busy = false
      if (q) ask = q
      else await exec()
    } catch (e) {
      busy = false
      store.notify(errText(e), 'error')
    }
  }
  const enter = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      void go()
    }
  }
  // У листа виньетки страниц нет: источник один — весь лист.
  const froms = $derived<{ scope: LayoutScope; label: string }[]>(
    sheet
      ? [{ scope: 'spread', label: 'Лист' }]
      : [
          { scope: 'left', label: 'Левая страница' },
          { scope: 'right', label: 'Правая страница' },
          { scope: 'spread', label: 'Разворот' },
        ],
  )
  const sides: { v: CopySide; label: string }[] = [
    { v: 'left', label: 'левые' },
    { v: 'right', label: 'правые' },
    { v: 'spread', label: 'обе' },
  ]
</script>

{#if cg}
  {@const what = decor ? 'Копировать оформление' : 'Копировать раскладку'}
  <Modal title={ask ? `${what}?` : what} onclose={close} wide>
    {#if ask}
      <p class="ask">{ask}</p>
      <div class="actions">
        <button class="ghost" onclick={() => (ask = null)}>Назад</button>
        <button class="primary" disabled={busy} onclick={exec}>Копировать</button>
      </div>
    {:else}
      <div class="group" role="radiogroup" aria-label="Откуда">
        <span class="cap">Откуда</span>
        {#each froms as f (f.scope)}
          <label><input type="radio" name="cg-from" checked={from === f.scope} onchange={() => setFrom(f.scope)} />{f.label}</label>
        {/each}
      </div>

      <div class="group col" role="radiogroup" aria-label="Куда">
        <span class="cap">Куда</span>
        {#if page}
          <label
            ><input type="radio" name="cg-where" value="next" bind:group={where} />на соседнюю страницу ({from === 'left'
              ? 'правую'
              : 'левую'})</label
          >
        {/if}
        <label class="inline"
          ><input type="radio" name="cg-where" value="spreads" bind:group={where} />на {sheet ? 'листы' : 'развороты'}:
          <input
            type="text"
            class="ranges"
            class:bad={!!ranges?.error && text.trim() !== ''}
            placeholder="3, 5-8"
            aria-label="Номера {sheet ? 'листов' : 'разворотов'}"
            bind:value={text}
            onfocus={() => (where = 'spreads')}
            onkeydown={enter}
          /></label
        >
        {#if ranges?.error && text.trim() !== ''}<span class="err">{ranges.error}</span>{/if}
        <label
          ><input type="radio" name="cg-where" value="all" bind:group={where} />на все {sheet ? 'листы' : 'развороты'} ({spreads.length})</label
        >
        {#if !page && !decor}
          <label class="inline"
            ><input type="radio" name="cg-where" value="new" bind:group={where} />создать новые {sheet ? 'листы' : 'развороты'} после этого:
            <input
              type="number"
              class="num"
              min="1"
              max="100"
              aria-label="Сколько новых {sheet ? 'листов' : 'разворотов'}"
              bind:value={count}
              onfocus={() => (where = 'new')}
              onkeydown={enter}
            /></label
          >
        {/if}
      </div>

      {#if page && (where === 'spreads' || where === 'all')}
        <div class="group" role="radiogroup" aria-label="На какие страницы">
          <span class="cap">Страницы</span>
          {#each sides as sd (sd.v)}
            <label><input type="radio" name="cg-side" checked={side === sd.v} onchange={() => (side = sd.v)} />{sd.label}</label>
          {/each}
        </div>
      {/if}

      <div class="group col">
        {#if decor}
          <label
            ><input type="checkbox" bind:checked={texts} onchange={remember} />С надписями
            <span class="hint">(тексты на листе)</span></label
          >
        {:else}
          <label
            ><input type="checkbox" bind:checked={design} onchange={remember} />С текстом и фоном
            <span class="hint">(подпись, цвет страницы)</span></label
          >
        {/if}
        {#if page}
          <label
            ><input type="checkbox" bind:checked={mirror} onchange={remember} />Зеркально на другой стороне
            <span class="hint">(левая ↔ правая)</span></label
          >
        {/if}
      </div>

      {#if where !== 'new' && n > 0}
        <p class="hint">Затронет {n} {plural(n, unit.plural)}.</p>
      {/if}
      <div class="actions">
        <button class="ghost" onclick={close}>Отмена</button>
        <button class="primary" disabled={!can} onclick={go}>{where === 'new' ? 'Создать' : 'Копировать'}</button>
      </div>
    {/if}
  </Modal>
{/if}

<style>
  .group {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px 14px;
    margin-bottom: 12px;
  }
  .group.col {
    flex-direction: column;
    align-items: flex-start;
  }
  .cap {
    flex: none;
    width: 64px;
    color: var(--muted);
    font-size: 12px;
  }
  .group.col > .cap {
    width: auto;
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
  .inline {
    flex-wrap: wrap;
  }
  .ranges,
  .num {
    height: 26px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .ranges {
    width: 140px;
  }
  .num {
    width: 64px;
  }
  .ranges.bad {
    border-color: #c0392b;
  }
  .err {
    color: #e5645a;
    font-size: 12px;
    padding-left: 22px;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    margin: 0;
  }
  .ask {
    margin: 4px 0 6px;
    line-height: 1.45;
    font-size: 14px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
