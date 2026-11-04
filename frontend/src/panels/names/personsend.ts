// Отложенная отправка имени (PersonFields): пауза набора, очередь вызовов Go и учёт в edit.idle().
// Вынесено из компонента, чтобы порядок «набрано → записано → смена альбома» проверялся тестом (087-9).
import { edit } from '../../editor/text/edit.svelte'

// Что отправить: считается в момент отправки (набор мог дойти до конца), null — отправлять нечего.
export type PersonJob = () => Promise<unknown>
export type PersonPrep = () => PersonJob | null

export class PersonSend {
  private timer = 0
  private prep: PersonPrep | null = null
  private queued: Promise<unknown> = Promise.resolve()

  // Есть отложенное, ещё не ушедшее в очередь.
  get waiting(): boolean {
    return this.prep !== null
  }

  // Отправить через delay после последнего вызова.
  schedule(prep: PersonPrep, delay: number) {
    clearTimeout(this.timer)
    this.prep = prep
    this.timer = window.setTimeout(() => this.flush(), delay)
  }

  // Отправить сразу: свой prep (уход из поля) или отложенный. Вызовы идут по очереди; edit.idle() их ждёт.
  flush(prep: PersonPrep | null = this.prep): Promise<unknown> {
    clearTimeout(this.timer)
    this.timer = 0
    this.prep = null
    const job = prep?.()
    if (job) {
      // Сбой прежнего вызова очередь не рвёт: store.run сам покажет ошибку.
      this.queued = this.queued.then(job, job)
      void edit.track(this.queued)
    }
    return this.queued
  }
}
