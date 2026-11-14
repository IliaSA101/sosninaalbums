<script lang="ts">
  import type { Snippet } from 'svelte'

  // Строка окна «Наборы» — и у стоящих, и у «Можно скачать»: название, строки подписи, описание в две строки, кнопка справа.
  // Всё — недоверенный текст из манифеста или каталога: только как текст (не {@html}).
  let { title, lines, about = '', children }: { title: string; lines: readonly string[]; about?: string; children?: Snippet } = $props()
</script>

<li>
  <div class="info">
    <span class="when">{title}</span>
    {#each lines.filter(Boolean) as line, i (i)}<span class="meta">{line}</span>{/each}
    <!-- Полный текст в title: в строке он обрезан line-clamp: 2 (design 129 §6) -->
    {#if about}<span class="meta about" title={about}>{about}</span>{/if}
  </div>
  {@render children?.()}
</li>

<style>
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 10px;
  }
  li:not(:first-child) {
    border-top: 1px solid var(--line);
  }
  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .when {
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .meta {
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  /* Описание из манифеста длинное: показываем две строки */
  .about {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
