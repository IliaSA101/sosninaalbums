<script lang="ts">
  import type { LayoutScope, SpreadView, VigChoice } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { gridAct } from './gridact.svelte'
  import { hasSeam, isCover, unitOf } from '../../base/geom/sheet'
  import { gallery, pageSwitchable } from '../gallery/gallery.svelte'
  import { stillSpread } from '../gallery/variants'
  import Icon from '../../base/ui/Icon.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { align } from './align.svelte'
  import AlignBar from './AlignBar.svelte'
  import { formOf, isVignette } from '../vignette/vignette'
  import { isOwnTpl, resetVignette, tplLabel } from '../vignette/vigown'
  import { vigOwn } from '../vignette/vigown.svelte'
  import { vigChoices } from '../vignette/vigact'

  // show — курсор над разворотом или над полосой (следит Stage). Рамку (ширина разворота при 100 %,
  // всё окно при зуме) задаёт Stage; «Текст» — в ленте EditRail, оформление текста — в карточке InspectorText.
  // compact — холст увеличен: полоса шире разворота, группы страниц (25 %/75 %) не над страницами — прячем.
  let { spread, show, compact = false }: { spread: SpreadView; show: boolean; compact?: boolean } = $props()

  // Есть выделение на этом развороте («Полный») — вместо раскладок полоса выравнивания (спека 010, вопрос 10);
  // видна без наведения, при переносе прячется, открытую галерею раскладок не перекрывает.
  const aligning = $derived(
    !drag.active && prefs.can('align') && align.ids.length > 0 && spread.id === store.spread?.id && gallery.cur?.spread !== spread.id,
  )
  // Обложка (032): раскладок «Авто», шаблонов и копирования сетки у неё нет — только выравнивание выделенного.
  const cover = $derived(isCover(spread))
  // Виньетка (решение 80): стрелки и ⊞ листают шаблоны виньетки; «Авто», «Ручная» и копирование сетки тут не нужны.
  const vig = $derived(isVignette(spread))
  // Имя шаблона в строке — из того же списка Go, что карточка справа (переименование видно сразу). Запрос только
  // для своего шаблона: имена каталожных не меняются, там хватает tplName листа.
  let vigItems = $state<VigChoice[]>([])
  let vigReq = 0
  $effect(() => {
    const v = spread.vig
    void store.view.rev
    void vigOwn.rev
    const my = ++vigReq // до раннего выхода: старый ответ не должен перезаписать новое состояние
    if (!isVignette(spread) || !v || !isOwnTpl(v.tpl)) {
      vigItems = []
      return
    }
    vigChoices(spread.id, formOf(v))
      .then((list) => {
        if (my === vigReq) vigItems = list
      })
      .catch(() => {
        // Имя останется из tplName; ошибку Go покажет карточка справа
      })
  })
  const vigName = $derived(spread.vig ? tplLabel(vigItems, spread.vig.tpl, spread.vig.tplName) : '')
  // Без «Вариантов» (спека 011) листать и выбирать раскладки нечем: остаётся подпись «Раскладка: Авто».
  const unit = $derived(unitOf(store.view.settings))
  const variants = $derived(prefs.can('variants'))
  const pages = $derived(
    compact || aligning || cover || !variants || !hasSeam(spread)
      ? []
      : (['left', 'right'] as const).map((side, i) => ({ side, page: spread.pages?.[i] })).filter((p) => pageSwitchable(p.page)),
  )
  const visible = $derived(
    !drag.active && (cover ? aligning : !!spread.layout || spread.grid) && (show || gallery.cur?.spread === spread.id),
  )
  // Листать нечего: в ряду одна раскладка, и она уже стоит.
  const still = (variant: number, variants: number) => variant >= 0 && variants <= 1

  function open(scope: LayoutScope) {
    gallery.open(scope)
  }
  function isOpen(scope: LayoutScope) {
    return gallery.cur?.spread === spread.id && gallery.cur.scope === scope && gallery.cur.tab === 'layouts'
  }
  const gridsOpen = $derived(gallery.cur?.spread === spread.id && gallery.cur.tab === 'grids')
  const stop = (e: PointerEvent) => e.stopPropagation()

  // Группа исчезла под курсором (раскладка стала не постраничной) — mouseleave не придёт.
  $effect(() => {
    const h = store.hoverPage
    if (h && !pages.some((p) => p.side === h)) store.hoverPage = null
  })
</script>

<div class="bar layout-bar" class:visible data-guide="layout-bar" data-guide-reveal>
  {#each pages as { side, page } (side)}
    {@const who = side === 'left' ? 'левой' : 'правой'}
    <div
      class="group page"
      style:left={side === 'left' ? '25%' : '75%'}
      role="group"
      aria-label={side === 'left' ? 'Левая страница' : 'Правая страница'}
      onmouseenter={() => (store.hoverPage = side)}
      onmouseleave={() => (store.hoverPage = null)}
    >
      <button
        class="arrow"
        disabled={still(page!.variant, page!.variants)}
        onpointerdown={stop}
        onclick={() => gallery.step(side, -1)}
        title="Предыдущая раскладка страницы"
        aria-label="Предыдущая раскладка {who} страницы"><Icon name="chevron-left" /></button
      >
      <button
        class="grid"
        class:active={isOpen(side)}
        onpointerdown={stop}
        onclick={() => open(side)}
        title="Раскладки {who} страницы"
        aria-label="Раскладки {who} страницы"
        aria-pressed={isOpen(side)}><Icon name="grid" size={14} /></button
      >
      <button
        class="arrow"
        disabled={still(page!.variant, page!.variants)}
        onpointerdown={stop}
        onclick={() => gallery.step(side, 1)}
        title="Следующая раскладка страницы"
        aria-label="Следующая раскладка {who} страницы"><Icon name="chevron-right" /></button
      >
    </div>
  {/each}

  {#if aligning}
    <AlignBar />
  {:else if cover}
    <!-- без пунктов -->
  {:else if vig}
    <!-- Как у страницы: «‹ Виньетка: имя ▦ ›», у ручной — «‹ Виньетка: имя · Ручная · Вернуть авто › ▦» -->
    {@const manual = !!spread.vig?.manual}
    <div class="group spine" style:left="50%">
      {#if variants}
        <button
          class="arrow"
          onpointerdown={stop}
          onclick={() => gallery.step('spread', -1)}
          title="Предыдущий шаблон виньетки (Shift+пробел)"
          aria-label="Предыдущий шаблон виньетки"><Icon name="chevron-left" /></button
        >
      {/if}
      <span class="tag name" title="Параметры и шаблоны — в карточке справа">{vigName ? `Виньетка: ${vigName}` : 'Виньетка'}</span>
      {#if manual}
        <span class="tag" title="Места сдвинуты вручную и отличаются от шаблона">· Ручная ·</span>
        <button
          class="text auto"
          onpointerdown={stop}
          onclick={() => gridAct.vigGate(spread.id, 'reset', () => resetVignette(spread.id))}
          title="Вернуть авто: места снова по шаблону; фото и подписи остаются (один шаг отмены)">Вернуть авто</button
        >
      {/if}
      {#if variants}
        {#snippet vigGrid()}
          <button
            class="grid"
            class:active={isOpen('spread')}
            onpointerdown={stop}
            onclick={() => open('spread')}
            data-guide="layout-all"
            title="Все шаблоны виньетки"
            aria-label="Все шаблоны виньетки"
            aria-pressed={isOpen('spread')}><Icon name="grid" size={14} /></button
          >
        {/snippet}
        {#snippet vigNext()}
          <button
            class="arrow"
            onpointerdown={stop}
            onclick={() => gallery.step('spread', 1)}
            title="Следующий шаблон виньетки (пробел)"
            aria-label="Следующий шаблон виньетки"><Icon name="chevron-right" /></button
          >
        {/snippet}
        {#if manual}
          {@render vigNext()}{@render vigGrid()}
        {:else}
          {@render vigGrid()}{@render vigNext()}
        {/if}
      {/if}
    </div>
  {:else if spread.grid}
    <div class="group spine" style:left="50%">
      <button
        class="arrow"
        onpointerdown={stop}
        onclick={() => gridAct.backToAuto(spread.id)}
        title="Вернуть авто (места сбросятся)"
        aria-label="Предыдущая раскладка: вернуть авто"><Icon name="chevron-left" /></button
      >
      <span class="tag" title="Места заданы вручную: раскладка не перестраивается">Раскладка: Ручная ·</span>
      <button
        class="text auto"
        onpointerdown={stop}
        onclick={() => gridAct.backToAuto(spread.id)}
        title="Вернуть авто: фото разложатся автоматически, места сбросятся">Вернуть авто</button
      >
      <button
        class="arrow"
        onpointerdown={stop}
        onclick={() => gridAct.backToAuto(spread.id)}
        title="Вернуть авто (места сбросятся)"
        aria-label="Следующая раскладка: вернуть авто"><Icon name="chevron-right" /></button
      >
      {#if prefs.can('templates')}
        <button
          class="grid"
          class:active={gridsOpen}
          onpointerdown={stop}
          onclick={() => gallery.open('spread', 'grids')}
          title="Галерея шаблонов"
          data-guide="grid-gallery"
          aria-label="Галерея шаблонов"
          aria-pressed={gridsOpen}><Icon name="grid" size={14} /></button
        >
        <button
          class="text"
          data-guide="copy-grid"
          onpointerdown={stop}
          onclick={() => (gridAct.copyGrid = { spread: spread.id, from: 'spread' })}
          title="Поставить такую же раскладку на другие {unit.many}">Копировать раскладку…</button
        >
      {/if}
    </div>
  {:else}
    <div class="group spine" style:left="50%">
      {#if variants}
        <button
          class="arrow"
          disabled={stillSpread(spread)}
          onpointerdown={stop}
          onclick={() => gallery.step('spread', -1)}
          title="Предыдущая раскладка (Shift+пробел)"
          aria-label="Предыдущая раскладка разворота"><Icon name="chevron-left" /></button
        >
      {/if}
      <span class="tag" title="Раскладка подбирается автоматически">Раскладка: Авто</span>
      {#if variants}
        <button
          class="grid"
          class:active={isOpen('spread')}
          onpointerdown={stop}
          onclick={() => open('spread')}
          data-guide="layout-all"
          title="Все раскладки разворота"
          aria-label="Все раскладки разворота"
          aria-pressed={isOpen('spread')}><Icon name="grid" size={14} /></button
        >
        <button
          class="arrow"
          disabled={stillSpread(spread)}
          onpointerdown={stop}
          onclick={() => gallery.step('spread', 1)}
          title="Следующая раскладка (пробел)"
          aria-label="Следующая раскладка разворота"><Icon name="chevron-right" /></button
        >
      {/if}
      {#if prefs.can('manual')}
        <button
          class="pin"
          onpointerdown={stop}
          onclick={() => gridAct.setGrid(true)}
          title="Сделать ручной: раскладка перестаёт меняться сама, места можно править"
          aria-label="Сделать ручной"><Icon name="pin" size={14} /></button
        >
      {/if}
    </div>
  {/if}
</div>

<style>
  .bar {
    position: relative;
    flex: none;
    height: 32px; /* = BAR в inspector/panels.ts */
  }
  .group {
    position: absolute;
    top: 0;
    bottom: 0;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 6px;
    opacity: 0;
    visibility: hidden;
    /* visibility при появлении включается сразу: при переходе 0.15 с в первом кадре элемент
       ещё hidden, и быстрый клик сразу после наведения проваливается сквозь кнопку. */
    transition:
      opacity 0.15s,
      visibility 0s linear 0.15s;
  }
  .visible .group,
  .bar:global([data-spot]) .group {
    opacity: 1;
    visibility: visible;
    transition:
      opacity 0.15s,
      visibility 0s;
  }
  button {
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .tag {
    padding: 0 0 0 6px;
    white-space: nowrap;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
  }
  /* Имя шаблона длинное — обрезается, полное в подсказке карточки справа */
  .tag.name {
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .arrow {
    width: 22px;
  }
  .pin {
    width: 26px;
  }
  .text {
    padding: 0 7px;
    font-size: 12px;
    white-space: nowrap;
  }
  .text.auto {
    padding: 0 6px 0 4px;
    color: var(--accent);
    font-weight: 600;
  }
  .grid {
    width: 28px;
  }
  .grid.active {
    background: var(--accent);
    color: #1e1e1e;
  }
  .grid.active:hover:not(:disabled) {
    background: #ffe53a;
  }
  /* Стрелки страницы появляются только над её группой; место держат, чтобы ⊞ не сдвигался. */
  .page .arrow {
    visibility: hidden;
  }
  .visible .page:hover .arrow {
    visibility: visible;
  }
</style>
