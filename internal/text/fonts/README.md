# Встроенные шрифты

Все — SIL Open Font License 1.1 (тексты лицензий — `OFL-*.txt` рядом). Шрифты вшиты в программу: текст на экране и в экспорте рисуется одними и теми же файлами, на другой компьютер переносить ничего не нужно.

| Гарнитура | Файлы | Источник | Изменения |
| --- | --- | --- | --- |
| Montserrat | `Montserrat-*.ttf` (4 начертания) | github.com/JulietaUla/Montserrat, `fonts/ttf` | урезан набор знаков |
| Cormorant Garamond | `CormorantGaramond-*.ttf` (4) | github.com/CatharsisFonts/Cormorant, `fonts/ttf` | урезан набор знаков |
| PT Serif | `PT_Serif-Web-*.ttf` (4) | github.com/google/fonts, `ofl/ptserif` | без изменений |
| Marck Script | `MarckScript-Regular.ttf` | github.com/google/fonts, `ofl/marckscript` | без изменений (нет знака ₽) |

Урезание (fontTools, 2026-10-07) — латиница с Latin-1 и Latin Extended-A, кириллица (U+0400–045F, Ґґ), типографские знаки (тире, кавычки, многоточие, ‰), € ₽ № ™; оставлены кернинг и лигатуры, хинтинг убран:

```
python -m fontTools.subset F.ttf --unicodes="U+0020-007E,U+00A0-00FF,U+0100-017F,U+0400-045F,U+0490-0491,U+2010-2027,U+2030-203A,U+20AC,U+20BD,U+2116,U+2122" --layout-features='kern,liga,clig,calt,ccmp,locl,mark,mkmk,onum,lnum,pnum,tnum' --no-hinting --desubroutinize --output-file=F.ttf
```

PT Serif и Marck Script не урезаны намеренно: у них в лицензии «зарезервированное имя» (Reserved Font Name), а изменённый файл с таким именем распространять нельзя.
