<script lang="ts">
  import { tick, untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { run } from './commands'
  import { menuUi } from './menuui.svelte'
  import { menuCtx } from './menuctx'
  import { buildMenu, hasIcons, hasMarks, type MenuId, type MenuItem } from './menu'
  import Icon from '../../base/ui/Icon.svelte'
  import { placePopover, placeSubmenu, type Box } from '../../base/ui/popover'

  // Своя строка меню (не системная, спека 010): щелчок открывает, наведение переключает соседнее меню,
  // Esc и щелчок мимо закрывают, ← → ↑ ↓ Enter работают. Пункты и серые — menu/menu.ts.
  const menus = $derived(buildMenu(menuCtx()))

  let root = $state<HTMLElement>()
  let drop = $state<HTMLElement>()
  let subDrop = $state<HTMLElement>()
  const openId = $derived(menuUi.openId)
  let anchor = $state<Box>({ left: 0, top: 0, width: 0, height: 0 })
  let sub = $state<{ id: string; anchor: Box } | null>(null)
  let w = $state(0)
  let h = $state(0)
  let sw = $state(0)
  let sh = $state(0)

  const menu = $derived(menus.find((m) => m.id === openId))
  const subItems = $derived(sub ? (menu?.items.find((i) => i.id === sub!.id)?.sub ?? []) : [])
  const view = () => ({ width: window.innerWidth, height: window.innerHeight })
  const pos = $derived(placePopover(anchor, { width: w, height: h }, view(), 0))
  const subPos = $derived(sub ? placeSubmenu(sub.anchor, { width: sw, height: sh }, view()) : null)

  const box = (el: Element): Box => {
    const r = el.getBoundingClientRect()
    return { left: r.left, top: r.top, width: r.width, height: r.height }
  }

  function close() {
    menuUi.openId = null
    sub = null
    w = h = sw = sh = 0
  }

  const enabled = (c: HTMLElement | undefined) => [...(c?.querySelectorAll<HTMLElement>('button:not(:disabled)') ?? [])]
  // Контейнер читаем после tick: подменю только что отрисовано, и bind:this к вызову ещё пуст.
  const focusFirst = (get: () => HTMLElement | undefined) => tick().then(() => enabled(get())[0]?.focus())

  function openMenu(id: MenuId, btn: Element, focus: boolean) {
    anchor = box(btn)
    sub = null
    w = h = 0
    menuUi.openId = id
    if (id === 'file') void store.refreshRecent()
    if (focus) focusFirst(() => drop)
  }

  // Справочник просит раскрыть меню (спека 033): как щелчком, но без фокуса на пункте.
  $effect(() => {
    const r = menuUi.req
    if (!r) return
    untrack(() => {
      const btn = root?.querySelectorAll('.bar > button')[menus.findIndex((m) => m.id === r.id)]
      if (btn) openMenu(r.id, btn, false)
    })
    menuUi.req = null
  })

  function toggle(id: MenuId, btn: Element) {
    if (openId === id) close()
    else openMenu(id, btn, true)
  }

  function hover(id: MenuId, btn: Element) {
    if (openId && openId !== id) openMenu(id, btn, false)
  }

  // Верхние кнопки по порядку: ← → переключают меню.
  function switchMenu(delta: number) {
    const btns = [...(root?.querySelectorAll<HTMLElement>('.bar > button') ?? [])]
    const at = menus.findIndex((m) => m.id === openId)
    const to = (at + delta + menus.length) % menus.length
    openMenu(menus[to].id, btns[to], true)
  }

  function openSub(it: MenuItem, el: Element, focus: boolean) {
    if (it.disabled) return
    sub = { id: it.id, anchor: box(el) }
    sw = sh = 0
    if (focus) focusFirst(() => subDrop)
  }

  // Наведение на пункт основного списка открывает его подменю или закрывает чужое;
  // на пункт самого подменю (nested) ничего не меняет — иначе оно закрылось бы под курсором.
  function enter(it: MenuItem, el: Element, nested: boolean) {
    if (nested) return
    if (it.sub) openSub(it, el, false)
    else sub = null
  }

  // viaKey: Enter/Пробел (click без указателя) на «▸» уводит фокус в подменю, как →.
  function pick(it: MenuItem, el: Element, viaKey = false) {
    if (it.disabled) return
    if (it.sub) return sub?.id === it.id && !viaKey ? (sub = null) : openSub(it, el, viaKey)
    if (!it.cmd) return
    const cmd = it.cmd
    close()
    ;(document.activeElement as HTMLElement | null)?.blur()
    run(cmd, undefined, it.arg)
  }

  function move(delta: number) {
    const inSub = !!subDrop && subDrop.contains(document.activeElement)
    const list = enabled(inSub ? subDrop : drop)
    if (list.length === 0) return
    const at = list.indexOf(document.activeElement as HTMLElement)
    const to = at < 0 ? (delta > 0 ? 0 : list.length - 1) : (at + delta + list.length) % list.length
    list[to].focus()
  }

  // →: на пункте с подменю — войти в него, иначе соседнее меню; внутри подменю ничего.
  function arrowRight(active: HTMLElement | null, inSub: boolean) {
    const it = menu?.items.find((i) => i.id === active?.dataset.item)
    if (it?.sub && !inSub) openSub(it, active!, true)
    else if (!inSub) switchMenu(1)
  }

  // ←: из подменю — назад к его пункту, иначе соседнее меню.
  function arrowLeft(inSub: boolean) {
    if (!inSub) return switchMenu(-1)
    const parent = drop?.querySelector<HTMLElement>(`[data-item="${sub?.id}"]`)
    sub = null
    parent?.focus()
  }

  function key(e: KeyboardEvent) {
    if (!openId) return
    // Пока меню открыто, горячие клавиши приложения молчат (как у меню фото); Enter и Пробел делает кнопка.
    e.stopPropagation()
    const active = document.activeElement as HTMLElement | null
    const inSub = !!subDrop && subDrop.contains(active)
    switch (e.key) {
      case 'Escape':
        e.preventDefault()
        close()
        break
      case 'Tab':
        close()
        break
      case 'ArrowDown':
      case 'ArrowUp':
        e.preventDefault()
        move(e.key === 'ArrowDown' ? 1 : -1)
        break
      case 'ArrowRight':
        e.preventDefault()
        arrowRight(active, inSub)
        break
      case 'ArrowLeft':
        e.preventDefault()
        arrowLeft(inSub)
        break
    }
  }

  function outside(e: PointerEvent) {
    if (openId && root && !root.contains(e.target as Node)) close()
  }
</script>

<svelte:window onkeydowncapture={key} onpointerdowncapture={outside} onresize={close} onblur={close} />

{#snippet items(list: MenuItem[], nested: boolean)}
  {@const icons = hasIcons(list)}
  {@const marks = hasMarks(list)}
  {#each list as it (it.id)}
    {#if it.sep}<div class="sep" role="separator"></div>{/if}
    <button
      role={it.checked !== undefined ? 'menuitemcheckbox' : 'menuitem'}
      aria-checked={it.checked}
      aria-haspopup={it.sub ? 'menu' : undefined}
      aria-expanded={it.sub ? sub?.id === it.id : undefined}
      class:iconed={icons && !marks}
      data-item={it.id}
      disabled={it.disabled}
      title={it.hint}
      onpointerenter={(e) => enter(it, e.currentTarget, nested)}
      onclick={(e) => pick(it, e.currentTarget, e.detail === 0)}
    >
      {#if marks}<span class="mark" aria-hidden="true">{it.checked ? '✓' : ''}</span>{/if}
      {#if icons}<span class="ico" aria-hidden="true"
          >{#if it.icon}<Icon name={it.icon} size={16} stroke={1.8} />{/if}</span
        >{/if}
      <span class="label">{it.label}</span>
      {#if it.sub}<kbd>▸</kbd>{:else if it.keys}<kbd>{it.keys}</kbd>{/if}
    </button>
  {/each}
{/snippet}

<div class="root" bind:this={root} data-chrome data-text-ui>
  <div class="bar" role="menubar" tabindex="-1">
    {#each menus as m (m.id)}
      <button
        class="ghost"
        class:open={openId === m.id}
        role="menuitem"
        aria-haspopup="menu"
        aria-expanded={openId === m.id}
        onclick={(e) => toggle(m.id, e.currentTarget)}
        onpointerenter={(e) => hover(m.id, e.currentTarget)}>{m.label}</button
      >
    {/each}
  </div>

  {#if menu}
    <div
      class="drop"
      role="menu"
      tabindex="-1"
      bind:this={drop}
      bind:clientWidth={w}
      bind:clientHeight={h}
      style:left="{pos.left}px"
      style:top="{pos.top}px"
      style:visibility={h ? undefined : 'hidden'}
    >
      {@render items(menu.items, false)}
    </div>
  {/if}

  {#if sub && subItems.length > 0}
    <div
      class="drop"
      role="menu"
      tabindex="-1"
      bind:this={subDrop}
      bind:clientWidth={sw}
      bind:clientHeight={sh}
      style:left="{subPos?.left ?? 0}px"
      style:top="{subPos?.top ?? 0}px"
      style:visibility={sh ? undefined : 'hidden'}
    >
      {@render items(subItems, true)}
    </div>
  {/if}
</div>

<style>
  .root {
    flex: none;
  }
  .bar {
    display: flex;
    gap: 2px;
  }
  .bar button {
    padding: 0 10px;
  }
  .bar button.open {
    background: #3a3a3e;
  }
  .drop {
    position: fixed;
    z-index: 70;
    min-width: 220px;
    padding: 4px;
    background: rgba(30, 30, 32, 0.97);
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
    font-size: 13px;
  }
  .drop button {
    width: 100%;
    height: 28px;
    padding: 0 10px 0 4px;
    display: flex;
    align-items: center;
    gap: 6px;
    text-align: left;
    background: transparent;
    color: var(--text);
    border-radius: 4px;
  }
  .drop button:hover:not(:disabled),
  .drop button:focus-visible {
    background: #38383c;
    color: #fff;
  }
  .drop button:focus-visible {
    outline: none;
  }
  .mark {
    flex: none;
    width: 16px;
    text-align: center;
    color: var(--accent);
  }
  /* Список со значками и без отметок — как MenuList (меню фото): отступ 10, зазор 8 */
  .drop button.iconed {
    padding-left: 10px;
    gap: 8px;
  }
  .ico {
    flex: none;
    width: 16px;
    height: 16px;
    display: flex;
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
  .sep {
    height: 1px;
    margin: 4px 2px;
    background: var(--line);
  }
</style>
