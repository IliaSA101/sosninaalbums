<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { tplCall } from '../layouts/gridactions'
  import type { GridChoice, LayoutScope } from '../../base/core/types'
  import { BORDER_NOTE } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import Seg from '../../base/tpl/Seg.svelte'

  // Три окна «своих» шаблонов: «Сохранить как шаблон», «Переименовать шаблон», «Удалить шаблон?». Общие для галереи
  // разворота (GridGallery) и сторонки обложки (CoverGridGallery, 132): слова и порядок кнопок одни (правило «похожая функция»),
  // различаются входами: id листа, пункты «Область», подсказка и концовка текста удаления.
  let {
    spread,
    areas,
    hint,
    borders,
    applied,
    ondone,
  }: {
    spread: string // что отдаём в SaveGridTemplate: id разворота или 'cover'
    areas: { id: LayoutScope; label: string }[] // пункты «Область»; один — выбора нет
    hint: string // подсказка окна сохранения
    borders: (scope: LayoutScope) => boolean // у мест области есть рамочки — «Рамочки в шаблон не входят»
    applied: string // «Развороты» / «Листы» / «Обложка и развороты»: где шаблон уже применён и не изменится
    ondone: () => void // шаблоны изменились — галерея перезапрашивает ряд
  } = $props()

  // Имена в полях — отдельное состояние: окно закрывается (rename/save = null) раньше, чем bind:value
  // отпустит поле, и привязка к свойству null падала.
  let rename = $state<GridChoice | null>(null)
  let renameName = $state('')
  let del = $state<GridChoice | null>(null)
  let save = $state<{ scope: LayoutScope } | null>(null)
  let saveName = $state('')

  export function openSave(scope: LayoutScope) {
    saveName = ''
    save = { scope }
  }
  export function openRename(ch: GridChoice) {
    renameName = ch.name
    rename = ch
  }
  export function openDelete(ch: GridChoice) {
    del = ch
  }

  async function doRename() {
    const r = rename
    if (!r) return
    rename = null
    if (await tplCall(Api.RenameGridTemplate(r.id, renameName))) ondone()
  }
  async function doDelete() {
    const d = del
    if (!d) return
    del = null
    if (await tplCall(Api.DeleteGridTemplate(d.id))) ondone()
  }
  async function doSave() {
    const v = save
    if (!v) return
    save = null
    gallery.preview = null
    if (await tplCall(Api.SaveGridTemplate(spread, v.scope, saveName.trim()))) {
      store.notify('Шаблон сохранён')
      ondone()
    }
  }
  // Enter в поле имени — как кнопка окна.
  const enter = (fn: () => void) => (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      fn()
    }
  }
</script>

{#if rename}
  <Modal title="Переименовать шаблон" onclose={() => (rename = null)}>
    <input type="text" maxlength="60" data-autofocus bind:value={renameName} onkeydown={enter(doRename)} aria-label="Имя шаблона" />
    <div class="actions">
      <button class="ghost" onclick={() => (rename = null)}>Отмена</button>
      <button class="primary" onclick={doRename}>Сохранить</button>
    </div>
  </Modal>
{/if}

{#if del}
  <Modal title="Удалить шаблон?" onclose={() => (del = null)}>
    <p>Шаблон «{del.name}» будет удалён из списка. {applied}, где он уже применён, не изменятся.</p>
    <div class="actions">
      <button class="ghost" onclick={() => (del = null)}>Отмена</button>
      <button class="primary" onclick={doDelete}>Удалить</button>
    </div>
  </Modal>
{/if}

{#if save}
  <Modal title="Сохранить как шаблон" onclose={() => (save = null)}>
    <label class="field"
      >Название
      <input type="text" maxlength="60" data-autofocus placeholder="Шаблон N" bind:value={saveName} onkeydown={enter(doSave)} /></label
    >
    {#if areas.length > 1}
      <div class="field">
        Область
        <div class="segwrap">
          <Seg items={areas} value={save.scope} onpick={(v) => save && (save.scope = v)} label="Область шаблона" />
        </div>
      </div>
    {/if}
    <p class="hint">{hint}</p>
    {#if borders(save.scope)}<p class="hint">{BORDER_NOTE}</p>{/if}
    <div class="actions">
      <button class="ghost" onclick={() => (save = null)}>Отмена</button>
      <button class="primary" onclick={doSave}>Сохранить</button>
    </div>
  </Modal>
{/if}

<style>
  input[type='text'] {
    width: 100%;
    height: 30px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 10px;
    color: var(--muted);
    font-size: 12px;
  }
  .segwrap {
    align-self: flex-start;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
