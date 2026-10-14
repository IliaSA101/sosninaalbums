<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { errText } from '../../base/core/errtext'
  import { sheetSize } from '../../base/geom/sheet'
  import type { SpreadView, VigChoice, VignetteView } from '../../base/core/types'
  import { gallery } from '../layouts/gallery.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import TplRow from '../../base/ui/TplRow.svelte'
  import TplItem from '../../base/ui/TplItem.svelte'
  import VigCard from './VigCard.svelte'
  import { changeTpl, vigChoices } from './vigact'
  import { canEditOwn, ownLabel, tileLines } from './vigown'
  import { vigOwn } from './vigown.svelte'
  import { formOf, portraitText } from './vignette'

  // Галерея ▦ на виньетке (решение 80): та же рамка, что у страницы (GridGallery) — ряды «Встроенные» и
  // «Мои шаблоны» (TplRow/TplItem), «Сохранить как шаблон…» справа сверху. Числа и пропорция — те, что стоят
  // на листе; щелчок применяет шаблон (один шаг отмены), на ручной виньетке — после вопроса, как у страницы.
  let { sp }: { sp: SpreadView & { vig: VignetteView } } = $props()

  const H = 56 // высота превью, как у карточек сеток страницы
  const sheet = $derived(sheetSize(sp))
  const cardW = $derived((H * sheet.w) / sheet.h)
  const cur = $derived(formOf(sp.vig))
  const canTpl = $derived(prefs.can('templates')) // сохранить, ✎ и × — с уровня «Шаблоны»

  let items = $state<VigChoice[] | null>(null) // null — ответ Go ещё не пришёл
  const builtin = $derived((items ?? []).filter((c) => !c.own))
  const mine = $derived((items ?? []).filter((c) => c.own))
  let req = 0
  $effect(() => {
    const f = { teachers: cur.teachers, pupils: cur.pupils, ratio: cur.ratio }
    void store.view.rev
    void vigOwn.rev // свои шаблоны сохранили, переименовали или удалили
    const my = ++req
    vigChoices(sp.id, f)
      .then((list) => {
        if (my === req) items = list
      })
      .catch((e) => {
        if (my === req) store.notify(errText(e), 'error')
      })
  })

  // Закрываем сразу, как у обычных раскладок; шаблон, что уже стоит, — просто закрытие.
  function pick(c: VigChoice) {
    gallery.close()
    if (c.id !== cur.tpl && !c.over && !c.gone) void changeTpl(sp.id, cur, items ?? [], c.id)
  }

  // Портрет в мм и «мелко» — мелкой строкой под плиткой, полный текст и пояснение — в title
  const tip = (c: VigChoice) => `${ownLabel(c)}: ${portraitText(c)}. Под превью — ширина портрета ученика`
</script>

{#if canTpl}
  <div class="bar">
    <button
      type="button"
      class="text"
      onclick={() => vigOwn.save(sp.id)}
      title="Запомнить места, шапку и стиль подписей этого листа как свой шаблон">Сохранить как шаблон…</button
    >
  </div>
{/if}

{#snippet row(label: string, list: VigChoice[], empty: string, own: boolean)}
  <TplRow {label} count={list.length} empty={items ? empty : ''} tall watch={items}>
    {#each list as c (c.id)}
      {@const edit = canEditOwn(c, canTpl)}
      <TplItem
        name={own ? ownLabel(c) : undefined}
        note={tileLines(c).join(' · ')}
        tone={c.over ? 'over' : c.small ? 'small' : undefined}
        onrename={edit ? () => vigOwn.rename(c) : undefined}
        ondelete={edit ? () => vigOwn.del(c) : undefined}
      >
        <button
          class="tile"
          class:current={c.id === cur.tpl}
          disabled={c.over || c.gone}
          title={tip(c)}
          aria-label="{ownLabel(c)}, {portraitText(c)}"
          onclick={() => pick(c)}
        >
          <VigCard {c} w={cardW} h={H} />
        </button>
      </TplItem>
    {/each}
  </TplRow>
{/snippet}

{@render row('Встроенные', builtin, 'Нет шаблонов для таких чисел', false)}
{@render row('Мои шаблоны', mine, 'Нет своих шаблонов виньетки', true)}

<style>
  .bar {
    display: flex;
    align-items: center;
    height: 32px;
  }
  .bar > .text {
    margin-left: auto;
    height: 26px;
    font-size: 12px;
    padding: 0 10px;
  }
  .tile {
    all: unset;
    display: block;
    cursor: pointer;
  }
  .tile:disabled {
    cursor: default;
  }
  /* Рамка — вокруг превью */
  .tile :global(.card) {
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition: outline-color 0.15s;
  }
  .tile:hover:not(:disabled) :global(.card) {
    outline-color: #8a8a90;
  }
  .tile.current :global(.card) {
    outline-color: var(--accent);
  }
  .tile:focus-visible :global(.card) {
    outline-color: var(--accent);
  }
</style>
