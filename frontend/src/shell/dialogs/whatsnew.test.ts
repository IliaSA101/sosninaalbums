import { describe, expect, it } from 'vitest'
import wails from '../../../../wails.json'
import { WHATS_NEW_VERSION } from './whatsnew'

describe('whatsnew', () => {
  // Страж: при поднятии версии в wails.json «Что нового» нужно обновить тоже.
  it('WHATS_NEW_VERSION совпадает с версией в wails.json', () => {
    expect(WHATS_NEW_VERSION).toBe(wails.info.productVersion)
  })
})
