<script lang="ts">
  // Черта с подписью внутри ряда плиток («другие», «мои», 050): подпись вертикально, высота — как у плитки.
  let { label, title }: { label: string; title?: string } = $props()
</script>

<div class="sep" {title}><span>{label}</span></div>

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
