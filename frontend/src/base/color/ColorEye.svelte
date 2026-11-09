<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { eyeHint, type ColorWords } from './colorwords'

  // Кнопка «Взять цвет с фото» в окне ColorPicker (109 U4): под чертой, подсказка — по виду листа. on — режим пипетки идёт.
  let { on, words, onclick }: { on: boolean; words: ColorWords; onclick: () => void } = $props()
</script>

<div class="sep"></div>
<button type="button" class="eye" class:on aria-pressed={on} {onclick}>
  <Icon name="pipette" size={18} />
  <span class="txt">
    <b>Взять цвет с фото</b>
    <span>{on ? 'Щёлкните по развороту. Esc — отмена' : eyeHint(words)}</span>
  </span>
</button>

<style>
  .sep {
    height: 1px;
    margin: 10px 0 8px;
    background: var(--line);
  }
  .eye {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 34px;
    height: auto;
    padding: 6px 10px;
    text-align: left;
    white-space: normal;
  }
  .eye.on {
    border-color: var(--accent);
  }
  .eye b {
    display: block;
    font-weight: 600;
    font-size: 12px;
  }
  .eye .txt span {
    display: block;
    color: var(--muted);
    font-size: 11px;
    line-height: 1.3;
  }
</style>
