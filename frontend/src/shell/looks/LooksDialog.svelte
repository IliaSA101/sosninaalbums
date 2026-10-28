<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { COVER_ID } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import { isTextField } from '../menu/keys'
  import { NAME_MAX, sideItems } from '../../panels/decor/decor'
  import { packsUi } from '../packs/packs.svelte'
  import PackRun from '../packs/PackRun.svelte'
  import GetAsk from '../../panels/decorbrowse/GetAsk.svelte'
  import { confirmLookGet } from './lookget.svelte'
  import { ASK_TITLE, EMPTY_DEMO, EMPTY_LOOKS, LOOKS_TITLE, MORE_LOOKS, applyQuestion, hideText, stepAskTitle } from './looks'
  import { looks } from './looks.svelte'
  import LookCards from './LookCards.svelte'
  import LookForm from './LookForm.svelte'
  import LookPreview from './LookPreview.svelte'

  // Окно «Макеты альбома» (131, U1): темы слева, плитки, справа предпросмотр выбранного макета и форма «Применить».
  // Образец — «Развороты» (DecorDialog); отличие — щелчок по плитке ничего не ставит (0.27-131-12).
  const side = $derived(sideItems(looks.cat))
  const none = $derived(!!looks.cat && looks.cat.items.length === 0 && !looks.cat.get?.length && !looks.loading)

  // Правка альбома под открытым окном (формат в панели деталей, событие Go): числа и план пересчитать.
  $effect(() => {
    const rev = store.view.rev
    untrack(() => looks.sync(rev))
  })

  // ←/→ листают шаги предпросмотра; в поле числа стрелки остаются каретке.
  function keys(e: KeyboardEvent) {
    if (looks.ask || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return
    if (isTextField(e.target as Element | null)) return
    if (e.key === 'ArrowLeft') looks.go(-1)
    else if (e.key === 'ArrowRight') looks.go(1)
    else return
    e.preventDefault()
  }

  function enter(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      void looks.commit()
    }
  }
  const selectAll = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select()
</script>

{#if looks.open}
  <Modal title={LOOKS_TITLE} size="large" onclose={() => looks.close()}>
    {#if none}
      <div class="none">
        {looks.demoOffer ? EMPTY_DEMO : EMPTY_LOOKS} ·
        <button type="button" class="link" onclick={() => packsUi.show()}>Наборы…</button>
      </div>
    {:else}
      <!-- Фокус по умолчанию — на теле окна: ←/→ листают шаги, пока фокус внутри (обработчик на элементе, не на window).
           Тело не виджет (клавиши нужны только как ускоритель к кнопкам ‹ ›), поэтому предупреждение a11y снято -->
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <div class="body" role="group" aria-label="Макеты" tabindex="-1" data-autofocus onkeydown={keys}>
        <div class="side">
          <SideList items={side} value={looks.theme} onpick={(id) => looks.setTheme(id)} label="Темы" scroll />
        </div>
        <div class="main">
          <div class="bar">
            <span class="grow"></span>
            <!-- 067 Ф2: те же наборы, что в окнах «Фоны» и «Развороты»; окно открывается поверх -->
            <button class="ghost" title="Наборы библиотеки: добавить из файла, удалить" onclick={() => packsUi.show()}>Наборы…</button>
          </div>
          <PackRun net={packsUi.net} />
          <LookCards />
          {#if looks.cat?.demoOnly}
            <!-- Все макеты окна — из примера (142): дальше их приносят наборы; образец — пустое состояние -->
            <div class="more">
              {MORE_LOOKS} ·
              <button type="button" class="link" onclick={() => packsUi.show()}>Наборы…</button>
            </div>
          {/if}
        </div>
        <div class="right">
          <LookPreview />
          <LookForm />
        </div>
      </div>
    {/if}
  </Modal>
{/if}

{#if looks.open && looks.ask?.kind === 'rename'}
  <Modal title="Переименовать макет" onclose={() => looks.dismiss()}>
    <input type="text" maxlength={NAME_MAX} data-autofocus bind:value={looks.name} onkeydown={enter} onfocus={selectAll} aria-label="Имя" />
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => looks.commit()}>Сохранить</button>
    </div>
  </Modal>
{:else if looks.open && looks.ask?.kind === 'hide'}
  <Modal title="Убрать из библиотеки?" onclose={() => looks.dismiss()}>
    <p>{hideText(looks.ask.name)}</p>
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => looks.commit()}>Убрать</button>
    </div>
  </Modal>
{:else if looks.open && looks.ask?.kind === 'apply'}
  <!-- Вопрос 067: заменится своё или фото уйдут в ленту; «Отмена» возвращает в окно макетов, альбом не тронут -->
  <Modal title={ASK_TITLE} onclose={() => looks.dismiss()}>
    <p>{applyQuestion(looks.ask.plan)}</p>
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => looks.confirm()}>Применить</button>
    </div>
  </Modal>
{:else if looks.open && looks.ask?.kind === 'get'}
  <!-- 144: образец макета из нескачанного набора; «Отмена» возвращает к плиткам -->
  <GetAsk tile={looks.ask.tile} kind="album" onconfirm={() => confirmLookGet()} oncancel={() => looks.dismiss()} />
{:else if looks.open && looks.ask?.kind === 'step'}
  <!-- «Поставить на разворот N»: тот же вопрос, что у «Развороты» (067) -->
  <Modal title={stepAskTitle(looks.ask.spread === COVER_ID)} onclose={() => looks.dismiss()}>
    <p>{looks.ask.text}</p>
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => looks.confirmStep()}>Поставить</button>
    </div>
  </Modal>
{/if}

<style>
  /* Высота задана окну, а не содержимому: темы и плитки прокручиваются каждые сами */
  .body {
    outline: none;
    display: flex;
    gap: 12px;
    height: min(560px, calc(100vh - 150px));
  }
  .side {
    flex: none;
    width: 150px;
    min-height: 0;
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .right {
    flex: none;
    width: 340px;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
  }
  .grow {
    flex: 1;
  }
  .none {
    padding: 40px 0;
    text-align: center;
    color: var(--muted);
    font-size: 13px;
  }
  .more {
    flex: none;
    color: var(--muted);
    font-size: 12px;
    text-align: center;
  }
  .link {
    all: unset;
    color: var(--accent);
    cursor: pointer;
    text-decoration: underline;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
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
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
