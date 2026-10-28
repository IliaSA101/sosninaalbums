<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from '../ui/Icon.svelte'

  // Плитка в ряду шаблонов (TplRow): карточка (children) и под ней одна строка: имя своего шаблона (обрезается «…»)
  // и note через « · » (tone подсвечивает «мелко» и «не влезет»). Обе части обрезаются «…» внутри плитки, полное — в title;
  // узкой карточке с подписью дан свой минимум ширины (noted / lone). ✎ и × при наведении —
  // только если заданы onrename и ondelete.
  let {
    name,
    note,
    tone,
    onrename,
    ondelete,
    renameLabel = 'Переименовать шаблон',
    deleteTitle = 'Удалить шаблон',
    deleteLabel = 'Удалить шаблон',
    children,
  }: {
    name?: string
    note?: string
    tone?: 'small' | 'over'
    onrename?: () => void
    ondelete?: () => void
    renameLabel?: string // aria-label ✎; окно фонов (048) пишет «фон» вместо «шаблон»
    deleteTitle?: string // подсказка ×: у библиотеки фонов — «Убрать из библиотеки»
    deleteLabel?: string
    children: Snippet
  } = $props()

  const hasName = $derived(name !== undefined && name !== '')
  // Полное — в подсказке строки
  const full = $derived([name, note].filter(Boolean).join(' · ') || undefined)
</script>

<div class="item" class:named={name !== undefined} class:noted={hasName && !!note} class:lone={!hasName && !!note}>
  {@render children()}
  {#if onrename && ondelete}
    <div class="tools">
      <button type="button" onclick={onrename} title="Переименовать" aria-label={renameLabel}>✎</button>
      <button type="button" onclick={ondelete} title={deleteTitle} aria-label={deleteLabel}><Icon name="x" size={12} /></button>
    </div>
  {/if}
  {#if name !== undefined || note}
    <div class="line1" title={full}>
      {#if hasName}<span class="name">{name}</span>{/if}
      {#if hasName && note}<span class="dot">·</span>{/if}
      {#if note}<span class="note {tone ?? ''}">{note}</span>{/if}
    </div>
  {/if}
</div>

<style>
  .item {
    position: relative;
    flex: none;
  }
  .item.named {
    min-width: 72px; /* узкая карточка не сжимает имя: «Шаблон 1» и «Шаблон 1б» различимы */
  }
  /* Имя ≈ 52 px + « · » 11 + «23 мм · мелко» ≈ 62 → 128: на узком листе (карточка ≈ 54) и имя, и «мелко» видны */
  .item.noted {
    min-width: 128px;
  }
  .item.lone {
    min-width: 64px; /* встроенная плитка: «23 мм · мелко» не выходит за плитку целиком */
  }
  .tools {
    position: absolute;
    left: 2px;
    top: 2px;
    display: none;
    gap: 2px;
  }
  .item:hover .tools,
  .item:focus-within .tools {
    display: flex;
  }
  .tools button {
    width: 20px;
    height: 20px;
    padding: 0;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: rgba(17, 17, 17, 0.8);
  }
  /* Строка одна (13 px) и не шире карточки: длинное имя обрезается, а не раздвигает плитку */
  .line1 {
    display: flex;
    align-items: baseline;
    gap: 4px;
    margin-top: 3px;
    height: 13px;
    line-height: 13px;
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
    width: 0;
    min-width: 100%;
  }
  .name {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .dot {
    flex: none;
  }
  .note {
    flex: none; /* длинное имя режется первым, пометка (мм, «мелко», «не влезет») остаётся */
    font-size: 10px;
  }
  .note.small {
    color: var(--accent);
  }
  .note.over {
    color: #e0806f;
  }
</style>
