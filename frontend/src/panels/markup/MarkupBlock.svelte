<script lang="ts">
  import type { GuideView } from '../../base/core/types'
  import { markupUi } from '../../editor/markup/markupui.svelte'
  import { canClear, type MarkupKind } from '../../editor/markup/markup'
  import { markupAct } from './markupact.svelte'

  // Блок «Разметка типографии» (032, 066): файл, предупреждения Go, «Выбрать файл…» · «Убрать», прозрачность.
  // Один на карточку обложки и на «Настройки альбома» (развороты); применяется сразу, один шаг отмены.
  let { kind, guide }: { kind: MarkupKind; guide: GuideView | null } = $props()

  // Ползунок нужен, когда есть что показывать: картинка или хотя бы линии.
  const shown = $derived(!!guide && guide.ok && (guide.image || !!guide.lines?.length))
</script>

<div class="mk">
  <div class="cap">Разметка типографии</div>
  {#if guide}
    <div class="file" title={guide.name}>{guide.name}</div>
    {#if !guide.ok}
      <div class="warn">Файл разметки не найден</div>
    {:else}
      {#if guide.warn}<div class="warn">{guide.warn}</div>{/if}
      {#if guide.note}<div class="warn">{guide.note}</div>{/if}
    {/if}
  {/if}
  <!-- якоря справочника: у обложки и у настроек альбома свои (литералами, динамический запрещён) -->
  <div class="btns" {...kind === 'cover' ? { 'data-guide': 'cover-markup' } : { 'data-guide': 'spread-markup' }}>
    <button onclick={() => markupAct.pick(kind)}>Выбрать файл…</button>
    <!-- как у фона и узора: «Убрать» только когда есть что убирать (файл выбран, даже если пропал с диска) -->
    {#if canClear(guide)}<button class="ghost" onclick={() => markupAct.clear(kind)}>Убрать</button>{/if}
  </div>
  {#if shown}
    <label class="fade" title="Прозрачность разметки на экране; в файл она не попадает">
      <span class="lbl">Прозрачность</span>
      <input
        type="range"
        min="5"
        max="100"
        step="1"
        value={Math.round(markupUi.opacity[kind] * 100)}
        aria-label="Прозрачность разметки, %"
        oninput={(e) => markupUi.setOpacity(kind, +e.currentTarget.value / 100)}
      />
    </label>
  {/if}
</div>

<style>
  .mk {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 8px;
    font-size: 12px;
  }
  .cap {
    font-weight: 600;
  }
  .file {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
  }
  .warn {
    color: #e0806f;
  }
  .btns {
    display: flex;
    flex-wrap: wrap; /* узкая колонка: кнопки переносятся, а не сжимаются */
    gap: 6px;
  }
  .btns button {
    flex: 1 0 auto;
    white-space: nowrap;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .fade {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .lbl {
    flex: 1;
    color: var(--muted);
  }
  .fade input {
    width: 90px;
  }
</style>
