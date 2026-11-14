<script lang="ts">
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import { store } from '../../base/core/store.svelte'
  import { isSheet } from '../../base/geom/sheet'
  import { edit } from '../text/edit.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { panels } from './panelmode.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import { BAR, BTN, COL_GAP, COL_W, HEAD, ICON, M, SEP, railColumns, railLabeled } from './panels'
  import type { FeatureId } from '../../base/prefs/levels'
  import { modes } from '../../base/prefs/view.svelte'
  import { bgmode } from '../backdrop/bgmode.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon, { type IconName } from '../../base/ui/Icon.svelte'
  import MirrorMenu from '../../panels/layouts/MirrorMenu.svelte'
  import PlaceAddMenu from '../places/PlaceAddMenu.svelte'
  import { decor } from '../../panels/decor/decor.svelte'

  // Лента действий слева на холсте (спека 009, п. 2): две группы, одинаковые во всех режимах, —
  // «Добавить» (Текст, Место) и «Раскладка» (Разделить ↔ ↕, Отразить). Не влезли по высоте — новый
  // столбец (группа целиком), без прокрутки. Лента в один столбец и запас слева позволяет
  // (panels.railLabeled) — справа от значка короткая подпись `short`.
  interface Item {
    icon: IconName | 'mirror-menu'
    label: string
    short: string
    feature: FeatureId // функция уровня: без неё пункта нет (спека 011)
    off?: boolean // неактивна: подсказка `label` объясняет почему
    run?: () => void
  }
  interface Group {
    title: string
    items: Item[]
  }

  // «Стикер»: окно с плитками; сам стикер добавит щелчок по плитке (decor.pick).
  function openStickers() {
    const s = store.spread
    if (s) decor.show('sticker', s.id, 'spread')
  }

  // Разделять есть что, только когда выделено место (или фото в нём).
  const noPlace = $derived(placesel.selected.length === 0)
  const splitOff = 'Выделите место'
  // В виньетке страниц нет — только лист (031).
  const page = $derived(isSheet(store.view.settings) ? 'лист' : 'страницу')
  const full = $derived<Group[]>([
    {
      title: 'Добавить',
      items: [
        { icon: 'text', label: `Текст на ${page}`, short: 'Текст', feature: 'text', run: () => edit.addText() },
        { icon: 'place-add', label: `Новое место на ${page}`, short: 'Место', feature: 'manual' }, // меню «Место ▾» — PlaceAddMenu (серая кнопка в режиме фона — обычная)
        // 048: окно «Стикеры»; щелчок по плитке ставит стикер в центр страницы под курсором
        { icon: 'sticker', label: `Стикер на ${page}`, short: 'Стикер', feature: 'decor', run: openStickers },
      ],
    },
    {
      title: 'Раскладка',
      items: [
        {
          icon: 'split-h',
          label: noPlace ? splitOff : 'Разделить место рядом',
          short: 'Разделить ↔',
          feature: 'manual',
          off: noPlace,
          run: () => placesel.arrange('splitH'),
        },
        {
          icon: 'split-v',
          label: noPlace ? splitOff : 'Разделить место вниз',
          short: 'Разделить ↕',
          feature: 'manual',
          off: noPlace,
          run: () => placesel.arrange('splitV'),
        },
        { icon: 'mirror-menu', label: 'Отразить раскладку', short: 'Отразить', feature: 'templates' },
      ],
    },
  ])
  // Пункты по функциям «Своего»; пустая группа не рисуется (design §2.2, §2.3).
  // У обложки (032) нет быстрых раскладок и шаблонов: «Отразить раскладку» там скрыто.
  const all = $derived(
    full
      .map((g) => ({ ...g, items: g.items.filter((i) => prefs.can(i.feature) && !(store.onCover && i.icon === 'mirror-menu')) }))
      .filter((g) => g.items.length > 0),
  )
  // Режим фона: лента неактивна целиком, подсказка объясняет почему.
  const bgOff = 'Выйдите из режима фона'
  const groups = $derived(bgmode.cur ? all.map((g) => ({ ...g, items: g.items.map((i) => ({ ...i, label: bgOff, off: true })) })) : all)
  // Столбцы по высоте окна холста: индексы групп.
  const cols = $derived(
    railColumns(
      groups.map((g) => g.items.length),
      canvas.vp.h,
    ),
  )
  const wide = $derived(
    railLabeled(canvas.vp.h, modes.details, canvas.vp.pad ?? 0, Math.max(0, ...groups.flatMap((g) => g.items.map((i) => i.short.length)))),
  )
  // Кнопки не забирают фокус у редактора текста: курсор остаётся в нём.
  const keepFocus = (e: MouseEvent) => e.preventDefault()
</script>

{#snippet rbtn(it: Item)}
  <button
    class="rb"
    class:off={it.off}
    aria-disabled={it.off ? 'true' : undefined}
    aria-label={it.label}
    onmousedown={keepFocus}
    onpointerdown={(e) => e.stopPropagation()}
    use:tipped={{ text: it.label, side: 'right' }}
    onclick={() => {
      tip.hide()
      if (!it.off) it.run?.()
    }}
    ><Icon name={it.icon === 'mirror-menu' ? 'mirror-layout' : it.icon} size={ICON} />{#if wide}<span class="rl">{it.short}</span
      >{/if}</button
  >
{/snippet}

<!-- Плавающие панели — уровень «Полный» (вопрос 11); ниже карточка живёт в панели деталей -->
{#if panels.mode && prefs.can('panels') && groups.length > 0}
  <div
    class="rail"
    data-mini-edge="left"
    class:wide
    style:left="{M}px"
    style:top="{BAR + M}px"
    style:--col-w="{COL_W}px"
    style:--col-gap="{COL_GAP}px"
    style:--btn="{BTN}px"
    style:--head="{HEAD}px"
    style:--sep="{SEP}px"
    role="toolbar"
    aria-orientation="vertical"
    aria-label="Действия"
    data-chrome
    data-text-ui
    data-guide="rail"
  >
    {#each cols as col, c (c)}
      <div class="col">
        {#each col as gi, k (gi)}
          {@const g = groups[gi]}
          <div class="group" class:next={k > 0} role="group" aria-label={g.title}>
            <div class="cap">{g.title}</div>
            {#each g.items as it (it.icon)}
              {#if it.icon === 'mirror-menu' && !it.off}
                <MirrorMenu side="right" label={wide ? it.short : ''} onpick={(scope) => gridAct.mirrorGrid(scope)} />
              {:else if it.icon === 'text'}
                <div data-guide="rail-text">{@render rbtn(it)}</div>
              {:else if it.icon === 'place-add' && !it.off}
                <div data-guide="rail-place">
                  <PlaceAddMenu title={it.label} label={wide ? it.short : ''} onpick={(shape) => placesel.addPlace(shape)} />
                </div>
              {:else if it.icon === 'place-add'}
                <div data-guide="rail-place">{@render rbtn(it)}</div>
              {:else if it.icon === 'split-h' || it.icon === 'split-v'}
                <div data-guide="rail-split">{@render rbtn(it)}</div>
              {:else}
                {@render rbtn(it)}
              {/if}
            {/each}
          </div>
        {/each}
      </div>
    {/each}
  </div>
{/if}

<style>
  .rail {
    position: absolute;
    z-index: 20;
    display: flex;
    gap: var(--col-gap);
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.45);
    cursor: default;
  }
  .col {
    width: var(--col-w);
    display: flex;
    flex-direction: column;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  /* Обёртки якорей справки (data-guide) тянут кнопку на ширину столбца: иначе в режиме с подписями
     кнопка сжимается по содержимому и подсветка при наведении короче, чем у соседей. Прямоугольник обёртки нужен справке,
     поэтому не display: contents. В узком режиме .rb и так width: 100% — без изменений. */
  .group > div {
    display: flex;
    flex-direction: column;
  }
  /* Черта между группами одного столбца */
  .group.next {
    margin-top: var(--sep);
    position: relative;
  }
  .group.next::before {
    content: '';
    position: absolute;
    left: 6px;
    right: 6px;
    top: calc(-0.5 * var(--sep));
    border-top: 1px solid var(--line);
  }
  .cap {
    height: calc(var(--head) - 4px); /* 4 — зазор группы до первой кнопки: вместе ровно HEAD */
    padding-top: 3px;
    box-sizing: border-box;
    font-size: 10px;
    line-height: 9px;
    text-align: center;
    color: var(--muted);
    white-space: nowrap;
    user-select: none;
  }
  /* rb — и у кнопки MirrorMenu в ленте */
  .rail :global(.rb) {
    width: 100%;
    height: var(--btn);
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 6px;
  }
  /* Подписи: колонка по самой длинной, значок слева, подпись справа */
  .rail.wide .col {
    width: auto;
    min-width: var(--col-w);
  }
  .rail.wide :global(.rb) {
    width: auto;
    display: flex;
    align-items: center;
    gap: 13px; /* 7 + LABEL_GAP (6): значок остаётся по центру своих 32 px */
    padding: 0 10px 0 7px; /* 10 = LABEL_PAD */
    font-size: 12px;
    white-space: nowrap;
  }
  .rail :global(.rb:hover:not(.off)) {
    background: #48484d;
    color: var(--text);
  }
  .rail :global(.rb.off) {
    opacity: 0.4;
    cursor: default;
  }
  .rail :global(.rb[aria-expanded='true']) {
    background: var(--accent);
    color: #1e1e1e;
  }
</style>
