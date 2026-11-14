// Прокладка к Go для подборок (083, design §3): вызовы NewBin/RenameBin/DeleteBin/SetBin из wailsjs, вид приводится к View
// (так же, как в store). Тесты подменяют модуль целиком.
import * as Api from '../../../wailsjs/go/app/App'
import type { View } from '../../base/core/types'

// Новая подборка в конце View.bins + фото ids в ней, один шаг. Ошибки: «Подборок не больше 50», «Такая подборка уже есть».
export const NewBin = (name: string, ids: string[]): Promise<View> => Api.NewBin(name, ids) as unknown as Promise<View>
// Имя; пустое или то же — холостая. Ошибки: «Такая подборка уже есть», «Подборки больше нет».
export const RenameBin = (id: string, name: string): Promise<View> => Api.RenameBin(id, name) as unknown as Promise<View>
// Подборка и её членство у всех фото, один шаг.
export const DeleteBin = (id: string): Promise<View> => Api.DeleteBin(id) as unknown as Promise<View>
// in — добавить ids в подборку, иначе убрать; фото вне ленты Go пропускает.
export const SetBin = (id: string, ids: string[], inBin: boolean): Promise<View> => Api.SetBin(id, ids, inBin) as unknown as Promise<View>
