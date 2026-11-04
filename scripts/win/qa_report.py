"""Сводка прогона win-qa (спека 118, design §6): results.jsonl всех заданий -> summary.md и eyes.md.

Запуск: python -I scripts/win/qa_report.py --blocks probe,install --in <папка с win-qa-*> --out <папка> [--update docs/qa/update.md]
Код 1: есть ПРОВАЛ (в том числе неизвестный ID и пропавший снимок) или НЕ ПРОВЕРЕНО в выбранных блоках.
Список пунктов берётся из update.md; в дереве ветки winqa/* его нет (кухня проекта не публикуется) —
тогда из qa-items.txt рядом со скриптом (копия, её сверяет test_qa_report.py). Только stdlib.
"""

import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ITEMS_FILE = os.path.join(HERE, "qa-items.txt")

OK, FAIL, EYES, LAPTOP, UNCHECKED, SKIPPED = "OK", "ПРОВАЛ", "ГЛАЗА", "НОУТБУК", "НЕ ПРОВЕРЕНО", "пропуск"

ITEM_RE = re.compile(r"^- \[ \] ([А-ЯЁ]\d+(?:-\d+)?)\.\s*(.*)$")

# Пункты update.md по заданиям (блок выбран - его пункты обязательны).
BLOCKS = {
    "install": ["В0-1", "В0-2", "В0-3", "В0-4"] + ["В%d" % n for n in range(2, 12)] + ["Ж1", "Ж2", "Ж3", "Ж4", "Г6", "Г7", "Г8"],
    "update": ["Г1", "Г2", "Г3", "Г4", "Г5"] + ["Г%d" % n for n in range(9, 17)] + ["Е%d" % n for n in range(1, 6)],
    "open": ["А%d" % n for n in range(1, 15)] + ["Б%d" % n for n in range(1, 10)] + ["Д%d" % n for n in range(1, 8)],
}
# Всегда на ноутбук: без строки в итогах получают НОУТБУК, а не НЕ ПРОВЕРЕНО
ALWAYS_LAPTOP = ["В1", "Ж5"]
# Служебные пункты задания: обязательны, если блок выбран (DPI-* необязательны: масштабы задаёт вход dpi)
EXTRA_REQUIRED = {"probe": ["P0-uia", "P0-invoke", "P0-keys", "P0-dpi", "P0-openwith", "P0-choice", "P0-front"], "install": ["profile-save", "profile-restore"]}
EXTRA_RE = re.compile(r"^(P0-[a-z]+|profile-(save|restore)|DPI-\d+)$")


def read_items(update_path):
    """Пункты update.md: список (ID, текст) по порядку; нет файла - qa-items.txt."""
    items = []
    if update_path and os.path.isfile(update_path):
        with open(update_path, encoding="utf-8") as f:
            for line in f:
                m = ITEM_RE.match(line.rstrip("\n"))
                if m:
                    items.append((m.group(1), m.group(2)))
        return items
    with open(ITEMS_FILE, encoding="utf-8") as f:
        for line in f:
            if line.strip():
                i, t = line.rstrip("\n").split("\t", 1)
                items.append((i, t))
    return items


def read_results(in_dir):
    """Строки results.jsonl всех подпапок (и самой папки): (строка, папка файла)."""
    rows = []
    for root, _, names in os.walk(in_dir):
        if "results.jsonl" in names:
            with open(os.path.join(root, "results.jsonl"), encoding="utf-8-sig") as f:
                for line in f:
                    line = line.strip()
                    if line:
                        rows.append((json.loads(line), root))
    return rows


def read_aborts(in_dir):
    """logs/abort.txt сторожа (id и шаг по строкам): {блок: (id, шаг)}. Блок - по владельцу пункта."""
    owner = {i: b for b, ids in BLOCKS.items() for i in ids}
    owner.update({i: b for b, ids in EXTRA_REQUIRED.items() for i in ids})
    res = {}
    for root, _, names in os.walk(in_dir):
        path = os.path.join(root, "abort.txt")
        if "abort.txt" not in names and os.path.isfile(os.path.join(root, "logs", "abort.txt")):
            path = os.path.join(root, "logs", "abort.txt")
        elif "abort.txt" not in names:
            continue
        with open(path, encoding="utf-8-sig") as f:
            lines = [ln.strip() for ln in f.read().splitlines()]
        if lines and lines[0]:
            res[owner.get(lines[0], "install")] = (lines[0], lines[1] if len(lines) > 1 else "")
    return res


def plain(text, limit):
    text = re.sub(r"[`*|]", "", text).replace("\n", " ")
    return text if len(text) <= limit else text[: limit - 1] + "…"


def verdict(rows, selected, required_laptop):
    """Итог пункта по его строкам; selected - выбран ли блок пункта."""
    st = {r["status"] for r, _ in rows}
    if "fail" in st:
        return FAIL
    if "skip" in st:
        return UNCHECKED
    if "eyes" in st:
        return EYES
    if "laptop" in st:
        return LAPTOP
    if "ok" in st:
        return OK
    if required_laptop:
        return LAPTOP
    return UNCHECKED if selected else SKIPPED


def build(items, blocks, rows, aborts=None):
    """Возвращает (записи пунктов, ошибки сводки). Запись: id, block, text, verdict, detail, shots."""
    known = {i for i, _ in items}
    owner = {i: b for b, ids in BLOCKS.items() for i in ids}
    extras = {i: b for b, ids in EXTRA_REQUIRED.items() for i in ids}
    aborts = aborts or {}
    problems = []
    by_id = {}
    missing = set()
    for r, folder in rows:
        i = r["id"]
        if i not in known and not EXTRA_RE.match(i):
            problems.append("неизвестный ID в итогах: %s" % i)
            continue
        by_id.setdefault(i, []).append((r, folder))
        for rel in list(r.get("shots") or []) + ([r["video"]] if r.get("video") else []):
            if not os.path.isfile(os.path.join(folder, rel)):
                problems.append("нет файла %s (пункт %s)" % (rel, i))
                missing.add(i)
    text_of = dict(items)
    order = [i for i, _ in items] + [i for b in EXTRA_REQUIRED for i in EXTRA_REQUIRED[b]]
    order += sorted(i for i in by_id if i not in order)
    out = []
    for i in order:
        rs = by_id.get(i, [])
        if not rs and i not in text_of and i not in extras:
            continue
        block = "" if i in ALWAYS_LAPTOP else owner.get(i) or extras.get(i) or "install"
        v = verdict(rs, block in blocks, i in ALWAYS_LAPTOP)
        shots = [s for r, _ in rs for s in (r.get("shots") or [])] + [r["video"] for r, _ in rs if r.get("video")]
        detail = "; ".join(r.get("detail", "") for r, _ in rs if r.get("detail"))
        if i in missing:
            v, detail = FAIL, "нет снимка: " + detail
        if v == EYES and not shots:
            problems.append("пункт %s: ГЛАЗА без снимка" % i)
        if v == SKIPPED:
            detail = "пропуск: блок не выбран"
        if v == UNCHECKED and not rs and block in aborts:
            detail = "НЕ ПРОВЕРЕНО: блок прерван на %s (%s)" % aborts[block]
        out.append({"id": i, "block": block or "ноутбук", "text": text_of.get(i, ""), "verdict": v, "detail": detail, "shots": shots})
    return out, problems


def render(out, problems):
    lines = ["# Проверка Windows-exe (win-qa)", ""]
    count = {}
    for e in out:
        count[e["verdict"]] = count.get(e["verdict"], 0) + 1
    lines.append("Итого: " + ", ".join("%s - %d" % (k, v) for k, v in sorted(count.items())))
    lines.append("")
    if problems:
        lines += ["## Ошибки сводки", ""] + ["- %s" % p for p in problems] + [""]
    for block in ("probe", "install", "update", "open", "ноутбук"):
        part = [e for e in out if e["block"] == block]
        if not part:
            continue
        lines += ["## Блок %s" % block, "", "| Пункт | Итог | Подробно | Снимки |", "| --- | --- | --- | --- |"]
        for e in part:
            lines.append("| %s | %s | %s | %s |" % (e["id"], e["verdict"], plain(e["detail"], 300), plain(", ".join(e["shots"]), 300)))
        lines.append("")
    eyes = ["# Что посмотреть глазами", "", "| Пункт | Что смотреть | Файлы |", "| --- | --- | --- |"]
    for e in out:
        if e["verdict"] == EYES:
            eyes.append("| %s | %s | %s |" % (e["id"], plain(e["text"] or e["detail"], 200), plain(", ".join(e["shots"]), 300)))
    return "\n".join(lines) + "\n", "\n".join(eyes) + "\n"


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--update", default="")
    ap.add_argument("--blocks", default="")
    ap.add_argument("--in", dest="in_dir", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args(argv)
    blocks = {b for b in a.blocks.split(",") if b}
    out, problems = build(read_items(a.update), blocks, read_results(a.in_dir), read_aborts(a.in_dir))
    summary, eyes = render(out, problems)
    os.makedirs(a.out, exist_ok=True)
    for name, text in (("summary.md", summary), ("eyes.md", eyes)):
        with open(os.path.join(a.out, name), "w", encoding="utf-8", newline="\n") as f:
            f.write(text)
    sys.stdout.write(summary)
    failed = problems or any(e["verdict"] in (FAIL, UNCHECKED) for e in out)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
