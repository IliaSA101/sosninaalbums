<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import { miniContext } from '../places/minipanel'
  import { Api, store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { useCard } from '../../base/prefs/cardgroups.svelte'
  import Icon from '../../base/ui/Icon.svelte'

  // Последняя строка группы «Кадр» (спека 124): «Повернуть ↺ ↻ · Зеркало ↔ ↕». Поворот на 90° — по фото карточки (TurnPhotos),
  // зеркало — по выделенным (спека 009 U4): подсвечено, если отражено у всех, решение «всем или никому» принимает Go (SetFlip).
  // В узкой карточке (200 px) слова скрыты: 4 кнопки и слова не помещаются, у кнопок остаются title и aria-label.
  // Сразу за этой строкой идёт «Цвет | Ч/б» (владелец 2026-10-18, страж photocard.order.dom.test.ts).
  let { id }: { id: string } = $props()

  const card = useCard()
  const c = $derived(miniContext(store.spread, placesel.selected))
  const turn = (quarters: -1 | 1) => store.run(Api.TurnPhotos([id], quarters))
</script>

<div class="trow" class:narrow={card.narrow}>
  {#if prefs.can('turn')}
    <div class="part" role="group" aria-label="Повернуть">
      {#if !card.narrow}<span class="lbl">Повернуть</span>{/if}
      <button class="ic" onclick={() => turn(-1)} aria-label="Повернуть на 90° против часовой" title="↺ 90°"
        ><Icon name="turn-ccw" size={16} /></button
      >
      <button class="ic" onclick={() => turn(1)} aria-label="Повернуть на 90° по часовой" title="↻ 90°"
        ><Icon name="turn-cw" size={16} /></button
      >
    </div>
  {/if}
  {#if prefs.can('flip')}
    <div class="part" role="group" aria-label="Зеркало" data-guide="flip-toggle">
      {#if !card.narrow}<span class="lbl">Зеркало</span>{/if}
      <button
        class="ic"
        class:on={c.flipH}
        aria-pressed={c.flipH}
        aria-label="Отразить по горизонтали"
        title="Отразить по горизонтали"
        onclick={() => placesel.flip('h')}><Icon name="flip-h" size={16} /></button
      >
      <button
        class="ic"
        class:on={c.flipV}
        aria-pressed={c.flipV}
        aria-label="Отразить по вертикали"
        title="Отразить по вертикали"
        onclick={() => placesel.flip('v')}><Icon name="flip-v" size={16} /></button
      >
    </div>
  {/if}
</div>

<style>
  .trow {
    display: flex;
    align-items: center;
    gap: 6px 12px;
    margin-top: 10px;
    font-size: 12px;
  }
  .trow.narrow {
    gap: 6px;
  }
  .part {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .lbl {
    margin-right: 4px;
    color: var(--muted);
  }
  .ic {
    width: 26px;
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: #38383c;
    color: #cfcfd4;
  }
  .ic.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .ic.on:hover {
    background: #ffe53a;
  }
</style>
