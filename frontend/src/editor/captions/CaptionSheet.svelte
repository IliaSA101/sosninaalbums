<script lang="ts">
  import { edit } from '../text/edit.svelte'
  import type { SpreadView } from '../../base/core/types'
  import FieldMenu from './FieldMenu.svelte'
  import { fieldsOf, insertAt, type Field } from './fields'
  import { templateOf } from './sheetcaps'
  import { unitWords } from './unitword'
  import { store } from '../../base/core/store.svelte'

  // «Один стиль на лист» (спека 030): шрифт, кегль, цвет, выравнивание, «на фото» и отступ меняются разом
  // у всех подписей листа; свои стили не стираются, пока флажок снят. При включённом стиле — шаблон «Подпись
  // к каждому фото» (его получают фото без своей подписи) и «Ужимать длинные». Живёт в карточке подписи
  // и в карточке разворота (там — когда на листе одни заглушки и выделять нечего).
  let { sp }: { sp: SpreadView } = $props()

  const caps = $derived(sp.caps)
  const w = $derived(unitWords(store.view.settings, sp))
  // Включить можно, только если есть образец стиля — подпись на листе.
  const canOn = $derived(!!caps || sp.captions.length > 0)

  // Шаблон правится целиком и уходит в Go по уходу из поля (один шаг undo); пока поле в фокусе, ответы Go его не трогают.
  let draft = $state('')
  let focused = false
  let ta = $state<HTMLTextAreaElement>()
  $effect(() => {
    const t = caps?.text ?? ''
    if (!focused) draft = t
  })

  const commit = () => {
    focused = false
    const t = templateOf(draft)
    if (caps && t !== (caps.text ?? '')) edit.setSheetPart(sp.id, { text: t })
  }

  // Поле — в позицию курсора шаблона и сразу в Go: кнопка меню фокус не забирает.
  function insert(f: Field) {
    if (!ta) return
    const r = insertAt(draft, ta.selectionStart, ta.selectionEnd, f)
    draft = r.value
    edit.setSheetPart(sp.id, { text: templateOf(r.value) })
    queueMicrotask(() => ta?.setSelectionRange(r.caret, r.caret))
  }
</script>

<label
  class="chk"
  title={canOn ? `Шрифт, кегль, цвет, отступ и «на фото» — общие для всех подписей ${w.genThis}` : `${w.onCap} пока нет подписей`}
>
  <input type="checkbox" checked={!!caps} disabled={!canOn} onchange={(e) => edit.setSheet(sp.id, e.currentTarget.checked)} />
  Один стиль на {w.acc}
</label>
{#if caps}
  <div class="tpl">
    <span class="lbl">Подпись к каждому фото</span>
    <textarea
      bind:this={ta}
      bind:value={draft}
      rows="3"
      spellcheck="false"
      aria-label="Подпись к каждому фото"
      placeholder={'Например, {Фамилия}\n{Имя}'}
      onfocus={() => (focused = true)}
      onblur={commit}></textarea>
    <FieldMenu onpick={insert} fields={fieldsOf(sp)} label="Поле в шаблон" />
  </div>
  <label class="chk" title="Строка шире ячейки уменьшает кегль, но не ниже 6 пт; иначе переносится">
    <input type="checkbox" checked={!caps.wrap} onchange={(e) => edit.setSheetPart(sp.id, { wrap: !e.currentTarget.checked })} />
    Ужимать длинные
  </label>
{/if}

<style>
  .chk {
    margin-top: 8px;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .tpl {
    margin-top: 6px;
  }
  .lbl {
    display: block;
    font-size: 12px;
    color: var(--muted);
    margin-bottom: 4px;
  }
  textarea {
    box-sizing: border-box;
    width: 100%;
    resize: vertical;
    font-size: 12px;
    font-family: inherit;
  }
</style>
