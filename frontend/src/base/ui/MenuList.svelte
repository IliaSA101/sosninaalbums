<script lang="ts">
  import { portal } from './portal'
  import { tick } from 'svelte'
  import { arrowKey, markText, nextIndex, nodeKey, nodeRole, placeAtPoint, placeBeside, type MenuNode } from './menulist'
  import type { Box } from './popover'

  // Общий список меню (спека 108): меню правого щелчка по фото и список «⋯» мини-панели. Вид и клавиши — как у строки
  // меню программы (MenuBar): подменю сбоку при наведении, у края окна — влево; ← → ↑ ↓ Enter Esc; отметка слева.
  // Список уходит в body: мини-панель стоит в развороте с transform, и position: fixed считался бы от неё.
  // onpick получает пункт без подменю и список не закрывает — закрывает вызывающий (у него свои условия).
  let {
    items,
    at,
    side = 'right',
    keep,
    onpick,
    onclose,
  }: {
    items: MenuNode[]
    at: { x: number; y: number } | Box // точка щелчка (меню фото) или кнопка (⋯)
    side?: 'right' | 'left' // куда от кнопки; у точки щелчка не используется
    keep?: HTMLElement // кнопка, открывшая список: щелчок по ней «мимо» не считается
    onpick: (n: MenuNode) => void
    onclose: () => void
  } = $props()

  let drop = $state<HTMLElement>()
  let subDrop = $state<HTMLElement>()
  let sub = $state<{ id: string; anchor: Box } | null>(null)
  let w = $state(0)
  let h = $state(0)
  // Текущий пункт (список и номер): подсвечен ровно он. Наведение и клавиши оба ставят фокус, фокус — единственный источник;
  // отдельный :hover красил бы второй пункт, пока курсор стоит над прежним.
  let cur = $state<{ nested: boolean; i: number } | null>(null)
  let sw = $state(0)
  let sh = $state(0)

  const view = () => ({ width: window.innerWidth, height: window.innerHeight })
  const pos = $derived(
    'x' in at ? placeAtPoint(at, { width: w, height: h }, view()) : placeBeside(at, { width: w, height: h }, view(), side),
  )
  const subItems = $derived(sub ? (items.find((n) => nodeKey(n) === sub!.id)?.sub ?? []) : [])
  const subPos = $derived(sub ? placeBeside(sub.anchor, { width: sw, height: sh }, view(), 'x' in at ? 'right' : side) : null)

  const box = (el: Element): Box => {
    const r = el.getBoundingClientRect()
    return { left: r.left, top: r.top, width: r.width, height: r.height }
  }
  const idx = (el: Element | null): number => Number((el as HTMLElement | null)?.dataset.i ?? -1)
  const itemAt = (c: HTMLElement | undefined, i: number) => c?.querySelector<HTMLElement>(`[data-i="${i}"]`)

  // Фокус на первый доступный пункт: стрелки и Enter работают сразу. Контейнер читаем после tick: portal уже отработал.
  $effect(() => {
    if (!drop) return
    void tick().then(() => {
      const i = nextIndex(items, -1, 1)
      if (i >= 0) itemAt(drop, i)?.focus()
    })
  })

  function openSub(n: MenuNode, el: Element, focus: boolean) {
    if (n.disabled) return
    sub = { id: nodeKey(n), anchor: box(el) }
    sw = sh = 0
    if (focus)
      void tick().then(() => {
        const i = nextIndex(n.sub ?? [], -1, 1)
        if (i >= 0) itemAt(subDrop, i)?.focus()
      })
  }

  // Наведение переносит текущий пункт клавиатуры на наведённый (↓/↑ идут от него, а не от первого).
  // Заодно основной список открывает подменю пункта или закрывает чужое;
  // на пункт самого подменю подменю не меняется — иначе оно закрылось бы под курсором.
  function enter(n: MenuNode, el: Element, nested: boolean) {
    if (!n.disabled) (el as HTMLElement).focus({ preventScroll: true })
    if (nested) return
    if (!n.sub) sub = null
    else if (sub?.id !== nodeKey(n)) openSub(n, el, false)
  }

  // viaKey: Enter или Пробел (щелчок без указателя) на «▸» уводит фокус в подменю, как →.
  function pick(n: MenuNode, el: Element, viaKey: boolean) {
    if (n.disabled) return
    if (n.sub) {
      if (sub?.id === nodeKey(n) && !viaKey) sub = null
      else openSub(n, el, viaKey)
      return
    }
    onpick(n)
  }

  function move(delta: 1 | -1, inSub: boolean) {
    const c = inSub ? subDrop : drop
    const to = nextIndex(inSub ? subItems : items, idx(document.activeElement), delta)
    if (to >= 0) itemAt(c, to)?.focus()
  }

  function key(e: KeyboardEvent) {
    // Пока список открыт, горячие клавиши приложения молчат (как у меню программы); Enter и Пробел делает кнопка.
    e.stopPropagation()
    const active = document.activeElement
    const inSub = !!subDrop && subDrop.contains(active)
    switch (e.key) {
      case 'Escape':
        e.preventDefault()
        onclose()
        break
      case 'Tab':
        onclose()
        break
      case 'ArrowDown':
      case 'ArrowUp':
        e.preventDefault()
        move(e.key === 'ArrowDown' ? 1 : -1, inSub)
        break
      case 'ArrowRight':
      case 'ArrowLeft': {
        e.preventDefault()
        const n = inSub ? undefined : items[idx(active)]
        const what = arrowKey(e.key, n, inSub)
        if (what === 'enter' && n && active) openSub(n, active, true)
        else if (what === 'back') {
          const parent = items.findIndex((p) => nodeKey(p) === sub?.id)
          sub = null
          itemAt(drop, parent)?.focus()
        }
        break
      }
    }
  }

  function outside(e: PointerEvent) {
    const t = e.target as Node
    if (!drop?.contains(t) && !subDrop?.contains(t) && !keep?.contains(t)) onclose()
  }
</script>

<svelte:window onkeydowncapture={key} onpointerdowncapture={outside} onresize={onclose} onblur={onclose} />

{#snippet rows(list: MenuNode[], nested: boolean)}
  {#each list as it, i (nodeKey(it))}
    {#if it.sep && i > 0}<div class="sep" role="separator"></div>{/if}
    {#if it.kind === 'note'}
      <div class="note">{it.label}</div>
    {:else}
      <button
        class:danger={it.danger}
        role={nodeRole(it)}
        aria-checked={it.mark ? !!it.checked : undefined}
        aria-haspopup={it.sub ? 'menu' : undefined}
        aria-expanded={it.sub ? sub?.id === nodeKey(it) : undefined}
        class:cur={cur?.nested === nested && cur.i === i}
        data-i={i}
        data-item={it.id}
        onfocus={() => (cur = { nested, i })}
        disabled={it.disabled}
        onpointerenter={(e) => enter(it, e.currentTarget, nested)}
        onclick={(e) => pick(it, e.currentTarget, e.detail === 0)}
      >
        <span class="mark" aria-hidden="true">{markText(it)}</span>
        <span class="label">{it.label}</span>
        {#if it.sub}<kbd>▸</kbd>{:else if it.hint}<kbd>{it.hint}</kbd>{/if}
      </button>
    {/if}
  {/each}
{/snippet}

<!-- Корень — один span: portal уносит списки в body, и если бы корнем были они, Svelte при закрытии шёл бы по
     nextSibling от унесённого узла и сносил соседей в body (как с якорем мини-панели, спека 100 п. 5) -->
<span class="host">
  <div
    class="drop"
    role="menu"
    tabindex="-1"
    use:portal
    bind:this={drop}
    bind:clientWidth={w}
    bind:clientHeight={h}
    style:left="{pos.left}px"
    style:top="{pos.top}px"
    style:visibility={h ? undefined : 'hidden'}
    data-chrome
    data-text-ui
    oncontextmenu={(e) => e.preventDefault()}
  >
    {@render rows(items, false)}
  </div>

  <!-- key: у другого подменю элемент новый, и размер меряется заново (при том же размере наблюдатель молчит) -->
  {#if sub && subItems.length > 0}
    {#key sub.id}
      <div
        class="drop"
        role="menu"
        tabindex="-1"
        use:portal
        bind:this={subDrop}
        bind:clientWidth={sw}
        bind:clientHeight={sh}
        style:left="{subPos?.left ?? 0}px"
        style:top="{subPos?.top ?? 0}px"
        style:visibility={sh ? undefined : 'hidden'}
        data-chrome
        data-text-ui
        oncontextmenu={(e) => e.preventDefault()}
      >
        {@render rows(subItems, true)}
      </div>
    {/key}
  {/if}
</span>

<style>
  .host {
    display: none;
  }
  .drop {
    position: fixed;
    z-index: 70;
    min-width: 200px;
    padding: 4px;
    box-sizing: border-box;
    background: rgba(30, 30, 32, 0.97);
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
    font-size: 13px;
    user-select: none;
  }
  button {
    width: 100%;
    height: 28px;
    padding: 0 10px 0 4px;
    display: flex;
    align-items: center;
    gap: 6px;
    text-align: left;
    white-space: nowrap;
    background: transparent;
    color: var(--text);
    border-radius: 4px;
  }
  button:disabled {
    color: var(--muted);
    cursor: default;
  }
  /* Глобальный button:hover (style.css) подсветил бы и пункт под мышью, пока ↓ ведёт .cur в другое место:
     наведение и так делает пункт текущим, поэтому светится только .cur */
  button:hover:not(:disabled) {
    background: transparent;
  }
  button.cur,
  button.cur:hover:not(:disabled) {
    background: #38383c;
    color: #fff;
  }
  button:focus-visible {
    outline: none;
  }
  button.danger.cur {
    color: #ff8a8a;
  }
  .mark {
    flex: none;
    width: 16px;
    text-align: center;
    color: var(--accent);
  }
  .label {
    flex: 1;
  }
  kbd {
    margin-left: 24px;
    font: inherit;
    font-size: 11px;
    color: var(--muted);
  }
  .note {
    padding: 6px 10px 4px 22px;
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
  }
  .sep {
    height: 1px;
    margin: 4px 2px;
    background: var(--line);
  }
</style>
