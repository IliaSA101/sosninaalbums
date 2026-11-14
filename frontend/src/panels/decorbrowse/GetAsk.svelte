<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { ASK_CANCEL, ASK_TITLE, askOk, getQuestion, type GetItem, type GetKind } from './decorget'

  // Вопрос «Скачать набор?» (144) перед скачиванием образца: состав набора, размер, «Вместе с ним» и что встанет.
  // Образец — «Поставить разворот?» окна «Фоны»: «Отмена» слева, действие справа и в фокусе. Одно окно на «Фоны» и «Макеты альбома».
  // Название набора — недоверенный текст каталога: только текстом.
  let { tile, kind, onconfirm, oncancel }: { tile: GetItem; kind: GetKind; onconfirm: () => void; oncancel: () => void } = $props()
</script>

<Modal title={ASK_TITLE} onclose={oncancel}>
  <p>{getQuestion(tile.get, kind)}</p>
  <div class="actions">
    <button class="ghost" onclick={oncancel}>{ASK_CANCEL}</button>
    <button class="primary" data-autofocus onclick={onconfirm}>{askOk(kind)}</button>
  </div>
</Modal>

<style>
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
