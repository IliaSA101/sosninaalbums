<script lang="ts">
  import Spread from './Spread.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { row } from './spreadrow.svelte'
  import { pitch, renderRange } from './spreadrow'
  import { labelTitle } from '../../panels/timeline/spreadname'
  import type { Settings } from '../../base/core/types'

  type Box = { left: number; top: number; width: number; height: number }

  // Содержимое окна холста: слоты ряда (развороты и «новый»), пустой альбом. fit — вписанный разворот
  // (px окна), place — действующий (в зуме отличается), width/spreadW — ширина рисунка соседа/текущего.
  let {
    settings,
    width,
    spreadW,
    w,
    aspect,
    fit,
    place,
    zoomed,
    onhover,
  }: {
    settings: Settings
    width: number
    spreadW: number
    w: number
    aspect: number
    fit: Box
    place: Box
    zoomed: boolean
    onhover: (on: boolean) => void
  } = $props()

  const spreads = $derived(store.view.spreads)
  const n = $derived(spreads.length)
  const cur = $derived(store.current)
  const P = $derived(pitch(fit.width))
  const range = $derived(renderRange(row.pos, n, row.max, w, fit.width, P, zoomed, cur))
  // Ряд едет transform'ом (композитор); в покое transform нет — у position: fixed внутри нет лишнего контейнера.
  const shift = $derived(!zoomed && row.pos !== cur ? `translate3d(${(cur - row.pos) * P}px, 0, 0)` : undefined)
  // Слот i: от текущего на шаг P; в зуме текущий стоит по placement.
  const box = (i: number): Box => (zoomed ? place : { ...fit, left: fit.left + (i - cur) * P })
</script>

{#if n === 0}
  <div
    class="first"
    class:over={drag.over === 'new:0'}
    style:left="{fit.left}px"
    style:top="{fit.top}px"
    style:width="{width}px"
    style:height="{width / aspect}px"
    data-drop="new"
    data-at="0"
  >
    {#if store.loaded}
      <div>
        <div class="big">Альбом пуст</div>
        {#if modes.big}
          Выключите крупный просмотр (F), чтобы добавить фото
        {:else if store.photos.length}
          Выделите фото в ленте и перетащите сюда<br />или нажмите «Автосборка»
        {:else}
          Сначала добавьте папку с фотографиями (кнопка внизу)<br />или перетащите папку в окно
        {/if}
      </div>
    {/if}
  </div>
{:else}
  <div class="row" class:moving={row.moving} style:transform={shift}>
    <!-- Ключ — id разворота: один и тот же экземпляр Spread у слота при смене текущего (mini ↔ полный), без
         перемонтирования и мерцания. Не заменять на {#if i === cur} с двумя компонентами или ключ по cur. -->
    {#each range as i (spreads[i]?.id ?? 'new')}
      {@const sp = spreads[i]}
      {@const b = box(i)}
      {#if sp}
        {@const isCur = i === cur}
        <div
          class="slot"
          style:left="{b.left}px"
          style:top="{b.top}px"
          style:width="{b.width}px"
          style:height="{b.height}px"
          data-current={isCur ? '' : undefined}
          role="presentation"
          onmouseenter={isCur ? () => onhover(true) : undefined}
          onmouseleave={isCur ? () => onhover(false) : undefined}
        >
          <Spread spread={sp} {settings} width={isCur ? spreadW : width} mini={!isCur} peer={!isCur} />
          {#if !isCur}
            <!-- Слой соседа: щелчок, перетаскивание ряда и бросок фото «на весь разворот» -->
            <div
              class="peer"
              class:over={drag.over === `spread:${sp.id}`}
              data-peer
              data-drop="spread"
              data-spread={sp.id}
              data-index={i}
              role="presentation"
              title={labelTitle(i, sp.name) ?? `Разворот ${i + 1}`}
              onclick={() => drag.clickAllowed() && row.to(i)}
            ></div>
          {/if}
        </div>
      {:else}
        <div class="slot" style:left="{b.left}px" style:top="{b.top}px" style:width="{b.width}px" style:height="{b.height}px">
          <button
            type="button"
            class="new"
            class:over={drag.over === `new:${n}`}
            data-drop="new"
            data-at={n}
            title="Щёлкните — новый пустой разворот; или перетащите сюда фото"
            onclick={(e) => e.detail < 2 && drag.clickAllowed() && store.addEmptySpread(n)}
          >
            {#if modes.big}
              <Icon name="plus" size={24} />
            {:else}
              <span>+ Новый разворот<br />перетащите фото сюда</span>
            {/if}
          </button>
        </div>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .row {
    position: absolute;
    inset: 0;
  }
  .row.moving {
    will-change: transform;
  }
  .slot {
    position: absolute;
    z-index: 0; /* свой контекст: слой соседа (z 20) не перекрывает панели текущего */
  }
  /* Рамка выделения, ручки, мини-панель и уголок текущего выходят за лист и не должны уходить под соседей */
  .slot[data-current] {
    z-index: 1;
  }
  .peer {
    position: absolute;
    inset: 0;
    z-index: 20;
    cursor: pointer;
    outline: 3px solid transparent;
    outline-offset: 4px;
    transition: outline-color 0.15s;
  }
  .peer.over {
    outline-color: var(--accent);
  }
  .new {
    width: 100%;
    height: 100%;
    background: none;
    font: inherit;
    cursor: pointer;
    box-sizing: border-box;
    border: 2px dashed #6a6a70;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    color: #8a8a90;
    font-size: 13px;
    text-align: center;
    transition:
      border-color 0.15s,
      background 0.15s;
  }
  .new.over,
  .first.over {
    border-color: var(--accent);
    background: rgba(245, 217, 10, 0.08);
    color: var(--accent);
  }
  .first {
    position: absolute;
    border: 2px dashed #6a6a70;
    display: grid;
    place-items: center;
    text-align: center;
    color: #9a9aa0;
    line-height: 1.6;
  }
  .big {
    font-size: 22px;
    color: #ddd;
    margin-bottom: 8px;
  }
</style>
