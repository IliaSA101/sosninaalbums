<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { canVary, deleteLabel, menuPos, setupItem, varyLabel, vignetteItem } from './spreadmenu'
  import { store } from '../../base/core/store.svelte'
  import { isSheet } from '../../base/geom/sheet'
  import { prefs } from '../../base/prefs/prefs.svelte'

  // Меню правого щелчка по группе ленты: виньетка (031) и удаление. Esc и щелчок мимо закрывают;
  // окно держится внутри экрана. Устройство — как у PhotoMenu. count > 1 — выделено несколько групп.
  // label — свой текст пункта удаления (плитка обложки: «Удалить обложку»); у неё пункта виньетки нет.
  let {
    x,
    y,
    count = 1,
    label,
    kind,
    onVignette,
    onNewVignette,
    onVary,
    varies = true,
    onDelete,
    onclose,
  }: {
    x: number
    y: number
    count?: number
    label?: string
    kind?: string // вид листа под курсором (SpreadView.kind)
    onVignette?: (act: 'make' | 'clear' | 'setup') => void
    onNewVignette?: () => void // плитка «+»: новая виньетка вместо пунктов группы
    onVary?: () => void // «Разнообразить раскладки» (041); нет — пункта нет (плитка обложки)
    varies?: boolean // среди целей есть разворот «Авто» с фото (anyVaries); нет — пункт серый
    onDelete?: () => void
    onclose: () => void
  } = $props()
  const vig = $derived(onVignette ? vignetteItem(count, kind, isSheet(store.view.settings), prefs.can('vignette')) : null)

  // Пункт показан, если функция есть на уровне; серым — когда менять нечего (сетки, виньетки, пустые).
  const showVary = $derived(!!onVary && !isSheet(store.view.settings) && prefs.can('variants'))
  const vary = $derived(canVary(varies, isSheet(store.view.settings), prefs.can('variants')))

  const setup = $derived(onVignette ? setupItem(count, kind, prefs.can('vignette')) : null)

  let box = $state<HTMLElement>()
  let w = $state(0)
  let h = $state(0)
  const pos = $derived(menuPos(x, y, { w, h }, { w: window.innerWidth, h: window.innerHeight }))

  // Фокус на пункт: Enter работает сразу.
  $effect(() => {
    box?.querySelector<HTMLElement>('button:not(:disabled)')?.focus()
  })

  function key(e: KeyboardEvent) {
    // Пока меню открыто, горячие клавиши приложения не работают (как в PhotoMenu).
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      onclose()
    }
  }

  function outside(e: PointerEvent) {
    if (box && !box.contains(e.target as Node)) onclose()
  }
</script>

<svelte:window onkeydowncapture={key} onpointerdowncapture={outside} onresize={onclose} onblur={onclose} />

<div
  bind:this={box}
  bind:clientWidth={w}
  bind:clientHeight={h}
  class="menu"
  style:left="{pos.x}px"
  style:top="{pos.y}px"
  style:visibility={h ? undefined : 'hidden'}
  role="menu"
  tabindex="-1"
  data-chrome
  data-text-ui
  oncontextmenu={(e) => e.preventDefault()}
>
  {#if setup}
    <button role="menuitem" onclick={() => onVignette?.(setup.act)}>
      <Icon name="vignette" size={16} stroke={1.8} /><span>{setup.label}</span>
    </button>
  {/if}
  {#if vig}
    <button role="menuitem" onclick={() => onVignette?.(vig.act)}>
      <Icon name="vignette" size={16} stroke={1.8} /><span>{vig.label}</span>
    </button>
  {/if}
  {#if onNewVignette}
    <button role="menuitem" onclick={onNewVignette}>
      <Icon name="vignette" size={16} stroke={1.8} /><span>Новая виньетка</span>
    </button>
  {/if}
  {#if showVary}
    <button role="menuitem" disabled={!vary} onclick={onVary}>
      <Icon name="shuffle" size={16} stroke={1.8} /><span>{varyLabel(count)}</span>
    </button>
  {/if}
  {#if onDelete}
    <button class="danger" role="menuitem" onclick={onDelete}>
      <Icon name="spread-delete" size={16} stroke={1.8} /><span>{label ?? deleteLabel(count, isSheet(store.view.settings))}</span>
    </button>
  {/if}
</div>

<style>
  .menu {
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
    padding: 0 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    text-align: left;
    white-space: nowrap;
    background: transparent;
    color: var(--text);
    border-radius: 4px;
  }
  button:disabled {
    opacity: 0.4;
    cursor: default;
  }
  button:hover:not(:disabled),
  button:focus-visible {
    background: #38383c;
    color: #fff;
  }
  button.danger:hover,
  button.danger:focus-visible {
    color: #ff8a8a;
  }
</style>
