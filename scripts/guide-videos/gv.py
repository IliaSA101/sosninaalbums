"""Ролики справочника F1 (спека 140, Т6): запись сценариев статей на стенде и кодирование.

    python3 scripts/guide-videos/gv.py all --port 34117 [--stand v]          запись всех пяти статей + encode.sh
    python3 scripts/guide-videos/gv.py autobuild,cover --port 34117 --shots   снимки на метках (отладка сценария)

Запуск — Python из venv e2e (Playwright). Стенд поднимает оркестратор; на порт владельца 34115 скрипт не ходит.
Один прогон собирает все провалы: упавшая статья не прерывает остальные, у каждой — снимок и состояние страницы
в <out>/fail/. Результат — <out>/raw/<id>.webm (+ .json со временем обрезки) и, после encode.sh, <id>-<версия>.webm/.mp4
с <id>.rec.json. Сценарий повторяем: интерфейс изменился — запуск снимает заново.
Коды выхода: 0 готово; 2 отказ (порт, стенд, фото, браузер); 4 хотя бы одна статья не снята; 3 личное в тексте страницы
или ошибки консоли.
"""

from __future__ import annotations

import argparse
import importlib
import json
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))  # плоские импорты и под `python3 -I gv.py`

import gvlib  # noqa: E402
from gvlib import Refuse  # noqa: E402

REC_VERSION = 1  # версия ролика в имени файла; переснятый ролик — +1 при публикации (design §6)


def parse_args(argv: list[str]) -> argparse.Namespace:
    ap = argparse.ArgumentParser(prog="gv.py", description="Запись роликов справочника F1")
    ap.add_argument("ids", help="all или статьи через запятую: " + ", ".join(gvlib.IDS))
    ap.add_argument("--port", type=int, required=True, help="порт стенда 34116–34199")
    ap.add_argument("--out", default=None, help="папка роликов (по умолчанию docs/private/mockups/0.30/videos основного дерева)")
    ap.add_argument("--photos", default=None, help="папка stock (по умолчанию .testphotos/stock)")
    ap.add_argument("--shots", action="store_true", help="снимок экрана на каждой метке сценария")
    ap.add_argument("--no-record", action="store_true", help="пройти сценарий без записи видео")
    ap.add_argument("--no-warm", action="store_true", help="без прогрева (кэш миниатюр и предпросмотров холодный)")
    ap.add_argument("--no-encode", action="store_true", help="не звать encode.sh")
    return ap.parse_args(argv)


def run_encode(webm: Path, out: Path, art_id: str) -> dict:
    """encode.sh → JSON {video, guide}; ошибка кодирования — Refuse с текстом ffmpeg."""
    r = subprocess.run(
        ["bash", str(gvlib.HERE / "encode.sh"), str(webm), "--id", art_id, "--version", str(REC_VERSION), "--out", str(out)],
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        raise Refuse(f"encode.sh {art_id}: {r.stderr.strip()[-400:]}")
    return json.loads(r.stdout.strip().splitlines()[-1])


def finish(res: dict, out: Path, art_id: str, encode: bool) -> dict:
    """Сырое видео — в <out>/raw с спутником; затем encode.sh. Печать длины с предупреждением вне 20–40 с."""
    if res["raw"]:
        webm = out / "raw" / f"{art_id}.webm"
        shutil.copyfile(res["raw"], webm)
        gvlib.write_sidecar(webm, {k: res[k] for k in ("id", "start", "seconds", "note")})
        if encode:
            res["encoded"] = run_encode(webm, out, art_id)
    print(f"gv: {art_id}: {res['seconds']} с {gvlib.length_note(res['seconds'])}".rstrip(), flush=True)
    return res


def main(argv: list[str]) -> int:
    a = parse_args(argv)
    ids = gvlib.check_ids(a.ids)
    gvlib.check_port(a.port)
    out = gvlib.check_out(a.out)
    out.mkdir(parents=True, exist_ok=True)
    (out / "raw").mkdir(exist_ok=True)
    import gvrec
    import pilot

    work = gvlib.ROOT / ".gotmp" / "guide-videos"
    folder = gvrec.stage_photos(work, Path(a.photos) if a.photos else None)
    results: dict[str, dict] = {}
    failed: dict[str, str] = {}
    from common import Profile

    cursor = (gvlib.DEMO / "cursor.js").read_text(encoding="utf-8")
    prof = Profile("guide", gvlib.W, gvlib.H, 1.0, "ролик F1")
    Stand = gvrec.make_stand_class()
    with Stand(a.port, work / "browser", prof, init_scripts=(cursor,)) as st:
        for art_id in ids:
            art = importlib.import_module(art_id)
            print(f"gv: {art_id}…", flush=True)
            try:
                res = gvrec.run_article(st, art, work, folder, record=not a.no_record, shots=a.shots, warm=not a.no_warm)
            except pilot.SceneFail as e:
                failed[art_id] = str(e)
                print(f"gv: {art_id}: ПРОВАЛ — {e}", file=sys.stderr)
                continue
            except Exception as e:  # noqa: BLE001 — не прерывать остальные статьи (П6)
                failed[art_id] = f"{type(e).__name__}: {str(e).splitlines()[0] if str(e) else ''}"
                print(f"gv: {art_id}: ПРОВАЛ — {failed[art_id]}", file=sys.stderr)
                continue
            results[art_id] = finish(res, out, art_id, not a.no_encode)
        st.write_console()
    run = out / "run.json"  # сливаем по статьям: прогон части статей не стирает записи остальных
    merged = gvlib.merge_run(gvlib.read_run(run), results, failed)
    run.write_text(json.dumps(merged, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    bad = [i for i, r in results.items() if r["privacy"] or r["console"]]
    for i in bad:
        print(f"gv: {i}: личное или ошибки консоли: {(results[i]['privacy'] + results[i]['console'])[:5]}", file=sys.stderr)
    if failed:
        return 4
    return 3 if bad else 0


if __name__ == "__main__":
    sys.exit(__import__("common").run_main(lambda: main(sys.argv[1:])))
