<script lang="ts">
  // Черта с подписью внутри ряда плиток («другие», «мои», 050): подпись вертикально, высота — как у плитки.
  // key — метка для поиска черты снаружи (кнопка «Другие ▸» в шапке галереи прокручивает ряд к ней).
  let { label, title, key }: { label: string; title?: string; key?: string } = $props()
</script>

<div class="sep" {title} data-sep={key}><span>{label}</span></div>

<style>
  .sep {
    flex: none;
    height: 80px;
    display: flex;
    align-items: center;
    border-left: 1px solid var(--line);
    padding-left: 4px;
  }
  span {
    writing-mode: vertical-rl;
    transform: rotate(180deg);
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
