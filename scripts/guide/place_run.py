# -*- coding: utf-8 -*-
"""
Снимки прогона mac-shots.yml → docs/install/assets (raw + .ann.json + annot) → docx инструкции.

Одна команда после скачивания артефакта (gh run download <id> -n mac-shots-macos-15 -D <папка>):
    python -I scripts/guide/place_run.py <папка артефакта> --os 15 --docx
    python -I scripts/guide/place_run.py <папка артефакта> --os 26     # снимки Mac 26: имена с «-26»

Что делает: читает <папка>/shots.json и png/NN-имя.png; кладёт полный экран в
assets/raw/mac-NN-имя.png (для окон-диалогов ещё «-win» — крупный план); из target.rect
(кнопка, найденная прогоном через AX/OCR) пишет assets/raw/*.ann.json со стрелкой и подписью
(«видим такое окно → идём сюда»); зовёт annotate.py → assets/annot/*.png; обновляет
assets/sources.json. --docx собирает .docx в --out (по умолчанию .gotmp/guide), --video —
ролик assets/mac-steps.mp4 из полноэкранных снимков со стрелками (запасной путь, если
запись экрана раннера не вышла).
Снимки шире 1920 px (Retina) уменьшаются до 1920: файл в репозитории ≤ 400 КБ, текст на 1920 читается.
--targets файл.json — вручную поправленные цели: {"03-gatekeeper": {"rect": [x, y, w, h], "label": "Готово"}};
кроме rect и label можно задать "text" (подпись, {label} подставляется) и "side" (left|right|above|below —
сторона подписи, если авто-выбор кладёт её поверх текста окна).
"""
import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import annotate  # noqa: E402
from PIL import Image  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / "docs" / "install" / "assets"
MAX_W = 1920
SMALL_WINDOW = 0.35  # доля экрана: окно меньше — диалог, подпись ставится вне него

# шаг прогона → как называется в инструкции и что написано у стрелки.
# text — подпись стрелки ({label} — надпись кнопки из прогона), None — стрелка не нужна;
# closeup — ещё и крупный план окна; window_only — вместо экрана только окно (без стрелки);
# side — сторона подписи на полном экране; video — кадр ролика; order — порядок кадров
STEPS = {
    "02-app-window": {"asset": "mac-02-app-window", "text": None, "video": True, "order": 70},
    "03-gatekeeper": {"asset": "mac-03-gatekeeper", "text": "Нажмите «{label}»", "closeup": True,
                      "default": "Готово", "video": True, "order": 10},
    "04a-apple-menu": {"asset": "mac-04a-apple-menu", "text": "Выберите «Системные настройки…»",
                       "video": True, "order": 20},
    "04-privacy": {"asset": "mac-04-privacy", "text": "Откроется этот раздел", "side": "right",
                   "video": True, "order": 30},
    "05-open-anyway": {"asset": "mac-05-open-anyway", "text": "Нажмите «{label}»",
                       "default": "Все равно открыть", "video": True, "order": 40},
    "06-confirm": {"asset": "mac-06-confirm", "text": "Нажмите «{label}»", "closeup": True, "side": "right",
                   "default": "Все равно открыть", "video": True, "order": 50},
    # только окно «Терминала»: на полном экране за ним остаётся диалог предыдущего шага — он сбивает с толку
    "07-terminal-xattr": {"asset": "mac-07-terminal-xattr", "text": None, "window_only": True,
                          "video": True, "order": 60},
    # только macOS 12–14 (раннер macos-14): правая кнопка по программе → «Открыть»
    "10-context-menu": {"asset": "mac-10-context-menu", "text": "Нажмите правой кнопкой по значку программы",
                        "order": 80},
    # распаковка архива (спека 039): имена шагов прогона уже с префиксом mac-01-; ролик — отдельный (-unzip)
    "mac-01-zip-downloads": {"asset": "mac-01-zip-downloads", "text": "Дважды щёлкните по архиву", "order": 5},
    "mac-01-zip-folder": {"asset": "mac-01-zip-folder", "text": "Перетащите программу в «Программы»",
                          "order": 6},
    "mac-01-drag-applications": {"asset": "mac-01-drag-applications", "text": "Бросьте программу сюда",
                                 "order": 7},
    "11-open-dialog": {"asset": "mac-11-open-dialog", "text": "Нажмите «{label}»", "closeup": True,
                       "default": "Открыть", "order": 90},
}


def clean_label(label: str, default: str) -> str:
    """Надпись кнопки из прогона: OCR отдаёт строку целиком, при странном значении — стандартная."""
    label = " ".join((label or "").split())
    return label if 0 < len(label) <= 40 else default


def to_ann_rect(rect, origin=(0, 0), scale=1.0):
    """[x, y, w, h] снимка → [x0, y0, x1, y1] для .ann.json; origin — левый верх окна крупного плана."""
    x, y, w, h = rect
    return [round((x - origin[0]) * scale), round((y - origin[1]) * scale),
            round((x + w - origin[0]) * scale), round((y + h - origin[1]) * scale)]


def inside(r, size) -> bool:
    return r[0] >= 0 and r[1] >= 0 and r[2] <= size[0] and r[3] <= size[1] and r[2] > r[0] and r[3] > r[1]


def closeup_pad(win_w: int) -> int:
    """Поле вокруг крупного плана окна: подпись и стрелка уходят на поле, а не закрывают текст диалога."""
    return max(80, round(win_w * 0.35))


def pad_image(img: Image.Image, pad: int, color=(232, 232, 232)) -> Image.Image:
    out = Image.new("RGB", (img.width + 2 * pad, img.height + 2 * pad), color)
    out.paste(img.convert("RGB"), (pad, pad))
    return out


def fit_scale(width: int) -> float:
    """Во сколько раз уменьшить снимок: шире MAX_W — до MAX_W, иначе 1."""
    return MAX_W / width if width > MAX_W else 1.0


def build_specs(shot: dict, step: dict, size, override=None):
    """(spec полного экрана, spec крупного плана или None) для .ann.json.

    size — размер снимка в пикселях; координаты в спеках — уже после fit_scale.
    """
    target = {**(shot.get("target") or {}), **(override or {})}  # поправка дополняет цель прогона
    rect = target.get("rect")
    if not step.get("text") or not rect or len(rect) != 4:
        return {}, None
    k = fit_scale(size[0])
    label = clean_label(target.get("label", ""), step.get("default", ""))
    text = (target.get("text") or step["text"]).format(label=label)
    full_t = {"rect": to_ann_rect(rect, scale=k), "text": text}
    side = target.get("side") or step.get("side")
    if side:
        full_t["side"] = side
    full = {"targets": [full_t]}
    win = None
    wrect = shot.get("window")
    ok_win = bool(wrect) and len(wrect) == 4 and wrect[2] > 0 and wrect[3] > 0
    # небольшое окно (диалог) подпись не закрывает; окно во весь экран — не мешает искать место
    if ok_win and wrect[2] * wrect[3] < SMALL_WINDOW * size[0] * size[1]:
        full["keep_clear"] = [to_ann_rect(wrect, scale=k)]
    if step.get("closeup") and ok_win:
        r = to_ann_rect(rect, origin=wrect[:2], scale=k)
        ww, wh = round(wrect[2] * k), round(wrect[3] * k)
        if inside(r, (ww, wh)):
            pad = closeup_pad(ww)
            win = {"targets": [{"rect": [v + pad for v in r], "text": text}],
                   "keep_clear": [[pad, pad, pad + ww, pad + wh]]}
    return full, win


def save_raw(img: Image.Image, path: Path, k: float):
    if k != 1.0:
        img = img.resize((round(img.width * k), round(img.height * k)), Image.Resampling.LANCZOS)
    annotate.save_clean(img.convert("RGB"), path)


def place(run: Path, os_ver: str, assets: Path, overrides: dict, source: str, skip=()) -> list:
    """Раскладывает снимки; возвращает имена готовых рисунков (annot/<имя>.png) в порядке ролика."""
    shots = {s["name"]: s for s in json.loads((run / "shots.json").read_text(encoding="utf-8"))}
    suffix = "" if os_ver == "15" else f"-{os_ver}"  # 14 = и macOS 12–13: те же окна
    sources_path = assets / "sources.json"
    sources = json.loads(sources_path.read_text(encoding="utf-8")) if sources_path.is_file() else {}
    done = []
    for name, step in sorted(STEPS.items(), key=lambda kv: kv[1]["order"]):
        if name in skip:
            print(f"  [пропуск] {name}: --skip")
            continue
        shot = shots.get(name)
        if not shot or not shot.get("ok") or not shot.get("file"):
            print(f"  [нет] {name}: {'снимка нет' if not shot else shot.get('why') or 'не годен'}")
            continue
        src = run / shot["file"]
        if not src.is_file():
            print(f"  [нет] {name}: нет файла {shot['file']}")
            continue
        if step.get("window_only") and shot.get("win") and (run / shot["win"]).is_file():
            src = run / shot["win"]
        with Image.open(src) as im:
            im.load()
            full_img, size = im.copy(), im.size
        k = fit_scale(size[0])
        full_spec, win_spec = build_specs(shot, step, size, overrides.get(name))
        if src != run / shot["file"]:  # окно вместо экрана: координаты цели относятся к экрану, не к окну
            full_spec, win_spec = {}, None
        base = step["asset"] + suffix
        raw = assets / "raw"
        save_raw(full_img, raw / f"{base}.png", k)
        (raw / f"{base}.ann.json").write_text(json.dumps(full_spec, ensure_ascii=False, indent=2) + "\n",
                                              encoding="utf-8")
        annotate.annotate_one(raw / f"{base}.png")
        if step.get("video"):
            done.append(base)
        if step.get("closeup") and shot.get("win") and (run / shot["win"]).is_file():
            with Image.open(run / shot["win"]) as wim:
                wim.load()
                if win_spec:  # поле только там, где есть стрелка
                    wim = pad_image(wim, round(closeup_pad(round(wim.width * k)) / k))
                save_raw(wim, raw / f"{base}-win.png", k)
            (raw / f"{base}-win.ann.json").write_text(
                json.dumps(win_spec or {}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            annotate.annotate_one(raw / f"{base}-win.png")
        sources[base] = {
            "from": source or "GitHub Actions, workflow mac-shots.yml",
            "macos": shot.get("macos", ""),
            "lang": shot.get("lang", ""),
            "shot": shot["file"],
            "screen": list(size),
            "rule": "цель — target.rect из shots.json (AX/OCR прогона): рамка, стрелка и подпись — annotate.py targets",
        }
    sources_path.write_text(json.dumps(sources, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return done


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("run", type=Path, help="папка артефакта mac-shots-<раннер> (shots.json, png/)")
    ap.add_argument("--os", default="15", help="версия macOS снимков: 15 — без суффикса, иначе «-<версия>»")
    ap.add_argument("--assets", type=Path, default=ASSETS)
    ap.add_argument("--targets", type=Path, help="JSON с поправленными целями (имя шага → rect, label)")
    ap.add_argument("--source", default="", help="откуда снимки (прогон, раннер) — в sources.json")
    ap.add_argument("--skip", default="", help="шаги, которые не раскладывать, через запятую (07-terminal-xattr)")
    ap.add_argument("--docx", action="store_true", help="собрать docx инструкции")
    ap.add_argument("--video", action="store_true", help="собрать ролик из полноэкранных снимков")
    ap.add_argument("--version", help="версия для docx; по умолчанию из wails.json")
    ap.add_argument("--out", type=Path, default=ROOT / ".gotmp" / "guide")
    args = ap.parse_args(argv)
    if not (args.run / "shots.json").is_file():
        print(f"ОШИБКА: нет {args.run / 'shots.json'}", file=sys.stderr)
        return 1
    overrides = json.loads(args.targets.read_text(encoding="utf-8")) if args.targets else {}
    frames = place(args.run, args.os, args.assets, overrides, args.source,
                   [x.strip() for x in args.skip.split(",") if x.strip()])
    print(f"Снимков для ролика: {len(frames)}")
    if args.video:
        import slideshow  # noqa: PLC0415 — нужен только для ролика

        ok = slideshow.make([args.assets / "annot" / f"{f}.png" for f in frames],
                            args.assets / "mac-steps.mp4")
        print("Ролик: mac-steps.mp4" if ok else "::warning::ролик не собран (нет ffmpeg или он упал)")
    if args.docx:
        import build_docx  # noqa: PLC0415

        version = args.version or build_docx.read_version()
        out = build_docx.build(args.assets.parent / "guide.md", version, args.out)
        print(f"DOCX: {out}")
    return 0


if __name__ == "__main__":
    for _s in (sys.stdout, sys.stderr):
        _s.reconfigure(encoding="utf-8")
    sys.exit(main())
