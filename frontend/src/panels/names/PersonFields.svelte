<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { Api, store } from '../../base/core/store.svelte'
  import { sessionId } from '../../editor/text/textedit'
  import { personOf } from './names'

  // Имя на фото (спека 030) в карточке фото: Фамилия, Имя, Отчество, у учителя виньетки ещё Предмет (049). Пока своего имени нет, значения берутся
  // из имени файла и видны серым («из имени файла»); все три поля очищены — снова имя из файла. Набор уходит в Go
  // с паузой, одной сессией на фокус (один шаг отмены), и при смене фото досылается.
  // inline — окошко имени на холсте (037): без заголовка, показаны только поля из only, фокус — в первом,
  // Enter ведёт к следующему полю (на последнем — ondone), вся сессия набора — один шаг отмены.
  let { photo, only, inline = false, ondone }: { photo: string; only?: readonly string[]; inline?: boolean; ondone?: () => void } = $props()

  const DELAY = 500
  // k — ключ Person, w — имя поля в who шаблона (у предмета они разные: subj / subject).
  const ALL = [
    { k: 'last', w: 'last', label: 'Фамилия' },
    { k: 'first', w: 'first', label: 'Имя' },
    { k: 'middle', w: 'middle', label: 'Отчество' },
    { k: 'subj', w: 'subject', label: 'Предмет' },
  ] as const
  // Предмет в карточке — у учителя виньетки или если он уже задан; раз показанное поле не пропадает под курсором.
  let subjectShown = $state(false)
  const FIELDS = $derived(only ? ALL.filter((f) => only.includes(f.w)) : ALL.filter((f) => f.k !== 'subj' || subjectShown))

  let draft = $state({ last: '', first: '', middle: '', subj: '' })
  let fromFile = $state(false)
  let dirty = $state(false)
  let session = ''
  let root: HTMLElement | undefined
  let nSession = 0
  let seq = 0
  let timer = 0
  let queued: Promise<unknown> = Promise.resolve()
  let selectFirst = false // inline: после первой загрузки имени выделить текст в поле с фокусом

  // Действующее имя от Go: при смене фото и после любой правки. Свой набор (dirty) ответ не перебивает.
  $effect(() => {
    const id = photo
    void store.view.rev
    const n = ++seq
    Api.Person(id).then(
      (pv) => {
        if (n !== seq || dirty) return
        draft = { last: pv.last, first: pv.first, middle: pv.middle, subj: pv.subject }
        if (pv.teacher || pv.subject) subjectShown = true
        fromFile = pv.fromFile
        if (selectFirst) {
          selectFirst = false
          void tick().then(() => selectFocused())
        }
      },
      () => {},
    )
  })

  function send(id: string) {
    clearTimeout(timer)
    timer = 0
    if (!dirty) return
    // Имя ещё из файла, набран только предмет: имена не шлём — Go оставит имя из файла (решение 174).
    const p = personOf(fromFile ? { subj: draft.subj } : draft)
    const s = session
    dirty = false // дальше свежий ответ Go (имя из файла после очистки) можно показывать
    queued = queued.then(() => store.run(Api.SetPerson(id, p, s)))
  }

  // Карточка закрыта или фото сменилось (родитель пересоздаёт компонент по фото): набранное досылаем.
  $effect(() => {
    const id = photo
    return () => {
      if (timer) send(id)
    }
  })

  function focus() {
    if (inline) {
      if (!session) session = sessionId(Date.now(), ++nSession)
      return
    }
    if (!dirty) session = sessionId(Date.now(), ++nSession)
  }
  // Выделить весь текст в поле с фокусом: набор заменяет значение, а не дописывается к нему.
  function selectFocused() {
    const el = document.activeElement
    if (el instanceof HTMLInputElement && root?.contains(el)) el.select()
  }
  // Окошко на холсте открыто — курсор в первом поле. Значение приходит от Go позже (Api.Person), поэтому
  // выделение — после первого ответа; select() сейчас выделил бы пустое поле.
  onMount(() => {
    if (!inline) return
    root?.querySelector('input')?.focus()
    selectFirst = true
  })

  // Enter: следующее поле, на последнем — закрыть окошко.
  function key(e: KeyboardEvent) {
    if (!inline || e.key !== 'Enter') return
    e.preventDefault()
    const list = [...(root?.querySelectorAll('input') ?? [])]
    const next = list[list.indexOf(e.currentTarget as HTMLInputElement) + 1]
    if (next) {
      next.focus()
      next.select()
    } else ondone?.()
  }
  function input(k: (typeof ALL)[number]['k'], v: string) {
    selectFirst = false
    draft[k] = v
    dirty = true
    if (k !== 'subj') fromFile = false
    clearTimeout(timer)
    const id = photo
    timer = window.setTimeout(() => send(id), DELAY)
  }
</script>

<div bind:this={root} class="person" class:inline role="group" aria-label="Имя на фото" {...inline ? {} : { 'data-guide': 'person' }}>
  {#if !inline}<div class="cap">Имя на фото</div>{/if}
  {#each FIELDS as f (f.k)}
    <input
      type="text"
      class:file={fromFile && f.k !== 'subj'}
      maxlength="60"
      placeholder={f.label}
      aria-label={f.label}
      value={draft[f.k]}
      onfocus={focus}
      oninput={(e) => input(f.k, e.currentTarget.value)}
      onkeydown={key}
      onblur={() => send(photo)}
    />
  {/each}
  {#if fromFile}<div class="note">из имени файла</div>{/if}
</div>

<style>
  .person {
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
    display: grid;
    gap: 4px;
  }
  .person.inline {
    margin: 0;
    padding: 0;
    border: 0;
    grid-template-columns: minmax(0, 1fr);
  }
  /* Окошко на холсте: поля в стиле тёмных полей интерфейса, а не белые браузерные, и строго внутри плашки */
  .inline input {
    min-width: 0;
    height: 28px;
    padding: 0 8px;
    font: inherit;
    font-size: 12px;
    color: var(--text);
    background: #3a3a3e;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .inline input.file {
    color: var(--muted);
  }
  .inline input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .cap {
    font-size: 12px;
    font-weight: 600;
  }
  input {
    width: 100%;
    height: 24px;
    padding: 0 6px;
    box-sizing: border-box;
    font-size: 12px;
  }
  .file {
    color: var(--muted);
  }
  .note {
    font-size: 11px;
    color: var(--muted);
  }
</style>
