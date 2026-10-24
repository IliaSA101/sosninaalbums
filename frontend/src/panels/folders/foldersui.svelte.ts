// Окно «Папки» (093, отдельные фото — 097): открыто ли, вопрос перед отвязкой, подсветка строки после добавления. Отдельно
// от FolderMenu: кнопка может уйти в «⋯» и смонтироваться заново, а событие Go и вопрос должны пережить это.
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { store, Api } from '../../base/core/store.svelte'
import type { LooseAdded, View } from '../../base/core/types'
import { errText } from '../../base/words/errtext'
import { folderfilter } from './folderfilter.svelte'
import { GROUP_KEY, batchKey, cachedLoose, looseAskText, looseInAlbum, looseToast } from './loose'
import {
  addedCase,
  addedText,
  ancestors,
  cachedTree,
  findNode,
  inAlbum,
  nodeKey,
  removeText,
  type FolderAdded,
  type TreeNode,
} from './tree'

// Сколько мигает строка (как в макете владельца).
export const FLASH_MS = 2400
// После ответа ChooseFolder событие ещё может быть в пути: столько ждём его, чтобы открыть окно, из которого начали.
const LATE_MS = 1000

export interface RemoveAskState {
  node: TreeNode
  title: string // «Убрать папку?» | «Убрать фото?»
  text: string
}

class FoldersUi {
  open = $state(false)
  ask = $state<RemoveAskState | null>(null)
  flash = $state<string | null>(null)
  private fromMenu = false
  private flashTimer = 0
  private lateTimer = 0
  private readonly tree = cachedTree()
  private readonly loose = cachedLoose()
  private last: { folders: TreeNode[]; group: TreeNode | null; out: TreeNode[] } | null = null

  // Дерево по текущим папкам и фото, ветка «Фото отдельно» — последней (с памятью: окно и обработчики зовут его часто).
  forest(): TreeNode[] {
    const folders = this.tree(store.view.folders, store.photos, store.view.loose ?? [])
    const group = this.loose(store.view.loose ?? [], store.byId, store.unread, store.scan.active)
    const l = this.last
    if (l && l.folders === folders && l.group === group) return l.out
    const out = group ? [...folders, group] : folders
    this.last = { folders, group, out }
    return out
  }

  listen(): () => void {
    const offFolders = EventsOn('folders:added', (e: FolderAdded) => e && this.added(e))
    const offLoose = EventsOn('loose:added', (e: LooseAdded) => e && this.looseAdded(e, false))
    return () => {
      offFolders()
      offLoose()
    }
  }

  // «+ Добавить папку…» из окна: окно закрывается под системный выбор и откроется на добавленной.
  async add() {
    this.open = false
    this.fromMenu = true
    window.clearTimeout(this.lateTimer)
    await store.run(Api.ChooseFolder())
    this.lateTimer = window.setTimeout(() => (this.fromMenu = false), LATE_MS)
  }

  // «+ Добавить фото…» (окно, пустая лента): итог приходит ответом, а не событием — окно откроется на добавлении.
  async addFiles() {
    this.open = false
    try {
      const r = await Api.ChooseFiles()
      if (await store.take(r.view as unknown as View)) this.looseAdded(r, true)
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }

  // Итог добавления отдельных фото: ветка и новое добавление раскрыты; из кнопки — окно открыто на нём и строка мигает.
  // Бросок в окно (событие) окна не открывает — как бросок новой папки.
  looseAdded(r: LooseAdded, reveal: boolean) {
    if (r.batch) {
      folderfilter.openKey(GROUP_KEY, true)
      folderfilter.openKey(batchKey(r.batch), false)
      if (reveal) {
        this.open = true
        this.blink(batchKey(r.batch))
      }
    }
    const t = looseToast(r)
    if (t) store.notify(t.text, t.kind)
  }

  // Событие Go после AddFolderPath/ChooseFolder/броска папки. Окно открывается само, если папка уже была
  // (подпапка или та же) или добавление начато из окна; бросок новой папки окна не открывает, как раньше.
  added(e: FolderAdded) {
    const c = addedCase(e)
    const reveal = c !== 'new' || this.fromMenu
    this.fromMenu = false
    window.clearTimeout(this.lateTimer)
    const key = nodeKey(c === 'same' ? e.within : e.dir)
    const node = findNode(this.forest(), key)
    // Подпапки без фото в дереве нет — подсвечиваем её папку, но «отмечена» и «Только её» не обещаем.
    const exact = node?.key === key
    if (reveal) {
      if (node) folderfilter.expand(ancestors(node))
      if (c === 'new' && node) folderfilter.expand([node])
      this.open = true
      this.blink(node?.key ?? key)
    }
    const text = addedText(e, c !== 'inside' || exact)
    if (!text) return
    if (c === 'inside' && exact) store.notify(text, 'info', { label: 'Только её', run: () => this.onlyKey(key) })
    else store.notify(text, 'info')
  }

  private onlyKey(key: string) {
    const forest = this.forest()
    const n = findNode(forest, key)
    if (n?.key === key) folderfilter.only(forest, n)
  }

  blink(key: string) {
    this.flash = key
    window.clearTimeout(this.flashTimer)
    this.flashTimer = window.setTimeout(() => (this.flash = null), FLASH_MS)
  }

  // ✕ у добавленной папки, добавления или файла: их фото стоят в альбоме — сначала вопрос, иначе убрать сразу.
  remove(node: TreeNode) {
    const used = new Set([...store.used, ...store.backdropUsed])
    if (node.loose) {
      const n = looseInAlbum(node, used)
      if (n === 0) return void this.drop(node)
      this.ask = { node, title: 'Убрать фото?', text: looseAskText(node, n) }
      return
    }
    const n = inAlbum(node, store.photos, used)
    if (n === 0) return void this.drop(node)
    this.ask = { node, title: 'Убрать папку?', text: removeText(node, n) }
  }

  private drop(node: TreeNode) {
    const l = node.loose
    return store.run(l ? Api.RemoveLoose(l.id, l.file) : Api.RemoveFolder(node.path))
  }

  async confirm() {
    const a = this.ask
    this.ask = null
    if (a) await this.drop(a.node)
  }

  cancel() {
    this.ask = null
  }
}

export const foldersUi = new FoldersUi()
