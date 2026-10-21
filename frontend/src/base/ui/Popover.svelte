<script lang="ts" module>
  // Номер открытия: окно с большим номером открыто позже и считается вложенным в окна с меньшим.
  let seq = 0
</script>

<script lang="ts">
  import { untrack, type Snippet } from 'svelte'
  import { BARPOP_OPEN, closesOnPeer, placePopover, type Placed } from './popover'

  // Окно панели (спека 024): под кнопкой, в body (лента обрезает overflow). Закрывается Esc (только верхнее),
  // щелчком и колесом снаружи, сменой размера окна. Окна со своим `data-barpop` и без номера или с большим
  // номером считаются вложенными: щелчок, колесо и Esc внутри них это окно не закрывает.
  // Крупный просмотр и галерею проверяет вызывающий: base/ui их не знает.
  let {
    anchor,
    open = $bindable(false),
    label,
    kind,
    children,
  }: { anchor: HTMLElement | undefined; open?: boolean; label: string; kind: string; children: Snippet } = $props()

  let pop = $state<HTMLElement>()
  let placed = $state<Placed | null>(null)
  let z = $state(0)
  let tick = $state(0) // размер содержимого или место якоря изменились — позицию считаем заново

  $effect.pre(() => {
    if (open) {
      if (untrack(() => z) === 0) z = ++seq
    } else {
      z = 0
      placed = null
    }
  })

  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  $effect(() => {
    if (!open || !pop) return
    const ro = new ResizeObserver(() => tick++)
    ro.observe(pop)
    return () => ro.disconnect()
  })

  // Якорь мог сдвинуться без resize окна (другое окно «⋯» сдвинулось, подпись кнопки удлинилась, открылась панель
  // деталей): пока окно открыто, сверяем rect якоря раз в кадр.
  $effect(() => {
    if (!open || !anchor) return
    const el = anchor
    let last = ''
    let raf = 0
    const watch = () => {
      const r = el.getBoundingClientRect()
      const key = `${r.left}|${r.top}|${r.width}|${r.height}`
      if (key !== last) {
        if (last !== '') tick++
        last = key
      }
      raf = requestAnimationFrame(watch)
    }
    watch()
    return () => cancelAnimationFrame(raf)
  })

  $effect(() => {
    void tick
    if (!open || !pop || !anchor) return
    const a = anchor.getBoundingClientRect()
    placed = placePopover(
      { left: a.left, top: a.top, width: a.width, height: a.height },
      { width: pop.offsetWidth, height: pop.offsetHeight },
      { width: window.innerWidth, height: window.innerHeight },
    )
  })

  // Открылось другое окно панели — это закрываем (одно окно панели за раз); своё открытие объявляем остальным.
  $effect(() => {
    if (!open) return
    const on = (e: Event) => {
      if (closesOnPeer((e as CustomEvent).detail, { anchor, pop })) open = false
    }
    window.addEventListener(BARPOP_OPEN, on)
    return () => window.removeEventListener(BARPOP_OPEN, on)
  })
  $effect(() => {
    if (!open) return
    untrack(() => {
      if (anchor) window.dispatchEvent(new CustomEvent(BARPOP_OPEN, { detail: anchor }))
    })
  })

  // Вложенное окно относительно этого: другое окно панели без номера или с большим номером.
  function nestedOf(el: Element | null): boolean {
    const o = el?.closest<HTMLElement>('[data-barpop]')
    if (!o || o === pop) return false
    const oz = o.dataset.barpopZ
    return oz === undefined || Number(oz) > z
  }

  function hasNested(): boolean {
    return [...document.querySelectorAll<HTMLElement>('[data-barpop]')].some((o) => nestedOf(o))
  }

  function close(refocus = false) {
    open = false
    if (refocus) anchor?.focus()
  }

  function onKey(e: KeyboardEvent) {
    // defaultPrevented: вложенное окно уже обработало Esc (его DOM мог успеть исчезнуть между обработчиками)
    if (!open || e.key !== 'Escape' || e.defaultPrevented || hasNested()) return
    e.stopPropagation()
    e.preventDefault()
    close(true)
  }

  function outside(t: EventTarget | null): boolean {
    const n = t as Node
    return !pop?.contains(n) && !anchor?.contains(n) && !nestedOf(n instanceof Element ? n : n.parentElement)
  }

  // Колесо вне окна — пользователь крутит ленту; scroll не годится: браузер шлёт его сам при смене содержимого.
  const onWheel = (e: WheelEvent) => open && outside(e.target) && close()
  const onPointer = (e: PointerEvent) => open && outside(e.target) && close()
</script>

<svelte:window onkeydowncapture={onKey} onpointerdowncapture={onPointer} onwheelcapture={onWheel} onresize={() => close()} />

{#if open}
  <div
    bind:this={pop}
    class="pop"
    use:portal
    role="dialog"
    aria-label={label}
    data-barpop={kind}
    data-barpop-z={z || undefined}
    style:left="{placed?.left ?? 0}px"
    style:top="{placed?.top ?? 0}px"
    style:visibility={placed ? 'visible' : 'hidden'}
  >
    {@render children()}
  </div>
{/if}

<style>
  .pop {
    position: fixed;
    z-index: 90;
    padding: 10px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.5);
    font-size: 12px;
  }
</style>
