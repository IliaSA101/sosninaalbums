<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tipped } from '../../base/ui/tip.svelte'
  import { isMac, toggleKey } from '../../base/core/platform'
  import { unitOf } from '../../base/geom/sheet'
  import { gapKey } from '../../panels/timeline/cutkey.svelte'
  import { gapTip, type SlotBox } from './spreadrow'

  // Промежуток между соседними листами ряда (103 §3): с зажатым Ctrl (⌘) под курсором — черта и «+», Ctrl+щелчок
  // вставляет пустой разворот сюда. Без Ctrl зона прозрачна и мышь не ловит: щелчок, рамка, перенос ряда и бросок — как раньше.
  let {
    box,
    at,
  }: {
    box: SlotBox // место промежутка в окне, px
    at: number // индекс нового разворота в альбоме (после обложки — 0)
  } = $props()

  const unit = $derived(unitOf(store.view.settings))
  // Пока тащат фото или разворот, зона молчит: бросок не должен упираться в неё.
  const armed = $derived(gapKey.held && !drag.active)
</script>

<button
  type="button"
  class="gap"
  class:armed
  tabindex="-1"
  aria-hidden={!armed}
  aria-label="Вставить {unit.one} здесь"
  style:left="{box.left}px"
  style:top="{box.top}px"
  style:width="{box.width}px"
  style:height="{box.height}px"
  data-at={at}
  onpointerdown={(e) => e.stopPropagation()}
  use:tipped={{ text: armed ? gapTip(isMac(), unit) : '' }}
  onclick={(e) => armed && e.detail < 2 && toggleKey(e) && drag.clickAllowed() && store.addEmptySpread(at)}
>
  <span class="dot"><Icon name="plus" size={12} /></span>
</button>

<style>
  .gap {
    all: unset;
    position: absolute;
    z-index: 2; /* над слотами: ручки текущего листа выходят в промежуток */
    pointer-events: none;
    cursor: default;
  }
  .gap.armed {
    pointer-events: auto;
    cursor: pointer;
  }
  .gap::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: calc(50% - 1px);
    width: 2px;
    background: var(--accent);
    opacity: 0;
    transition: opacity 0.15s;
  }
  .dot {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 16px;
    height: 16px;
    margin: -8px 0 0 -8px;
    border-radius: 50%;
    background: var(--accent);
    color: #111;
    display: grid;
    place-items: center;
    opacity: 0;
    transition: opacity 0.15s;
  }
  /* Только под курсором (103-7): на холсте Ctrl жмут часто, мигающие черты мешали бы */
  .gap.armed:hover::before,
  .gap.armed:hover .dot {
    opacity: 1;
  }
</style>
