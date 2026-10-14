# Встроенные шрифты

Все — SIL Open Font License 1.1 (тексты лицензий — `OFL-<id>.txt` рядом, по одному на гарнитуру; `id` — из `families.go`). Шрифты вшиты в программу: текст на экране и в экспорте рисуется одними и теми же файлами, на другой компьютер переносить ничего не нужно.

Число гарнитур и файлов — в `families.go` и в заголовке раздела 3 `THIRD_PARTY_NOTICES.txt` (считает скрипт). Только статические TTF, по файлу на начертание: `x/image/font/sfnt` вариаций не применяет, а браузер подставил бы ось `wght`, и жирный на экране и в экспорте разошёлся бы (тест `TestFontsStatic` ищет `fvar`). Наборы начертаний — 1, 2 (обычное + жирное) или 4.

| Гарнитура | Группа | Файлы | Источник | RFN | Изменения |
| --- | --- | --- | --- | --- | --- |
| Bad Script | script | `BadScript-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Caveat | script | `Caveat-*.ttf` (R, B) | fonts.google.com, `static/` | нет | урезан набор знаков |
| Great Vibes | script | `GreatVibes-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Marck Script | script | `MarckScript-Regular.ttf` | github.com/google/fonts, `ofl/marckscript` | «Marck Script» | без изменений (нет знака ₽) |
| Pacifico | script | `Pacifico-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Amatic SC | kids | `AmaticSC-*.ttf` (R, B) | fonts.google.com | нет | урезан набор знаков |
| Comfortaa | kids | `Comfortaa-*.ttf` (R, B) | fonts.google.com, `static/` | «Comfortaa» | без изменений |
| Neucha | kids | `Neucha-Regular.ttf` | fonts.google.com | нет | урезан набор знаков |
| Nunito | kids | `Nunito-*.ttf` (4) | fonts.google.com, `static/` | нет | урезан набор знаков |
| Cormorant Garamond | serif | `CormorantGaramond-*.ttf` (4) | github.com/CatharsisFonts/Cormorant, `fonts/ttf` | нет | урезан набор знаков |
| Forum | serif | `Forum-Regular.ttf` | fonts.google.com | «Forum» | без изменений |
| Lora | serif | `Lora-*.ttf` (4) | fonts.google.com, `static/` | «Lora» | без изменений |
| Playfair Display | serif | `PlayfairDisplay-*.ttf` (4) | fonts.google.com, `static/` | «Playfair Display» | без изменений |
| PT Serif | serif | `PT_Serif-Web-*.ttf` (4) | github.com/google/fonts, `ofl/ptserif` | «PT Sans», «PT Serif», «ParaType» | без изменений |
| Yeseva One | serif | `YesevaOne-Regular.ttf` | fonts.google.com | «Yeseva» | без изменений |
| Montserrat | sans | `Montserrat-*.ttf` (4) | github.com/JulietaUla/Montserrat, `fonts/ttf` | нет | урезан набор знаков |
| Oswald | sans | `Oswald-*.ttf` (R, B) | fonts.google.com, `static/` | нет | урезан набор знаков |
| Raleway | sans | `Raleway-*.ttf` (4) | fonts.google.com, `static/` | «Raleway» | без изменений |

«fonts.google.com» — файлы из архива гарнитуры («Download family», статические экземпляры), взяты 2026-10-13. Знаки, обязательные для всех начертаний (тест `TestFontsCyrillic`): А–Я, а–я, Ёё, A–Z, a–z, 0–9 и `.,:;!?«»—–№`. Исключения: Marck Script — нет знака ₽ (в тест не входит).

Урезание (fontTools, 2026-10-07 и 2026-10-13) — латиница с Latin-1 и Latin Extended-A, кириллица (U+0400–045F, Ґґ), типографские знаки (тире, кавычки, многоточие, ‰), € ₽ № ™; оставлены кернинг и лигатуры, хинтинг убран:

```
python -m fontTools.subset F.ttf --unicodes="U+0020-007E,U+00A0-00FF,U+0100-017F,U+0400-045F,U+0490-0491,U+2010-2027,U+2030-203A,U+20AC,U+20BD,U+2116,U+2122" --layout-features='kern,liga,clig,calt,ccmp,locl,mark,mkmk,onum,lnum,pnum,tnum' --no-hinting --desubroutinize --output-file=F.ttf
```

Гарнитуры с зарезервированным именем (RFN) не урезаны намеренно: изменённый файл с таким именем распространять нельзя. Для них брать файл как отдаёт автор или Google Fonts.

Добавить гарнитуру: статические TTF и `OFL-<id>.txt` сюда, строка в `families.go`, строка в `scripts/third-party.mjs` (массив `fonts`), затем `node scripts/third-party.mjs`.
