<script lang="ts">
  // Полоса хода с подписью и «Отменить» (067 Ф3, 140): одна на загрузку набора (PackRun) и ролика (VideoWindow),
  // вид общий. text — готовая строка хода, percent — доля 0…100 или null (неопределённая полоса).
  let {
    text,
    percent,
    label,
    cancelling = false,
    autofocus = false,
    oncancel,
  }: {
    text: string
    percent: number | null
    label: string
    cancelling?: boolean
    autofocus?: boolean
    oncancel: () => void
  } = $props()
</script>

<div class="run">
  <div class="line">
    <span role="status">{cancelling ? 'Отменяем…' : text}</span>
    <span class="grow"></span>
    <button class="ghost" data-autofocus={autofocus ? '' : undefined} disabled={cancelling} onclick={oncancel}>Отменить</button>
  </div>
  <progress max="100" value={percent ?? undefined} aria-label={label}></progress>
</div>

<style>
  .run {
    flex: none;
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel2);
  }
  .line {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 6px;
    font-variant-numeric: tabular-nums;
  }
  .grow {
    flex: 1;
  }
  progress {
    width: 100%;
    height: 6px;
    accent-color: var(--accent);
  }
</style>
