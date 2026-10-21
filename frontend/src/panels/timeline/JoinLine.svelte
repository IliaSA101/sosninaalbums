<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tipped } from '../../base/ui/tip.svelte'
  import { isMac } from '../../base/core/platform'
  import { unitOf } from '../../base/geom/sheet'
  import { joinTip, type CanDo } from './timeline'
  import { joinClick, joinKey } from './stripact'

  // Черта между группами ленты: склейка — Ctrl+щелчок (на Mac ⌘, 094 §1), при перетаскивании фото — новый разворот здесь,
  // разворота — его место (095 сц. 9).
  // Склейка и разрез — функция «Разрез и склейка» (спека 011); без неё черта остаётся только целью переноса.
  let {
    i,
    id,
    state,
    cuts,
    cutting,
  }: {
    i: number // индекс левой группы
    id: string // разворот слева
    state: CanDo // можно ли склеить и почему нет (joinState)
    cuts: boolean
    cutting: boolean // зажат Ctrl: черта под курсором — акцентная
  } = $props()

  const unit = $derived(unitOf(store.view.settings))
  const blocked = $derived(cuts && !state.ok)
</script>

<button
  class="join"
  data-guide="join-line"
  class:blocked
  class:still={!cuts}
  class:armed={cutting}
  tabindex={cuts ? 0 : -1}
  class:over={drag.over === `new:${i + 1}`}
  aria-disabled={blocked}
  aria-hidden={!cuts}
  aria-label={state.ok ? `Склеить ${unit.many} ${i + 1} и ${i + 2}` : `Нельзя склеить ${unit.many} ${i + 1} и ${i + 2}`}
  data-drop="new"
  data-at={i + 1}
  onpointerdown={(e) => e.stopPropagation()}
  use:tipped={{ text: cuts ? joinTip(state, isMac()) : '' }}
  onclick={(e) => joinClick(e, id, state, cuts)}
  onkeydown={(e) => joinKey(e, id, state, cuts)}
></button>

<style>
  .join {
    all: unset;
    position: relative;
    flex: none;
    width: 14px;
    height: calc(var(--h) + 2 * var(--pad));
    cursor: default;
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
  /* Простой щелчок не клеит (094 §1): подсветка и курсор-рука — только при зажатом Ctrl или с клавиатуры. */
  .join.armed {
    cursor: pointer;
  }
  .join.armed:hover::before,
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
  .join.blocked.armed {
    cursor: not-allowed;
  }
  /* Нельзя склеить: черта и её подсветка приглушены, как у неактивных кнопок. */
  .join.blocked::before {
    background: #111;
    opacity: 0.4;
  }
  .join.blocked.armed:hover::before,
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
