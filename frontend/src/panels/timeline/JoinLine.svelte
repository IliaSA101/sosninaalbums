<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import { unitOf } from '../../base/geom/sheet'

  // Черта между группами ленты: в покое — «Склеить», при перетаскивании — новый разворот здесь.
  // Склейка и разрез — функция «Разрез и склейка» (спека 011); без неё черта остаётся только целью переноса.
  let {
    i,
    id,
    cnt,
    gridJoin,
    cuts,
    max,
  }: {
    i: number // индекс левой группы
    id: string // разворот слева
    cnt: number // сколько фото будет после склейки
    gridJoin: boolean // у одного из двух разворотов сетка: склейка закрыта до «Вернуть авто»
    cuts: boolean
    max: number
  } = $props()

  const unit = $derived(unitOf(store.view.settings))
  const blocked = $derived(cuts && (gridJoin || cnt > max))
  const joinTip = $derived(
    gridJoin ? 'Сначала «Вернуть авто»' : cnt > max ? `Нельзя склеить: вместе ${cnt} фото, максимум ${max}` : 'Склеить',
  )

  function merge() {
    if (!cuts || !drag.clickAllowed() || gridJoin || cnt > max) return
    tip.hide()
    store.merge(id)
  }
</script>

<button
  class="join"
  data-guide="join-line"
  class:blocked
  class:still={!cuts}
  tabindex={cuts ? 0 : -1}
  class:over={drag.over === `new:${i + 1}`}
  aria-disabled={blocked}
  aria-hidden={!cuts}
  aria-label={gridJoin || cnt > max ? `Нельзя склеить ${unit.many} ${i + 1} и ${i + 2}` : `Склеить ${unit.many} ${i + 1} и ${i + 2}`}
  data-drop="new"
  data-at={i + 1}
  onpointerdown={(e) => e.stopPropagation()}
  use:tipped={{ text: cuts ? joinTip : '' }}
  onclick={merge}
></button>

<style>
  .join {
    all: unset;
    position: relative;
    flex: none;
    width: 14px;
    height: calc(var(--h) + 2 * var(--pad));
    cursor: pointer;
  }
  .join::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 6px;
    width: 2px;
    background: #111;
    transition:
      background 0.15s,
      width 0.15s,
      left 0.15s;
  }
  .join:hover::before,
  .join:focus-visible::before {
    background: var(--accent);
  }
  /* Без «Разреза и склейки» черта не нажимается: тихая, подсветки нет. */
  .join.still {
    cursor: default;
  }
  .join.still:hover::before,
  .join.still:focus-visible::before {
    background: #111;
  }
  .join.still.over::before {
    background: var(--accent);
  }
  .join.blocked {
    cursor: not-allowed;
  }
  /* Нельзя склеить: черта и её подсветка приглушены, как у неактивных кнопок. */
  .join.blocked::before {
    background: #111;
    opacity: 0.4;
  }
  .join.blocked:hover::before,
  .join.blocked:focus-visible::before {
    background: var(--accent);
    opacity: 0.4;
  }
  .join.over::before,
  .join.blocked.over::before {
    left: 5px;
    width: 4px;
    background: var(--accent);
    opacity: 1;
  }
</style>
