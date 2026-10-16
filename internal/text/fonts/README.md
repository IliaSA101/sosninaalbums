# Встроенные шрифты

Все — SIL Open Font License 1.1 (тексты лицензий — `OFL-<id>.txt` рядом, по одному на гарнитуру; `id` — из `families.go`). Шрифты вшиты в программу: текст на экране и в экспорте рисуется одними и теми же файлами, на другой компьютер переносить ничего не нужно.

Число гарнитур и файлов — в `families.go` и в заголовке раздела 3 `THIRD_PARTY_NOTICES.txt` (считает скрипт). Только статические TTF, по файлу на начертание: `x/image/font/sfnt` вариаций не применяет, а браузер подставил бы ось `wght`, и жирный на экране и в экспорте разошёлся бы (тест `TestFontsStatic` ищет `fvar`). Наборы начертаний — 1, 2 (обычное + жирное) или 4.

| Гарнитура | Группа | Файлы | Источник | RFN | Изменения |
| --- | --- | --- | --- | --- | --- |
| Bad Script | script | `BadScript-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Caveat | script | `Caveat-*.ttf` (R, B) | fonts.google.com, `static/` | нет | урезан набор знаков |
| Comforter | script | `Comforter-Regular.ttf` | github.com/google/fonts, `ofl/comforter` | нет | урезан набор знаков |
| Great Vibes | script | `GreatVibes-Regular.ttf` | fonts.google.com | нет | урезан набор знаков, кернинг кириллицы в latn |
| Lobster | script | `Lobster-Regular.ttf` | github.com/google/fonts, `ofl/lobster` | «Lobster» | без изменений |
| Marck Script | script | `MarckScript-Regular.ttf` | github.com/google/fonts, `ofl/marckscript` | «Marck Script» | без изменений (нет знака ₽) |
| Pacifico | script | `Pacifico-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Amatic SC | kids | `AmaticSC-*.ttf` (R, B) | fonts.google.com | нет | урезан набор знаков |
| Comfortaa | kids | `Comfortaa-*.ttf` (R, B) | fonts.google.com, `static/` | «Comfortaa» | без изменений |
| Marmelad | kids | `Marmelad-Regular.ttf` | github.com/google/fonts, `ofl/marmelad` | нет | урезан набор знаков |
| Neucha | kids | `Neucha-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Nunito | kids | `Nunito-*.ttf` (4) | fonts.google.com, `static/` | нет | урезан набор знаков |
| Pangolin | kids | `Pangolin-Regular.ttf` | github.com/google/fonts, `ofl/pangolin` | нет | урезан набор знаков |
| Rubik | kids | `Rubik-*.ttf` (R, B) | github.com/google/fonts, `ofl/rubik`, экземпляры `wght` 400/700 | нет | экземпляры вариативного, урезан набор знаков, кернинг кириллицы в latn |
| Alice | serif | `Alice-Regular.ttf` | github.com/google/fonts, `ofl/alice` | нет | урезан набор знаков (нет №) |
| Cormorant Garamond | serif | `CormorantGaramond-*.ttf` (4) | github.com/CatharsisFonts/Cormorant, `fonts/ttf` | нет | урезан набор знаков |
| Forum | serif | `Forum-Regular.ttf` | fonts.google.com | «Forum» | без изменений |
| Kurale | serif | `Kurale-Regular.ttf` | github.com/google/fonts, `ofl/kurale` | нет | урезан набор знаков |
| Lora | serif | `Lora-*.ttf` (4) | fonts.google.com, `static/` | «Lora» | без изменений |
| Old Standard TT | serif | `OldStandard-*.ttf` (R, B) | github.com/google/fonts, `ofl/oldstandardtt` | нет | урезан набор знаков (курсив не взят: набор 1/2/4) |
| Playfair Display | serif | `PlayfairDisplay-*.ttf` (4) | fonts.google.com, `static/` | «Playfair Display» | без изменений |
| Prata | serif | `Prata-Regular.ttf` | github.com/google/fonts, `ofl/prata` | нет | урезан набор знаков (нет №) |
| PT Serif | serif | `PT_Serif-Web-*.ttf` (4) | github.com/google/fonts, `ofl/ptserif` | «PT Sans», «PT Serif», «ParaType» | без изменений |
| Yeseva One | serif | `YesevaOne-Regular.ttf` | fonts.google.com | «Yeseva» | без изменений |
| Golos Text | sans | `GolosText-*.ttf` (R, B) | github.com/google/fonts, `ofl/golostext`, экземпляры `wght` 400/700 | нет | экземпляры вариативного, урезан набор знаков |
| Montserrat | sans | `Montserrat-*.ttf` (4) | github.com/JulietaUla/Montserrat, `fonts/ttf` | нет | урезан набор знаков |
| Oswald | sans | `Oswald-*.ttf` (R, B) | fonts.google.com, `static/` | нет | урезан набор знаков |
| Raleway | sans | `Raleway-*.ttf` (4) | fonts.google.com, `static/` | «Raleway» | без изменений |
| Roboto Condensed | sans | `RobotoCondensed-*.ttf` (R, B) | github.com/google/fonts, `ofl/robotocondensed`, экземпляры `wght` 400/700 | нет | экземпляры вариативного, урезан набор знаков, кернинг кириллицы в latn |
| Tenor Sans | sans | `TenorSans-Regular.ttf` | github.com/google/fonts, `ofl/tenorsans` | «Tenor», «Tenor Sans» | без изменений (нет знака ₽) |
| Poiret One | display | `PoiretOne-Regular.ttf` | github.com/google/fonts, `ofl/poiretone` | нет | урезан набор знаков |
| Ruslan Display | display | `RuslanDisplay-Regular.ttf` | github.com/google/fonts, `ofl/ruslandisplay` | «Ruslan», «Ruslan Display» | без изменений (нет знака ₽) |
| Underdog | display | `Underdog-Regular.ttf` | github.com/google/fonts, `ofl/underdog` | «Underdog» | без изменений (нет знака ₽) |

Файлы 0.12 (15 гарнитур, спека 046) взяты 2026-10-15 из `github.com/google/fonts`, `ofl/<семейство>` (main, коммит 67bc647ce1910e92f461bcc82aae30f21d21a9cc от 2026-10-14); побайтовое совпадение с исходниками проверено. Все — OFL 1.1 (`METADATA.pb`: `license: "OFL"`). У Rubik, Golos Text и Roboto Condensed в репозитории только вариативные файлы: экземпляры сделаны `fonttools varLib.instancer F[wght].ttf wght=400` и `wght=700` с `--update-name-table` (у них нет RFN, изменение разрешено), затем урезаны командой ниже (решение №69).

«fonts.google.com» — файлы из архива гарнитуры («Download family», статические экземпляры), взяты 2026-10-13. Знаки, обязательные для всех начертаний (тест `TestFontsCyrillic`): А–Я, а–я, Ёё, A–Z, a–z, 0–9 и `.,:;!?«»—–№`. Исключения (`noGlyph` в `families_test.go`): Alice и Prata — нет №. Знака ₽ нет у Marck Script, Forum, Neucha, Playfair Display, Tenor Sans, Ruslan Display и Underdog (`noRuble` в `fallback_test.go`, полноту проверяет `TestFallbackRubleList`). Знак, которого нет в шрифте, рисуется запасным шрифтом — Montserrat.

Урезание (fontTools, 2026-10-07 и 2026-10-13) — латиница с Latin-1 и Latin Extended-A, кириллица (U+0400–045F, Ґґ), типографские знаки (тире, кавычки, многоточие, ‰), € ₽ № ™; оставлены кернинг и лигатуры, хинтинг убран:

```
python -m fontTools.subset F.ttf --unicodes="U+0020-007E,U+00A0-00FF,U+0100-017F,U+0400-045F,U+0490-0491,U+2010-2027,U+2030-203A,U+20AC,U+20BD,U+2116,U+2122" --layout-features='kern,liga,clig,calt,ccmp,locl,mark,mkmk,onum,lnum,pnum,tnum' --no-hinting --desubroutinize --output-file=F.ttf
```

## Кернинг кириллицы

`x/image/font/sfnt` берёт кернинг только из скрипта `latn` таблицы GPOS (нет его — из `DFLT`), а браузер для кириллицы — из `cyrl`. Если кириллические пары лежат в отдельном lookup, подключённом только к `cyrl`, экспорт их не кернит и строка в JPEG длиннее, чем на экране («То» в Roboto Condensed: −99/2048 em на экране, 0 в JPEG). Тест `TestFontsCyrillicKern` требует, чтобы lookups kern у `latn`/`DFLT` включали все lookups kern у `cyrl`.

Правка (2026-10-15: Rubik R/B, Roboto Condensed R/B, Great Vibes) — после урезания, только для файлов без RFN; номера lookups дописываются в конец kern-фичи `latn`, латиница не меняется ни в браузере, ни в Go:

```
python - F.ttf <<'EOF'
import sys
from fontTools.ttLib import TTFont
f = TTFont(sys.argv[1], recalcTimestamp=False)
g = f["GPOS"].table
feats = g.FeatureList.FeatureRecord
def kern(tag):
    s = next((r.Script for r in g.ScriptList.ScriptRecord if r.ScriptTag == tag), None)
    return [feats[i].Feature for i in s.DefaultLangSys.FeatureIndex if feats[i].FeatureTag == "kern"] if s else []
dst = kern("latn") or kern("DFLT")
assert len(dst) == 1
have = list(dst[0].LookupListIndex)
add = list(dict.fromkeys(i for ft in kern("cyrl") for i in ft.LookupListIndex if i not in have))
dst[0].LookupListIndex = have + add
dst[0].LookupCount = len(dst[0].LookupListIndex)
f.save(sys.argv[1])
print(sys.argv[1], "добавлены", add)
EOF
```

Lora (RFN «Lora») так же устроена, но файл менять нельзя — исключение `noCyrlKern` в `kern_test.go`: кириллица Lora в JPEG чуть шире, чем на экране.

Остаток расхождения у всех гарнитур: sfnt берёт значение первого lookup, где пара нашлась, а браузер складывает все lookups фичи. Пары «знак препинания или цифра + буква» (`'А`, `.у`) поэтому могут керниться на экране и не керниться в JPEG.

Гарнитуры с зарезервированным именем (RFN: Lobster, Tenor Sans, Ruslan Display, Underdog и прежние) не урезаны намеренно: изменённый файл с таким именем распространять нельзя. Для них брать файл как отдаёт автор или Google Fonts.

Добавить гарнитуру: статические TTF и `OFL-<id>.txt` сюда, строка в `families.go`, строка в `scripts/third-party.mjs` (массив `fonts`), затем `node scripts/third-party.mjs`.
