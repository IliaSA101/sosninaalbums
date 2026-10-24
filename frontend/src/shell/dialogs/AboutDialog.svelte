<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import logo from '../../assets/logo.png'
  import UpdateBlock from '../update/UpdateBlock.svelte'
  import { aboutUi, BASED_ON, REPO_TEXT } from './about.svelte'
</script>

<!-- Устроено как «Готово» после выгрузки (PrintDone): Modal, ряд ghost-кнопок, primary «Закрыть» справа. -->
{#if aboutUi.open}
  <Modal title="О программе" onclose={() => aboutUi.close()} wide>
    <div class="head">
      <img class="logo" src={logo} alt="Sosnina Albums" draggable="false" />
      <div>
        <div class="name">Sosnina Albums {store.version}</div>
        <div class="muted">Вёрстка свадебных фотоальбомов</div>
      </div>
    </div>
    <p class="line">Автор — Илья Соснин</p>
    <p class="line">
      Вместе с фотографом Мариной Сосниной —
      <button class="link" onclick={() => void aboutUi.link('marina')}>Sosnina films</button>
    </p>
    <p class="line">{BASED_ON} · {REPO_TEXT}</p>
    <p class="line muted">
      Свободная программа: GNU GPL v3 с дополнительным условием п. 7(b) — изменённые версии сохраняют в этом окне строку выше и ссылку. Без
      каких-либо гарантий.
    </p>
    <div class="row">
      <button class="ghost" onclick={() => void aboutUi.link('source')}>Исходный код</button>
      <button class="ghost" onclick={() => void aboutUi.link('license')}>Текст лицензии</button>
    </div>

    <UpdateBlock />

    <div class="feedback">
      <!-- Две строки по макету 101: у каждой свой адресат — Telegram или трекер GitHub (шаблон bug / idea). -->
      <div class="fb" role="group" aria-label="Сообщить о проблеме">
        <span class="fbname">Сообщить о проблеме</span>
        <button class="ghost" onclick={() => void aboutUi.link('telegram')}>Telegram</button>
        <button class="ghost" onclick={() => void aboutUi.link('bug')}>GitHub</button>
      </div>
      <div class="fb" role="group" aria-label="Предложить идею">
        <span class="fbname">Предложить идею</span>
        <button class="ghost" onclick={() => void aboutUi.link('telegram')}>Telegram</button>
        <button class="ghost" onclick={() => void aboutUi.link('idea')}>GitHub</button>
      </div>
    </div>

    <div class="actions">
      <button class="primary" data-autofocus onclick={() => aboutUi.close()}>Закрыть</button>
    </div>
  </Modal>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 10px;
  }
  .logo {
    height: 48px;
  }
  .name {
    font-weight: 600;
    font-size: 15px;
  }
  .muted {
    color: var(--muted);
    font-size: 12px;
  }
  .line {
    margin: 0 0 8px;
    line-height: 1.5;
    user-select: text;
  }
  /* Ссылка-кнопка: как в окне наборов, но в размер строки */
  .link {
    all: unset;
    cursor: pointer;
    color: var(--muted);
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
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 6px;
  }
  .feedback {
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px solid var(--line);
  }
  .fb {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }
  .fbname {
    flex: 0 0 11em;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
