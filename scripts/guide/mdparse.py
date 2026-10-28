# -*- coding: utf-8 -*-
"""
Разбор guide.md в список блоков и инлайн-разметки (чистая логика, без docx).

Правила (как в docs-pipeline-kit):
  - переносы строк ВНУТРИ абзаца склеиваются (абзац = до пустой строки), то же для
    пунктов списков с продолжением на следующих строках с отступом;
  - строка с `~ ` в начале — перенос строки внутри абзаца или пункта, не новый абзац;
  - каждый нумерованный список нумеруется заново (list_id); картинка или таблица
    между пунктами список не рвёт.

Поддерживаемое подмножество Markdown:
  # .. ####   заголовки (хвост {#anchor} отбрасывается)
  > текст     врезка
  - / *       маркированный список; 1.  нумерованный
  ![подпись](путь)   картинка; | a | b |  таблица со строкой |---|
  ```         блок кода; ---  отбивка; \\page  разрыв страницы
  **жирный** *курсив* `код` [текст](http…)   инлайн
Метка `{{VERSION}}` подставляется до разбора; любая другая `{{…}}` — ошибка сборки.
"""
import re
from dataclasses import dataclass

IMG_RE = re.compile(r"^!\[(.*?)\]\((.+?)\)\s*$")
H_RE = re.compile(r"^(#{1,4})\s+(.*)$")
UL_RE = re.compile(r"^[-*]\s+(.*)$")
OL_RE = re.compile(r"^(\d+)\.\s+(.*)$")
CONT_RE = re.compile(r"^\s{2,}\S")  # продолжение пункта списка
LINK_RE = re.compile(r"\[([^\]]+)\]\(([^)\s]+)\)")
INLINE_RE = re.compile(r"(\*\*.+?\*\*|\*[^*]+?\*|`.+?`)")
ANCHOR_RE = re.compile(r"\s*\{#[^}]+\}\s*$")
LABEL_RE = re.compile(r"\{\{([^{}]*)\}\}")
FIG_PREFIX_RE = re.compile(r"^Рис\.\s*\d+\.?\s*")


@dataclass
class Block:
    kind: str  # code heading quote table ul ol image hr page para
    text: str = ""  # code, heading, quote; подпись у image
    level: int = 0  # heading
    segs: tuple = ()  # para, ul, ol: сегменты (каждый с новой строки)
    header: tuple = ()  # table
    rows: tuple = ()  # table
    rel: str = ""  # image: путь из разметки
    start: int = 1  # ol: первый номер списка
    list_id: int = 0  # ol: сквозной номер списка


def parse_front_matter(lines):
    """Шапка `---` … `---` → (словарь, остаток строк)."""
    meta = {}
    if lines and lines[0].strip() == "---":
        for i in range(1, len(lines)):
            if lines[i].strip() == "---":
                return meta, lines[i + 1:]
            if ":" in lines[i]:
                k, v = lines[i].split(":", 1)
                meta[k.strip()] = v.strip()
    return meta, lines


ONLY_OPEN_RE = re.compile(r"^<!--\s*only:\s*([^>]*?)\s*-->\s*$")
ONLY_CLOSE_RE = re.compile(r"^<!--\s*/only\s*-->\s*$")


# Где виден блок с данной меткой (design 039 §7.1). None — общий документ; "macos" — все версии Mac
# одним файлом; "win" — только Windows; "linux" — только Linux; "26"/"15"/"12-14" — файл одной версии Mac.
#   метка        общий  win  macos  26/15/12-14
#   general       да     —    да     —
#   both          да     —    —      —    (текст сразу про обе ОС)
#   win           да     да   —      —    (раздел Windows)
#   windows       —      да   —      —    (только файл Windows)
#   linux         —      —    —      —    (только файл Linux; спека 101)
#   mac           —      —    —      да   (строки для файла одной версии Mac)
#   26/15/12-14   да     —    да     своя
#   macos         —      —    да     —    (только файл «все Mac»)
VERSIONS = ("26", "15", "12-14")
VARIANT_TAGS = {
    None: {"general", "both", "win", *VERSIONS},
    "win": {"win", "windows"},
    "linux": {"linux"},
    "macos": {"general", "macos", *VERSIONS},
    **{v: {"mac", v} for v in VERSIONS},
}


def select_variant(lines, variant=None):
    """Выбирает строки под файл: блок `<!-- only: a, b -->` … `<!-- /only -->` виден не всегда.

    Блок виден в варианте, если хотя бы одна его метка видна в нём (таблица VARIANT_TAGS);
    вложенный блок виден, только если видны все объемлющие. Блок без меток из таблицы скрыт.
    """
    if variant not in VARIANT_TAGS:
        raise ValueError(f"неизвестный вариант {variant!r}")
    visible = VARIANT_TAGS[variant]
    out, stack = [], []
    for line in lines:
        m = ONLY_OPEN_RE.match(line.strip())
        if m:
            tags = {t.strip() for t in m.group(1).split(",") if t.strip()}
            stack.append(bool(tags & visible))
            continue
        if ONLY_CLOSE_RE.match(line.strip()):
            if not stack:
                raise ValueError("<!-- /only --> без <!-- only: -->")
            stack.pop()
            continue
        if all(stack):
            out.append(line)
    if stack:
        raise ValueError("не закрыт <!-- only: -->")
    return out


def apply_vars(lines, values: dict):
    """Подставляет {{KEY}} из values; остальные метки остаются — их ловит find_labels."""
    out = []
    for line in lines:
        out.append(LABEL_RE.sub(lambda m: values.get(m.group(1).strip(), m.group(0)), line))
    return out


def find_labels(lines):
    """Нераскрытые метки {{…}} (после apply_vars)."""
    return sorted({m.strip() for line in lines for m in LABEL_RE.findall(line)})


def caption_text(alt: str) -> str:
    """Подпись без своего «Рис. N.» — номер ставит сборка по порядку."""
    return FIG_PREFIX_RE.sub("", alt).strip()


def is_block_start(line: str) -> bool:
    """Строка начинает новый блок (не продолжение абзаца)."""
    return bool(
        not line.strip()
        or H_RE.match(line) or IMG_RE.match(line) or UL_RE.match(line) or OL_RE.match(line)
        or line.startswith(("|", ">", "```")) or line.strip() in ("---", "\\page")
    )


def _segments(first: str, lines, i: int, cont_only: bool):
    """Собирает сегменты абзаца/пункта: `~ ` начинает новый сегмент (перенос строки)."""
    segs = [[first.strip()]]
    n = len(lines)
    while i < n:
        raw = lines[i]
        if cont_only:
            if not (CONT_RE.match(raw) and not is_block_start(raw.strip())):
                break
        elif is_block_start(raw):
            break
        s = raw.strip()
        if s.startswith("~ "):
            segs.append([s[2:]])
        else:
            segs[-1].append(s)
        i += 1
    return tuple(" ".join(x) for x in segs), i


def _table_start(lines, i):
    return (lines[i].startswith("|") and i + 1 < len(lines) and "-" in lines[i + 1]
            and set(lines[i + 1].replace("|", "").strip()) <= set("-: "))


def parse_blocks(lines):
    """Строки тела (без шапки) → список Block."""
    blocks, i, n = [], 0, len(lines)
    ol_id = 0  # id текущего нумерованного списка, 0 — списка нет
    next_id = 0
    while i < n:
        line = lines[i]

        if line.strip().startswith("```"):
            ol_id = 0
            i += 1
            buf = []
            while i < n and not lines[i].strip().startswith("```"):
                buf.append(lines[i])
                i += 1
            i += 1
            blocks.append(Block("code", text="\n".join(buf)))
            continue

        m = IMG_RE.match(line)
        if m:
            blocks.append(Block("image", text=m.group(1), rel=m.group(2)))
            i += 1
            continue

        m = H_RE.match(line)
        if m:
            ol_id = 0
            blocks.append(Block("heading", level=len(m.group(1)),
                                text=ANCHOR_RE.sub("", m.group(2)).strip()))
            i += 1
            continue

        if line.startswith(">"):
            ol_id = 0
            buf = []
            while i < n and lines[i].startswith(">"):
                buf.append(re.sub(r"^>\s?", "", lines[i]).rstrip())
                i += 1
            blocks.append(Block("quote", text=" ".join(x for x in buf if x)))
            continue

        if _table_start(lines, i):
            ol_id = 0
            header = tuple(c.strip() for c in line.strip("|").split("|"))
            i += 2
            rows = []
            while i < n and lines[i].startswith("|"):
                rows.append(tuple(c.strip() for c in lines[i].strip("|").split("|")))
                i += 1
            blocks.append(Block("table", header=header, rows=tuple(rows)))
            continue

        m = UL_RE.match(line)
        if m:
            ol_id = 0
            segs, i = _segments(m.group(1), lines, i + 1, cont_only=True)
            blocks.append(Block("ul", segs=segs))
            continue

        m = OL_RE.match(line)
        if m:
            if not ol_id:
                next_id += 1
                ol_id = next_id
            segs, i = _segments(m.group(2), lines, i + 1, cont_only=True)
            blocks.append(Block("ol", segs=segs, start=int(m.group(1)), list_id=ol_id))
            continue

        if line.strip() == "---":
            ol_id = 0
            blocks.append(Block("hr"))
            i += 1
            continue

        if line.strip() == "\\page":
            ol_id = 0
            blocks.append(Block("page"))
            i += 1
            continue

        if not line.strip():
            # пустая строка завершает нумерованный список, только если дальше
            # идёт не пункт, картинка или таблица
            j = i + 1
            while j < n and not lines[j].strip():
                j += 1
            if j < n and not (OL_RE.match(lines[j]) or IMG_RE.match(lines[j])
                              or lines[j].startswith("|")):
                ol_id = 0
            i += 1
            continue

        ol_id = 0
        segs, i = _segments(line, lines, i + 1, cont_only=False)
        blocks.append(Block("para", segs=segs))
    return blocks


def parse_inline(text: str):
    """Текст → [(вид, текст, url)]; вид: text bold italic code link."""
    out = []

    def formats(chunk):
        for part in INLINE_RE.split(chunk):
            if not part:
                continue
            if part.startswith("**") and part.endswith("**") and len(part) > 4:
                out.append(("bold", part[2:-2], ""))
            elif part.startswith("*") and part.endswith("*") and len(part) > 2:
                out.append(("italic", part[1:-1], ""))
            elif part.startswith("`") and part.endswith("`") and len(part) > 2:
                out.append(("code", part[1:-1], ""))
            else:
                out.append(("text", part, ""))

    pos = 0
    for m in LINK_RE.finditer(text):
        formats(text[pos:m.start()])
        out.append(("link", m.group(1), m.group(2)))
        pos = m.end()
    formats(text[pos:])
    return out


def plain_len(s: str) -> int:
    """Длина текста ячейки без разметки (для ширины колонок)."""
    s = LINK_RE.sub(r"\1", s)
    return len(re.sub(r"[*`]", "", s))
