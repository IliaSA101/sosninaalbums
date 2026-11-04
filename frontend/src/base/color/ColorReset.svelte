<script lang="ts">
  // Крестик сброса справа от образца цвета (спека 135): у фона разворота («Как в альбоме») и у цвета страниц
  // альбома в настройках. Не скрывается, а гаснет, когда сбрасывать нечего. onhover — показ исходного цвета
  // на время наведения/фокуса; после щелчка показ снимаем сами: погасшая кнопка pointerleave может не дать.
  let {
    label,
    title,
    disabled = false,
    onclick,
    onhover,
  }: {
    label: string
    title: string
    disabled?: boolean
    onclick: () => void
    onhover?: (on: boolean) => void
  } = $props()

  function click() {
    onclick()
    onhover?.(false)
  }
</script>

<button
  class="reset"
  {disabled}
  aria-label={label}
  {title}
  onclick={click}
  onpointerenter={() => onhover?.(true)}
  onpointerleave={() => onhover?.(false)}
  onfocus={() => onhover?.(true)}
  onblur={() => onhover?.(false)}>×</button
>

<style>
  .reset {
    width: 22px;
    height: 24px;
    padding: 0;
    font-size: 14px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
</style>
