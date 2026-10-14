<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  // Плитка в ряду шаблонов (TplRow): карточка (children) и под ней имя своего шаблона; ✎ и × при наведении —
  // только если заданы onrename и ondelete. note — мелкая строка под именем (tone подсвечивает «мелко» и «не влезет»).
  let {
    name,
    note,
    tone,
    onrename,
    ondelete,
    children,
  }: {
    name?: string
    note?: string
    tone?: 'small' | 'over'
    onrename?: () => void
    ondelete?: () => void
    children: Snippet
  } = $props()
</script>

<div class="item" class:named={name !== undefined}>
  {@render children()}
  {#if onrename && ondelete}
    <div class="tools">
      <button type="button" onclick={onrename} title="Переименовать" aria-label="Переименовать шаблон">✎</button>
      <button type="button" onclick={ondelete} title="Удалить шаблон" aria-label="Удалить шаблон"><Icon name="x" size={12} /></button>
    </div>
  {/if}
  {#if name !== undefined}<div class="line1 name" title={name || undefined}>{name}</div>{/if}
  {#if note}<div class="line1 note {tone ?? ''}">{note}</div>{/if}
</div>

<style>
  .item {
    position: relative;
    flex: none;
  }
  .item.named {
    min-width: 72px; /* узкая карточка не сжимает имя: «Шаблон 1» и «Шаблон 1б» различимы */
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
  /* Строка не шире карточки: длинное имя обрезается, а не раздвигает плитку */
  .line1 {
    margin-top: 4px;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 11px;
    color: var(--muted);
    width: 0;
    min-width: 100%;
  }
  /* Имя — в две строки, обрезается третья; полное — в подсказке */
  .name {
    white-space: normal;
    overflow-wrap: anywhere;
    line-height: 1.25;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
  }
  .note {
    margin-top: 1px;
    font-size: 10px;
  }
  .note.small {
    color: var(--accent);
  }
  .note.over {
    color: #e0806f;
  }
</style>
