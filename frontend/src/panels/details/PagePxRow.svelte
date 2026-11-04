<script lang="ts">
  import { FILE_DPI, parsePx } from './presets'
  import type { PagePxView } from '../../base/core/types'
  import type { PxAsker } from './pagepx'

  // Строка «Файл разворота при 300 dpi, px: Ш × В» (123, design §1.3): ввод уходит в Go, страница в мм попадает в поля формы.
  // px/py — файл текущей страницы формы; чужой ввод (кнопка формата, Undo) обновляет поля. Ошибка Go — под полем.
  let {
    px,
    py,
    asker,
    onfill,
  }: {
    px: number
    py: number
    asker: PxAsker
    onfill: (r: PagePxView) => void
  } = $props()

  let wEl = $state<HTMLInputElement>()
  let hEl = $state<HTMLInputElement>()
  let err = $state('')
  // Поля пишем напрямую (как CoverPx): значение, введённое без input-события, не должно пережить возврат.
  function back(w = px, h = py) {
    if (wEl) wEl.value = String(w)
    if (hEl) hEl.value = String(h)
  }
  $effect(() => {
    back(px, py) // зависимость от px/py: чужой ввод (кнопка формата, Undo) обновляет поля
    err = ''
  })

  // Enter или уход из поля: мусор, ноль и то же число возвращают прежнее значение, как в карточке обложки.
  function commit() {
    const w = parsePx(wEl?.value ?? '')
    const h = parsePx(hEl?.value ?? '')
    if (w === null || h === null || (w === px && h === py)) {
      back()
      err = ''
      return
    }
    void asker.ask(
      w,
      h,
      (r) => {
        err = r.err
        if (!r.err) onfill(r)
      },
      (t) => (err = t),
    )
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur()
  }
</script>

<label data-guide="page-px" title="Размер файла разворота при выгрузке для типографии: программа подберёт страницу в мм"
  >Файл разворота при {FILE_DPI} dpi, px: Ш × В
  <span class="pair"
    ><input
      type="number"
      min="1"
      step="1"
      aria-label="Ширина файла разворота, px"
      value={px}
      bind:this={wEl}
      onchange={commit}
      onkeydown={key}
    />
    ×
    <input
      type="number"
      min="1"
      step="1"
      aria-label="Высота файла разворота, px"
      value={py}
      bind:this={hEl}
      onchange={commit}
      onkeydown={key}
    /></span
  ></label
>
{#if err}<p class="bad" role="alert">{err}</p>{/if}

<style>
  label {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
    font-size: 13px;
  }
  .pair {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .pair input[type='number'] {
    width: 70px;
  }
  .bad {
    margin: 4px 0 0;
    font-size: 12px;
    color: #e0806f;
  }
</style>
