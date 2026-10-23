<script lang="ts">
  import { answerCopy, copyAsk, showWhere } from '../../base/drag/copyask.svelte'
  import { copyAskText } from '../../base/drag/copyask'
  import Modal from '../../base/modal/Modal.svelte'

  // Вопрос при броске на лист фото, уже стоящего в альбоме (098). Правки ещё не было: «Отмена», × и Esc — бросок
  // отменён, Go не зовём. Устроен как DropAskDialog (042): безопасный выбор слева и в фокусе, главный справа.
  // Строки «где стоит» — ссылки: переход на лист с выделенным местом (бросок при этом отменён).
  const a = $derived(copyAsk.cur)
  const t = $derived(a ? copyAskText(a.ids, a.used) : null)
</script>

{#if a && t}
  <Modal title={t.title} onclose={() => answerCopy('cancel')} width={540}>
    <p class="ask">{t.lead}</p>
    <ul class="where">
      {#each t.where as w, i (i)}
        <li><button type="button" class="link" onclick={() => showWhere(w)}>{w.link}</button> — {w.kind}</li>
      {/each}
      {#if t.more > 0}<li class="more">и ещё {t.more}</li>{/if}
    </ul>
    {#if t.rest}<p class="note">{t.rest}</p>{/if}
    {#if t.hint}<p class="note">{t.hint}</p>{/if}
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => answerCopy('cancel')}>Отмена</button>
      {#if t.move}<button class="ghost" onclick={() => answerCopy('move')}>Перенести сюда</button>{/if}
      <button class="primary" onclick={() => answerCopy('copy')}>Поставить копию</button>
    </div>
  </Modal>
{/if}

<style>
  .ask {
    margin: 4px 0 6px;
    line-height: 1.45;
    font-size: 14px;
  }
  .where {
    margin: 0 0 6px;
    padding-left: 18px;
    line-height: 1.5;
    font-size: 14px;
  }
  .more {
    list-style: none;
    margin-left: -18px;
    color: var(--muted);
  }
  .note {
    margin: 0 0 6px;
    font-size: 13px;
    color: var(--muted);
  }
  .link {
    all: unset;
    cursor: pointer;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration-style: solid;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
