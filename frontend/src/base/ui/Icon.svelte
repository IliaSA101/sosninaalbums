<script lang="ts" module>
  // Значки рисуются в квадрате 24×24 с центром в (12, 12): у символов шрифта
  // (× ‹ › ✓) свои отступы, и в круглой кнопке они съезжают.
  const ICONS = {
    x: { d: 'M6.5 6.5l11 11M17.5 6.5l-11 11' },
    'chevron-left': { d: 'M15.25 5.5L8.75 12l6.5 6.5' },
    'chevron-right': { d: 'M8.75 5.5L15.25 12l-6.5 6.5' },
    check: { d: 'M5 12l4.5 5L19 7' },
    plus: { d: 'M12 5v14M5 12h14' },
    // Панель ленты (024): папки, раскрытие окна, «⋯», флажки и «как в Lightroom».
    folder: { d: 'M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2.5h8.5A1.5 1.5 0 0 1 21 9v9.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5z' },
    'caret-down': { d: 'M6 9h12l-6 7z', fill: true },
    more: { d: 'M5 10.5h3v3H5zM10.5 10.5h3v3h-3zM16 10.5h3v3h-3z', fill: true },
    flag: { d: 'M6 21V4M6 4h11l-2.5 4 2.5 4H6' },
    ban: { d: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM5.6 5.6l12.8 12.8' },
    revert: { d: 'M9 14L4 9l5-5M4 9h10.5a5.5 5.5 0 0 1 0 11H11' },
    // Залитые: сетка раскладок и треугольники листания раскладок.
    grid: { d: 'M3 3h8v8H3zM13 3h8v8h-8zM3 13h8v8H3zM13 13h8v8h-8z', fill: true },
    'caret-left': { d: 'M16.5 5v14L7.5 12z', fill: true },
    'caret-right': { d: 'M7.5 5v14l9-7z', fill: true },
    // Булавка — «закрепить сетку».
    pin: { d: 'M9 4h6l-1 6 3 3v1.5H7V13l3-3zM12 14.5V21' },
    trash: { d: 'M4 7h16M9.5 7V4.5h5V7M6.5 7l1 12.5h9l1-12.5M10 11v5M14 11v5' },
    // Фото с минусом — «убрать фото из альбома».
    'photo-minus': {
      d: 'M13 5H4.5A1.5 1.5 0 0 0 3 6.5v11A1.5 1.5 0 0 0 4.5 19h13a1.5 1.5 0 0 0 1.5-1.5V12M3 16l5-4.5 4 3.5 2.5-2 4.5 3.5M16 6.5h6',
    },
    // Разворот (две страницы) с крестиком — «удалить разворот».
    'spread-delete': { d: 'M12 19H3V5h18v7M12 5v14M15.5 15.5l5 5M20.5 15.5l-5 5' },
    // Лента и карточка правки (спека 008, panels.md).
    text: { d: 'M5 5h14M12 5v14M9 19h6' },
    'place-add': { d: 'M13 5H5a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-7M18 3v6M15 6h6' },
    'split-h': { d: 'M4 5h16v14H4zM12 5v14' },
    'split-v': { d: 'M4 5h16v14H4zM4 12h16' },
    'same-size': { d: 'M3 7h8v10H3zM13 7h8v10h-8z' },
    'fit-photo': { d: 'M4 8V4h4M16 4h4v4M20 16v4h-4M8 20H4v-4M8 9h8v6H8z' },
    // Полоса выравнивания (010 U7): черта — край или ось опоры, прямоугольники — объекты.
    'al-left': { d: 'M4 3v18M8 6h11v4H8zM8 14h6v4H8z' },
    'al-hcenter': { d: 'M12 3v18M6 6h12v4H6zM8.5 14h7v4h-7z' },
    'al-right': { d: 'M20 3v18M5 6h11v4H5zM10 14h6v4h-6z' },
    'al-top': { d: 'M3 4h18M6 8h4v11H6zM14 8h4v6h-4z' },
    'al-vcenter': { d: 'M3 12h18M6 6h4v12H6zM14 8.5h4v7h-4z' },
    'al-bottom': { d: 'M3 20h18M6 5h4v11H6zM14 10h4v6h-4z' },
    'dist-h': { d: 'M3 3v18M21 3v18M7 7h3v10H7zM14 9h3v6h-3z' },
    'dist-v': { d: 'M3 3h18M3 21h18M7 7h10v3H7zM9 14h6v3H9z' },
    front: { d: 'M12 19V8M7 13l5-5 5 5M5 4h14' },
    back: { d: 'M12 5v11M7 11l5 5 5-5M5 20h14' },
    mirror: { d: 'M12 3v18M9 7L4 17h5zM15 7l5 10h-5z' },
    // «Отразить раскладку»: разворот с осью, блок вверху слева и внизу справа — раскладка перевёрнута.
    'mirror-layout': { d: 'M3 5h18v14H3zM12 5v14M5.5 7.5h3.5v4H5.5zM15 12.5h3.5v4H15z' },
    // Мини-панель (009 U1): зеркало фото, «Целиком», слои, обмен местами.
    'flip-h': { d: 'M12 3v18M9 7L4 17h5zM15 7l5 10h-5z' },
    'flip-v': { d: 'M3 12h18M7 9h10l-5-5zM7 15h10l-5 5z' },
    contain: { d: 'M3 5h18v14H3zM8 9h8v6H8z' },
    layers: { d: 'M12 4l8 4-8 4-8-4zM4 12l8 4 8-4M4 16l8 4 8-4' },
    swap: { d: 'M4 8h14M14 4l4 4-4 4M20 16H6M10 12l-4 4 4 4' },
    lock: { d: 'M8 11V8a4 4 0 0 1 8 0v3M6 11h12v9H6z' },
    magnet: { d: 'M5 4h4v8a3 3 0 0 0 6 0V4h4v8a7 7 0 0 1-14 0zM5 8h4M15 8h4' },
    caption: { d: 'M4 4h16v10H4zM6 18h12M8 21h8' },
    // Поворот фото на 90° (мини-панель, меню, карточка кадра).
    'turn-cw': { d: 'M4.05 11a8 8 0 1 1 .5 4m-.5 5v-5h5' },
    'turn-ccw': { d: 'M19.95 11a8 8 0 1 0-.5 4m.5 5v-5h-5' },
    reset: { d: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM9 9l6 6M15 9l-6 6' },
    'align-left': { d: 'M4 6h16M4 10h10M4 14h16M4 18h10' },
    'align-center': { d: 'M4 6h16M7 10h10M4 14h16M7 18h10' },
    'align-right': { d: 'M4 6h16M10 10h10M4 14h16M10 18h10' },
    // Порядок слоёв: закрашен тот лист, что двигается; «на передний/задний план» — с чертой у края.
    'layer-front': { d: 'M5 4h14M11 17H4V8h9v4', f: 'M11 12h9v9h-9z' },
    'layer-up': { d: 'M14 10V4H4v10h6', f: 'M10 10h10v10H10z' },
    'layer-down': { d: 'M10 10h10v10H10z', f: 'M4 4h10v6h-4v4H4z' },
    'layer-back': { d: 'M5 20h14M11 7h9v9h-9z', f: 'M4 3h9v4h-2v5H4z' },
    // «Отразить»: разворот с осью; закрашена страница, которую отражаем.
    'mirror-left': { d: 'M3 5h18v14H3zM12 3v18', f: 'M3 5h9v14H3z' },
    'mirror-right': { d: 'M3 5h18v14H3zM12 3v18', f: 'M12 5h9v14h-9z' },
    'mirror-spread': { d: 'M3 5h18v14H3zM12 3v18', f: 'M9.5 9.5l-3 2.5 3 2.5zM14.5 9.5l3 2.5-3 2.5z' },
  } satisfies Record<string, { d: string; f?: string; fill?: boolean }>

  export type IconName = keyof typeof ICONS
</script>

<script lang="ts">
  let { name, size = 16, stroke = 2 }: { name: IconName; size?: number; stroke?: number } = $props()
  const icon = $derived<{ d: string; f?: string; fill?: boolean }>(ICONS[name])
</script>

<svg
  class="icon"
  viewBox="0 0 24 24"
  width={size}
  height={size}
  aria-hidden="true"
  fill={icon.fill ? 'currentColor' : 'none'}
  stroke={icon.fill ? 'none' : 'currentColor'}
  stroke-width={stroke}
  stroke-linecap="round"
  stroke-linejoin="round"
  >{#if icon.f}<path d={icon.f} fill="currentColor" />{/if}<path d={icon.d} /></svg
>

<style>
  .icon {
    display: block;
    flex: none;
    pointer-events: none;
  }
</style>
