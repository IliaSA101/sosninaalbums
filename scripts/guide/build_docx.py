# -*- coding: utf-8 -*-
"""
Сборка .docx (и .pdf) инструкции из docs/install/guide.md: титульный лист,
фирменные стили, колонтитулы, рисунки с подписями «Рис. N».

Запуск:
    python -I scripts/guide/build_docx.py docs/install/guide.md --version 0.10.0 --out .gotmp/guide [--pdf]
    ... --variant macos|26|15|12-14|win|linux # все Mac одним файлом, одна версия macOS, только Windows или только Linux
Результат: <out>/SosninaAlbums-<версия>-install-guide-ru.docx (и .pdf с --pdf);
с --variant macos — …-install-guide-macos-ru.docx, 26|15|12-14 — …-install-guide-macos-<вариант>-ru.docx, win — …-install-guide-windows-ru.docx, linux — …-install-guide-linux-ru.docx.
Код возврата 1 — нет картинки, остался `{{…}}` или сбой разбора. Сбой только PDF —
код 0 и `::warning::` (PDF необязателен).

Шапка guide.md (front matter, всё необязательно):
    title: Название|второй абзац титула     # «|» = перенос строки
    subtitle, suptitle (по умолчанию «Sosnina Albums»), version, date, status, header
    toc: false                              # без оглавления
Метка {{VERSION}} подставляется из --version, иначе из wails.json. Правила разметки — mdparse.py.
"""
import argparse
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import mdparse  # noqa: E402
from docx import Document  # noqa: E402
from docx.enum.text import WD_ALIGN_PARAGRAPH, WD_BREAK  # noqa: E402
from docx.oxml import OxmlElement, parse_xml  # noqa: E402
from docx.oxml.ns import nsdecls, qn  # noqa: E402
from docx.shared import Cm, Emu, Inches, Pt, RGBColor  # noqa: E402
from docx_xml import (  # noqa: E402
    ACCENT, ACCENT_RGB, CODEBG, DARK, FONT, GREY, MONO, ZEBRA, Numbering,
    add_field, add_inline, add_segments, apply_numbering, bottom_border,
    cell_borders, no_proof, normalize_order, shade,
)
from PIL import Image  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
LINE_SPACING = 1.15
FIRST_LINE = Cm(0.6)  # красная строка
IMG_MAX_W = Inches(6.1)  # во всю ширину текста страницы A4 (поля 2,2 см)
IMG_MAX_H = Cm(13)  # высокий снимок не растягиваем на страницу (4:3 на 6,1 дюйма — 12,4 см)
IMG_MIN_PPI = 96  # снимок мельче не растягиваем: крупный план окна 384 px остаётся 4 дюйма, а не размытые 6


def figure_width(px_w: int, px_h: int) -> int:
    """Ширина рисунка в EMU: до ширины текста, не выше IMG_MAX_H и не больше исходного (IMG_MIN_PPI)."""
    native = int(Inches(1) * px_w / IMG_MIN_PPI)
    return min(int(IMG_MAX_W), int(IMG_MAX_H * px_w / px_h), native)


# Файлы под версию macOS: ключ = метка в `<!-- only: … -->` и в имени файла; suffix — окончание имени
# снимка этой версии (assets/annot/mac-04-privacy-26.png), сначала ищется он, потом общий снимок.
# 12–14: шаги те же (macOS 12 Monterey и 13 Ventura ведут себя как 14), снимает раннер macos-14.
# win — файл только про Windows (спека 036). file — середина имени файла, os и other — шапка титула.
_MAC = {"os": "Mac", "other": "Для других версий macOS есть отдельные файлы"}
VARIANTS = {
    "26": {"label": "macOS 26 (Tahoe)", "suffix": "-26", "file": "macos-26", **_MAC},
    "15": {"label": "macOS 15 (Sequoia)", "suffix": "", "file": "macos-15", **_MAC},
    "12-14": {"label": "macOS 12–14 (Monterey, Ventura, Sonoma)", "suffix": "-14", "file": "macos-12-14", **_MAC},
    # macos — все версии Mac одним файлом (спека 039): снимки основной (macOS 15), без суффикса
    "macos": {"label": "macOS 12–26", "suffix": "", "file": "macos", "os": "Mac",
              "other": "Для Windows есть отдельный файл"},
    "win": {"label": "Windows 10 и 11", "suffix": "", "file": "windows", "os": "Windows",
            "other": "Для Mac есть отдельные файлы"},
    # linux — файл только про Linux (спека 101): .deb и tar.gz, снимков экрана нет
    "linux": {"label": "Ubuntu, Mint, Debian", "suffix": "", "file": "linux", "os": "Linux",
              "other": "Для Windows и Mac есть отдельные файлы"},
}


def variant_image(path: Path, variant) -> Path:
    """Снимок под версию macOS: mac-04-privacy.png → mac-04-privacy-26.png, если такой есть."""
    suffix = VARIANTS[variant]["suffix"] if variant else ""
    if suffix:
        # крупный план окна: mac-03-gatekeeper-win.png → mac-03-gatekeeper-26-win.png (суффикс версии перед «-win»)
        stem, tail = (path.stem[:-4], "-win") if path.stem.endswith("-win") else (path.stem, "")
        alt = path.with_name(stem + suffix + tail + path.suffix)
        if alt.is_file():
            return alt
    return path


class BuildError(Exception):
    """Ошибка сборки: текст идёт в stderr, код возврата 1."""


def read_version() -> str:
    data = json.loads((ROOT / "wails.json").read_text(encoding="utf-8"))
    return data["info"]["productVersion"]


def out_name(version: str, ext: str, variant=None) -> str:
    # латиница: GitHub портит кириллицу и пробелы в именах файлов релиза
    mid = f"-{VARIANTS[variant]['file']}" if variant else ""
    return f"SosninaAlbums-{version}-install-guide{mid}-ru.{ext}"


# ── Блоки документа ──────────────────────────────────────────────────────
def title_page(doc, meta):
    doc.add_paragraph().paragraph_format.space_before = Pt(140)
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    r = p.add_run(meta.get("suptitle", "Sosnina Albums"))
    r.bold = True
    r.font.size = Pt(14)
    r.font.color.rgb = ACCENT_RGB
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    p.paragraph_format.space_after = Pt(18)
    for i, chunk in enumerate(meta.get("title", "Инструкция").split("|")):
        if i:
            p.add_run().add_break()
        r = p.add_run(chunk.strip())
        r.bold = True
        r.font.size = Pt(26)
        r.font.color.rgb = DARK
    if meta.get("subtitle"):
        p = doc.add_paragraph()
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        p.paragraph_format.space_after = Pt(48)
        r = p.add_run(meta["subtitle"])
        r.italic = True
        r.font.size = Pt(12)
        r.font.color.rgb = GREY
    rows = [(k, meta.get(key, "")) for k, key in
            (("Версия", "version"), ("Дата", "date"), ("Статус", "status"))]
    rows = [(k, v) for k, v in rows if v]
    if rows:
        t = doc.add_table(rows=0, cols=2)
        t.alignment = 1
        for k, v in rows:
            tr = t.add_row()
            for j, txt in enumerate((k, v)):
                c = tr.cells[j]
                c.width = Inches(1.8 if j == 0 else 3.2)
                cell_borders(c)
                run = c.paragraphs[0].add_run(txt)
                run.font.size = Pt(10.5)
                if j == 0:
                    run.bold = True
                    shade(c, "F3E6E6")
    doc.add_paragraph().add_run().add_break(WD_BREAK.PAGE)


def toc_block(doc):
    h = doc.add_paragraph()
    r = h.add_run("Содержание")
    r.bold = True
    r.font.size = Pt(16)
    r.font.color.rgb = ACCENT_RGB
    h.paragraph_format.space_after = Pt(10)
    p = doc.add_paragraph()
    add_field(p, r'TOC \o "1-3" \h \z \u',
              "Оглавление обновится при открытии документа (или Ctrl+A, F9).")
    doc.add_paragraph().add_run().add_break(WD_BREAK.PAGE)


def make_table(doc, header, rows):
    cols = len(header)
    # ширина колонок — по содержимому: вес = максимальная длина текста
    maxlen = [max(mdparse.plain_len(header[j]), 4) for j in range(cols)]
    for r in rows:
        for j, v in enumerate(r[:cols]):
            maxlen[j] = max(maxlen[j], min(mdparse.plain_len(v), 90))
    weights = [max(n, 5) for n in maxlen]
    sec = doc.sections[0]
    total = Emu(int(sec.page_width - sec.left_margin - sec.right_margin))
    s = sum(weights)
    widths = [Emu(int(total * w / s)) for w in weights]
    min_w, k_widest = Cm(1.7), widths.index(max(widths))
    for j in range(cols):  # узкие поднять до минимума за счёт самой широкой
        if widths[j] < min_w:
            widths[k_widest] = Emu(int(widths[k_widest] - (min_w - widths[j])))
            widths[j] = min_w

    t = doc.add_table(rows=1, cols=cols)
    t.autofit = False
    tblPr = t._tbl.tblPr
    tblPr.append(parse_xml('<w:tblLayout %s w:type="fixed"/>' % nsdecls("w")))
    # жёсткая геометрия: python-docx кладёт <w:tblW w:type="auto"/> — его надо
    # удалить, иначе Word возьмёт auto и таблица станет уже текста
    total_dxa = sum(int(w) for w in widths) // 635
    for old in tblPr.findall(qn("w:tblW")):
        tblPr.remove(old)
    tblW = parse_xml('<w:tblW %s w:w="%d" w:type="dxa"/>' % (nsdecls("w"), total_dxa))
    style_el = tblPr.find(qn("w:tblStyle"))
    if style_el is not None:
        style_el.addnext(tblW)
    else:
        tblPr.insert(0, tblW)
    grid = t._tbl.tblGrid
    for gc in grid.findall(qn("w:gridCol")):
        grid.remove(gc)
    for w in widths:
        gc = OxmlElement("w:gridCol")
        gc.set(qn("w:w"), str(int(w) // 635))
        grid.append(gc)
    # шапка повторяется на каждой странице длинной таблицы
    t.rows[0]._tr.get_or_add_trPr().append(parse_xml("<w:tblHeader %s/>" % nsdecls("w")))
    for j, hd in enumerate(header):
        c = t.rows[0].cells[j]
        c.width = widths[j]
        cell_borders(c)
        shade(c, ACCENT)
        c.paragraphs[0].paragraph_format.keep_with_next = True
        run = c.paragraphs[0].add_run(hd)
        run.bold = True
        run.font.size = Pt(9.5)
        run.font.color.rgb = RGBColor(0xFF, 0xFF, 0xFF)
    for ri, row in enumerate(rows):
        tr = t.add_row()
        tr._tr.get_or_add_trPr().append(parse_xml("<w:cantSplit %s/>" % nsdecls("w")))
        for j, val in enumerate(row[:cols]):
            c = tr.cells[j]
            c.width = widths[j]
            cell_borders(c)
            if ri % 2:
                shade(c, ZEBRA)
            add_inline(c.paragraphs[0], val, size=9.5)
    doc.add_paragraph().paragraph_format.space_after = Pt(4)


def setup_styles(doc):
    n = doc.styles["Normal"]
    n.font.name = FONT
    n.font.size = Pt(11)
    n.paragraph_format.space_after = Pt(6)
    n.paragraph_format.line_spacing = LINE_SPACING
    n.paragraph_format.widow_control = True
    n.element.get_or_add_rPr().append(parse_xml('<w:lang %s w:val="ru-RU"/>' % nsdecls("w")))
    for sid, size, color, before in (
            ("Heading 1", 16, ACCENT_RGB, 18), ("Heading 2", 13, DARK, 14),
            ("Heading 3", 11.5, DARK, 10), ("Heading 4", 11, GREY, 8)):
        st = doc.styles[sid]
        st.font.name = FONT
        st.font.size = Pt(size)
        st.font.bold = True
        st.font.color.rgb = color
        st.paragraph_format.space_before = Pt(before)
        st.paragraph_format.space_after = Pt(6)
        st.paragraph_format.line_spacing = 1.0
        st.paragraph_format.keep_with_next = True  # заголовок не остаётся один внизу страницы


def header_footer(doc, meta):
    sec = doc.sections[0]
    hp = sec.header.paragraphs[0]
    hp.alignment = WD_ALIGN_PARAGRAPH.RIGHT
    title = meta.get("title", "Инструкция").replace("|", " ")
    r = hp.add_run(meta.get("header", "Sosnina Albums — " + title))
    r.font.size = Pt(8)
    r.font.color.rgb = GREY
    bottom_border(hp)
    fp = sec.footer.paragraphs[0]
    fp.alignment = WD_ALIGN_PARAGRAPH.CENTER
    fp.add_run("Стр. ")
    add_field(fp, "PAGE")
    fp.add_run(" из ")
    add_field(fp, "NUMPAGES")
    for run in fp.runs:
        run.font.size = Pt(8)
        run.font.color.rgb = GREY


def hr(doc):
    p = doc.add_paragraph()
    p.paragraph_format.space_before = p.paragraph_format.space_after = Pt(8)
    bottom_border(p, color="C9C9C9", size=8)


def add_code(doc, text):
    p = doc.add_paragraph()
    p.paragraph_format.left_indent = Pt(8)
    p.paragraph_format.line_spacing = 1.0
    pPr = p._p.get_or_add_pPr()
    pPr.append(parse_xml(r'<w:shd %s w:val="clear" w:fill="%s"/>' % (nsdecls("w"), CODEBG)))
    pPr.append(parse_xml('<w:pBdr %s>%s</w:pBdr>' % (nsdecls("w"), ''.join(
        '<w:%s w:val="single" w:sz="4" w:space="4" w:color="E0E0E0"/>' % s
        for s in ("top", "bottom", "left", "right")))))
    run = p.add_run(text)
    run.font.name = MONO
    run.font.size = Pt(9)
    no_proof(run)


def add_figure(doc, base: Path, blk, number: int, errors: list, variant=None):
    """Рисунок по центру и подпись «Рис. N. …»; нет файла — ошибка сборки."""
    if blk.rel.startswith(("http://", "https://")):
        errors.append(f"удалённые рисунки не поддерживаются: {blk.rel}")
        return
    ip = (base / blk.rel).resolve()
    if not ip.is_file():
        errors.append(f"нет картинки: {blk.rel}")
        return
    ip = variant_image(ip, variant)
    cap = mdparse.caption_text(blk.text)
    # подводка не остаётся на предыдущей странице без своей картинки
    if doc.paragraphs:
        doc.paragraphs[-1].paragraph_format.keep_with_next = True
    try:
        with Image.open(ip) as im:
            w, h = im.size
        pic = doc.add_picture(str(ip), width=Emu(figure_width(w, h)))
    except Exception as e:  # noqa: BLE001 — любую ошибку формата сообщаем с именем файла
        errors.append(f"картинка не читается {blk.rel}: {e}")
        return
    pic._inline.docPr.set("descr", cap)  # альтернативный текст
    par = doc.paragraphs[-1]
    par.alignment = WD_ALIGN_PARAGRAPH.CENTER
    par.paragraph_format.keep_with_next = True
    c = doc.add_paragraph()
    c.alignment = WD_ALIGN_PARAGRAPH.CENTER
    c.paragraph_format.space_after = Pt(10)
    cr = c.add_run(f"Рис. {number}." + (f" {cap}" if cap else ""))
    cr.italic = True
    cr.font.size = Pt(9)
    cr.font.color.rgb = GREY


def info_box(doc, segs):
    """Плашка под пунктом списка (строки `~ `): где кликать."""
    p = doc.add_paragraph()
    p.paragraph_format.left_indent = Cm(2.0)
    p.paragraph_format.right_indent = Cm(0.3)
    p.paragraph_format.space_before = Pt(2)
    p.paragraph_format.space_after = Pt(8)
    p._p.get_or_add_pPr().append(parse_xml(
        r'<w:shd %s w:val="clear" w:fill="%s"/>' % (nsdecls("w"), "F4F1E8")))
    add_segments(p, segs, size=10)


def add_quote(doc, text):
    p = doc.add_paragraph()
    p.paragraph_format.left_indent = Pt(14)
    p._p.get_or_add_pPr().append(parse_xml(
        '<w:pBdr %s><w:left w:val="single" w:sz="18" w:space="8" w:color="%s"/></w:pBdr>'
        % (nsdecls("w"), ACCENT)))
    add_inline(p, text, size=10)
    for run in p.runs:
        run.italic = True
        run.font.color.rgb = GREY


def render(doc, meta, blocks, base: Path, errors: list, variant=None):
    num = Numbering(doc)
    fig = 0
    h1_skipped = False
    for blk in blocks:
        k = blk.kind
        if k == "code":
            add_code(doc, blk.text)
        elif k == "image":
            fig += 1
            add_figure(doc, base, blk, fig, errors, variant)
        elif k == "heading":
            # первый H1 — название документа: на титуле уже есть
            if blk.level == 1 and not h1_skipped and meta:
                h1_skipped = True
                continue
            doc.add_heading(blk.text, level=blk.level)
        elif k == "quote":
            add_quote(doc, blk.text)
        elif k == "table":
            if doc.paragraphs:
                doc.paragraphs[-1].paragraph_format.keep_with_next = True
            make_table(doc, blk.header, blk.rows)
        elif k in ("ul", "ol"):
            p = doc.add_paragraph()
            nid = num.bullet() if k == "ul" else num.for_list(blk.list_id, blk.start)
            apply_numbering(p, nid)
            p.paragraph_format.space_after = Pt(3)
            add_inline(p, blk.segs[0])
            if len(blk.segs) > 1:
                info_box(doc, blk.segs[1:])
        elif k == "hr":
            hr(doc)
        elif k == "page":
            doc.add_paragraph().add_run().add_break(WD_BREAK.PAGE)
        else:  # para
            p = doc.add_paragraph()
            p.alignment = WD_ALIGN_PARAGRAPH.JUSTIFY
            p.paragraph_format.first_line_indent = FIRST_LINE
            add_segments(p, blk.segs)


# ── Сборка ───────────────────────────────────────────────────────────────
def variant_meta(meta: dict, variant) -> dict:
    """Шапка файла под одну систему: свой заголовок, без строки про другие."""
    v = VARIANTS[variant]
    label = v["label"]
    meta = dict(meta)
    meta["title"] = f"Как установить на {v['os']}|{label}"
    meta["subtitle"] = v["other"]
    meta["header"] = f"Sosnina Albums — как установить, {label}"
    return meta


def build(md_path: Path, version: str, out_dir: Path, variant=None) -> Path:
    if variant is not None and variant not in VARIANTS:
        raise BuildError(f"неизвестный вариант {variant!r}; есть: {', '.join(VARIANTS)}")
    base = md_path.parent
    lines = md_path.read_text(encoding="utf-8").splitlines()
    try:
        lines = mdparse.select_variant(lines, variant)
    except ValueError as e:
        raise BuildError(str(e)) from e
    lines = mdparse.apply_vars(lines, {"VERSION": version})
    left = mdparse.find_labels(lines)
    if left:
        raise BuildError("метки без значения: " + ", ".join("{{" + x + "}}" for x in left))
    meta, body = mdparse.parse_front_matter(lines)
    if variant and meta:
        meta = variant_meta(meta, variant)
    blocks = mdparse.parse_blocks(body)

    doc = Document()
    sec = doc.sections[0]
    sec.page_width, sec.page_height = Cm(21.0), Cm(29.7)  # A4 (в python-docx по умолчанию Letter)
    sec.top_margin = sec.bottom_margin = Cm(2.0)
    sec.left_margin = sec.right_margin = Cm(2.2)
    # автоперенос выключен: при выравнивании по ширине даёт «дырявые» строки
    doc.settings.element.append(parse_xml("<w:autoHyphenation %s/>" % nsdecls("w")))
    setup_styles(doc)
    header_footer(doc, meta)
    props = doc.core_properties
    props.author = props.last_modified_by = "Sosnina Albums"
    props.title = meta.get("title", "Инструкция").replace("|", " ")
    if meta:
        sec.different_first_page_header_footer = True  # титул без колонтитулов
        title_page(doc, meta)
        if meta.get("toc", "true").lower() != "false":
            toc_block(doc)

    errors = []
    render(doc, meta, blocks, base, errors, variant)
    if errors:
        raise BuildError("\n".join(errors))
    for root in (doc.element, sec.header._element, sec.footer._element):
        normalize_order(root)
    out_dir.mkdir(parents=True, exist_ok=True)
    out = out_dir / out_name(version, "docx", variant)
    doc.save(out)
    return out


# ── PDF: Word (Windows) или LibreOffice ──────────────────────────────────
def pdf_via_word(docx: Path, pdf: Path):
    ps1 = Path(__file__).resolve().parent / "bake_toc.ps1"
    if sys.platform != "win32" or not ps1.is_file() or not shutil.which("powershell"):
        return
    subprocess.run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(ps1),
                    "-Path", str(docx.resolve()), "-Pdf", str(pdf.resolve())],
                   capture_output=True, text=True, timeout=300, check=False)


def pdf_via_soffice(docx: Path, pdf: Path):
    exe = shutil.which("soffice") or shutil.which("libreoffice")
    if not exe:
        return
    with tempfile.TemporaryDirectory() as prof:  # свой профиль: без блокировки чужим процессом
        subprocess.run([exe, f"-env:UserInstallation={Path(prof).as_uri()}", "--headless",
                        "--convert-to", "pdf", "--outdir", str(pdf.parent), str(docx)],
                       capture_output=True, text=True, timeout=300, check=False)


def make_pdf(docx: Path) -> bool:
    """True, если PDF появился. Сбой не роняет сборку: PDF необязателен."""
    pdf = docx.with_suffix(".pdf")
    for step in (pdf_via_word, pdf_via_soffice):
        pdf.unlink(missing_ok=True)
        try:
            step(docx, pdf)
        except (OSError, subprocess.SubprocessError) as e:
            print(f"{step.__name__}: {e}", file=sys.stderr)
        if pdf.is_file() and pdf.stat().st_size > 0:
            return True
    return False


def main(argv=None) -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("guide", type=Path, help="путь к guide.md")
    ap.add_argument("--version", help="метка {{VERSION}}; по умолчанию из wails.json")
    ap.add_argument("--out", type=Path, required=True, help="папка результата")
    ap.add_argument("--variant", choices=sorted(VARIANTS), help="файл про Mac одной версии macOS, только про Windows или только про Linux")
    ap.add_argument("--pdf", action="store_true", help="ещё и PDF (Word или LibreOffice)")
    args = ap.parse_args(argv)
    version = args.version or read_version()
    if not re.fullmatch(r"\d+\.\d+\.\d+", version):
        print(f"версия не вида X.Y.Z: {version}", file=sys.stderr)
        return 1
    try:
        out = build(args.guide, version, args.out, args.variant)
    except (BuildError, OSError) as e:
        print(f"ОШИБКА: {e}", file=sys.stderr)
        return 1
    print(f"DOCX: {out}")
    if args.pdf:
        if make_pdf(out):
            print(f"PDF: {out.with_suffix('.pdf')}")
        else:
            print("::warning::PDF не собран: нет Word и LibreOffice или конвертация упала")
    return 0


if __name__ == "__main__":
    for _s in (sys.stdout, sys.stderr):  # консоль Windows по умолчанию cp1251
        _s.reconfigure(encoding="utf-8")
    sys.exit(main())
