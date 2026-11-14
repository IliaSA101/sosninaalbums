<script lang="ts">
  import { tick, untrack } from 'svelte'
  import { isSheet, unitOf } from '../../base/geom/sheet'
  import { lookUrl } from '../../base/core/media'
  import { primaryPress } from '../../base/core/platform'
  import { store } from '../../base/core/store.svelte'
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { tilePx } from '../../panels/decor/decor'
  import { BUILTIN_PACK } from '../../panels/decorbrowse/browse'
  import PeekShell from '../../panels/decorbrowse/PeekShell.svelte'
  import { PEEK_HINT, PEEK_PX_MAX, lookCaption, lookKey, lookRatio, stepLabel, stepWhy, tileRatio, tileStep } from './looks'
  import { looks } from './looks.svelte'
  import LookSteps from './LookSteps.svelte'

  // Крупный просмотр макета (163 U3), сетка и слова — как у DecorPeek: тёмная подложка, лист шага, ‹ ›, лента шагов,
  // строка «подпись · кнопки · ✕», клавиши. ← → и ‹ › листают шаги макета (163-3), Enter и «Применить…» открывают окно
  // «Применить» поверх (163-4 а), «Поставить на разворот N» — вопрос 067. Оболочка (PeekShell) держит окно в modalStack.
  let { px: thumbPx }: { px: number } = $props() // размер плитки: из него адрес миниатюры, что видна, пока грузится крупная

  const CHROME = 120 // высота под листом: лента шагов, строка кнопок, клавиши

  const it = $derived(looks.peek.id !== '' && looks.peek.id === looks.sel ? looks.item : undefined)
  const chips = $derived(looks.chips)
  const at = $derived(chips.findIndex((c) => c.i === looks.step))
  const stepInfo = $derived(looks.step > 0 ? looks.info?.steps[looks.step - 1] : undefined)
  const cover = $derived(store.cover?.sheet)
  const r = $derived(lookRatio(store.view.settings, stepInfo?.pages ?? 2, looks.step === 0 ? cover : null))
  const key = $derived(lookKey(looks.sel))
  // Лист вписан в 94 % окна: от его ширины считается запрос (ступенями tilePx — разные размеры окна не плодят картинки)
  const big = $derived.by(() => {
    const w = Math.min(window.innerWidth * 0.94, (window.innerHeight * 0.94 - CHROME) * r)
    return lookUrl(key, { i: looks.step, px: Math.min(PEEK_PX_MAX, tilePx(w, screenDpr.value)), r, ph: looks.ph })
  })
  // Миниатюра плитки — только у шага, который показывает плитка
  const thumb = $derived.by(() => {
    if (!it) return ''
    const i = tileStep(it)
    return i === looks.step
      ? lookUrl(key, { i, px: thumbPx, r: tileRatio(store.view.settings, it, cover), front: i === 0, ph: looks.ph })
      : ''
  })
  let ready = $state(false) // крупная загрузилась: миниатюра под ней убирается
  let failed = $state(false) // не загрузилась: остаётся миниатюра, без тоста (как DecorPeek)
  $effect.pre(() => {
    void big // новый адрес — новое ожидание
    ready = false
    failed = false
  })

  // «Показать весь набор» — у макета набора, если в наборе их ≥ 2 (одна плитка — пустой переход)
  const setId = $derived.by(() => {
    const pid = it?.packId
    if (!pid || pid === BUILTIN_PACK) return ''
    return (looks.cat?.items.filter((i) => i.packId === pid).length ?? 0) >= 2 ? pid : ''
  })
  function showSet(leave: () => Promise<void>) {
    looks.showPack(setId)
    void leave()
  }

  // «Поставить на разворот N» (131 сц. 7): показанный шаг на текущий лист; серая, если шаг не для него
  const target = $derived(looks.stepTarget)
  const why = $derived(target ? stepWhy(looks.step, target.cover, !!store.cover, !!looks.info?.cover) : '')
  const sheetDoc = $derived(isSheet(store.view.settings)) // у проекта-листа 031 макетов альбома нет — кнопки тоже

  let applyBtn = $state<HTMLButtonElement>()
  let stepBtn = $state<HTMLButtonElement>()
  // Окно «Применить» или вопрос поверх закрылись, просмотр остался: фокус возвращается кнопке, что их открыла
  let prev = ''
  $effect(() => {
    const k = looks.ask?.kind ?? ''
    untrack(() => {
      const btn = prev === 'form' ? applyBtn : prev === 'step' ? stepBtn : undefined
      if (btn && k === '' && it) void tick().then(() => btn.focus())
    })
    prev = k
  })

  const plain = (e: KeyboardEvent) => !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey
  // Своя клавиша окна: ← → — шаги, Enter — «Применить…» (Esc и поля ввода — в PeekShell). На кнопке Enter — её щелчок;
  // автоповтор Enter и Enter при постановке (busy) ничего не открывают.
  function action(e: KeyboardEvent): (() => void) | null {
    if ((e.key === 'ArrowLeft' || e.key === 'ArrowRight') && plain(e)) return () => looks.go(e.key === 'ArrowRight' ? 1 : -1)
    if (e.key === 'Enter') return e.target instanceof HTMLButtonElement ? null : () => !e.repeat && looks.askForm()
    return null
  }
</script>

{#if it}
  <PeekShell tileId={it.id} onclose={() => looks.peek.close()} onkey={action} label="Крупный просмотр макета">
    {#snippet view(leave)}
      <div class="view" role="presentation" onpointerdown={(e) => primaryPress(e) && e.target === e.currentTarget && void leave()}>
        <button class="nav" disabled={at <= 0} aria-label="Предыдущий шаг" title="Предыдущий шаг" onclick={() => looks.go(-1)}>‹</button>
        <div class="sheet">
          {#if thumb && (!ready || failed)}<img src={thumb} alt="" draggable="false" />{/if}
          {#if !failed}
            <img
              src={big || undefined}
              alt={it.name}
              draggable="false"
              onload={() => (ready = true)}
              onerror={() => (failed = true)}
              {@attach cancelOnUnmount}
            />
          {/if}
        </div>
        <button
          class="nav"
          disabled={at < 0 || at >= chips.length - 1}
          aria-label="Следующий шаг"
          title="Следующий шаг"
          onclick={() => looks.go(1)}>›</button
        >
      </div>
    {/snippet}
    {#snippet strip()}<LookSteps />{/snippet}
    {#snippet caption()}{lookCaption(it)}{/snippet}
    {#snippet actions(leave)}
      {#if setId}
        <button class="ghost" title="Показать весь набор «{it.pack}»" onclick={() => showSet(leave)}>Показать весь набор</button>
      {/if}
      {#if target && !sheetDoc}
        <button
          type="button"
          class="ghost"
          bind:this={stepBtn}
          disabled={!looks.stepOk}
          title={why || undefined}
          onclick={() => looks.askStep()}>{stepLabel(target.cover, target.no, unitOf(store.view.settings).one)}</button
        >
      {/if}
      <button class="primary" bind:this={applyBtn} disabled={!looks.ready || looks.busy} onclick={() => looks.askForm()}>Применить…</button>
    {/snippet}
    {#snippet hint()}{PEEK_HINT}{/snippet}
  </PeekShell>
{/if}

<style>
  .view {
    min-height: 0;
    display: flex;
    align-items: stretch;
    gap: 8px;
  }
  /* Лист вписан в оставшееся место; картинка и обёртка не ловят щелчок — пустое место вокруг листа закрывает просмотр */
  .sheet {
    flex: 1;
    min-width: 0;
    position: relative;
    pointer-events: none;
  }
  .sheet img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .nav {
    flex: none;
    align-self: center;
    width: 40px;
    height: 72px;
    padding: 0;
    font-size: 26px;
    line-height: 1;
    background: rgba(255, 255, 255, 0.08);
    color: var(--text);
  }
</style>
