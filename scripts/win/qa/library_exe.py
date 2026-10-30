#!/usr/bin/env python3
"""Образцы наборов библиотеки в exe (win-qa, спека 0.31/144, решение 0.31-144-59): qa-exe на Windows-раннере с настоящим GitHub.

Что видно только в exe: раздача /media/lp/ через WebView2 и Wails, удаление испорченного листа на NTFS,
листание плиток без отмены r.Context() (Wails её не передаёт), папка настроек, переход 302 на CDN ассетов.
Окно exe запросов не принимает снаружи (CDP невозможен, 118-1), поэтому запросы шлёт зонд внутри окна:
library_probe.js выполняется Go (SA_QA_PROBE, internal/app/qaprobe_qa.go) и пишет наблюдения в файл. Здесь -
сценарий запусков и отчёт; запуск и память - library_run.py, вердикты - library_checks.py (чистые функции, их тесты -
test_library_exe.py).

Три запуска одного exe с общей папкой стенда qa (SA_STAND=qa, SA_LIBRARY_URL не задан - источник GitHub):
  main     обновление списка, листы образцов вместе со «Скачать» набора, годные ответы, обходы пути, листание
           (два круга, память процесса между ними), отзыв согласия;
  corrupt  между запусками подменяется байт листа на диске: первый показ в новом сеансе - 404, файл удалён;
  refetch  новый сеанс: лист докачан, плитки вернулись.
Один прогон собирает все провалы; в конце дамп состояния и код 1. Запуск: python library_exe.py --kit <набор> --out <папка>.
Только Windows; stdlib. Файл в UTF-8 (запускается python, не PowerShell).
"""

import argparse
import importlib.util
import json
import os
import shutil
import sys
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))


def _sibling(name):
    """Соседний модуль по пути: под `python -I` папка скрипта в sys.path не входит."""
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


ax = _sibling("agents_exe")  # Report, write_text, listing, use_utf8_streams
ck = _sibling("library_checks")
rn = _sibling("library_run")

STAND = "qa"
DATA = "SosninaAlbums-stand-" + STAND


def stand_config(allowed=True):
    """config.json стенда: как у agents_exe, плюс согласие на обращение к GitHub (prefs.LibraryNet)."""
    tours = ",".join('"%s":"done"' % t for t in ("first", "folders", "library", "autobuild", "timeline", "layouts", "photo", "export"))
    return '{"level":"simple","hideStart":true,"assoc":"off","tours":{%s},"update":{"off":true},"libraryNet":{"allowed":%s}}' % (
        tours, "true" if allowed else "false")


def set_consent(cfg_path, allowed=True):
    """Согласие в готовом config.json; остальное (принятый seq списка) сохраняется."""
    with open(cfg_path, encoding="utf-8") as f:
        cfg = json.load(f)
    net = cfg.get("libraryNet") if isinstance(cfg.get("libraryNet"), dict) else {}
    net["allowed"] = bool(allowed)
    cfg["libraryNet"] = net
    ax.write_text(cfg_path, json.dumps(cfg, ensure_ascii=False))




# ---------- отчёт ----------

class Report(ax.Report):
    def __init__(self):
        super().__init__()
        self.notes = []

    def markdown(self):
        out = ["## win-qa: образцы наборов в exe (144)", "", "| пункт | итог | подробности |", "|---|---|---|"]
        for rid, st, d in self.rows:
            out.append("| %s | %s | %s |" % (rid, st, d.replace("|", "/").replace("\n", " ")))
        return "\n".join(out + [""] + self.notes) + "\n"

    def judge(self, rid, verdict):
        self.add(rid, verdict[0], verdict[1])


OWNER_LEFT = """### Остаток владельцу (в CI не проверяется)

- Картинки плиток глазами: зонд проверяет тип, размер и тот же лист по контрольной сумме, но не рисует плитки в окне (UIA не видит пикселей).
- Антивирус Kaspersky (держит `.dl-*.tmp`, `os.Remove` листа): на раннере только Defender.
- Настоящая прокрутка колесом и отмена `<img>` вне экрана: листание здесь - потоки `fetch` с обрывом (`AbortController`) по тем же адресам.
- Windows 10 и ARM-раннер: прогон на `windows-2025`.
- Образцы наборов с `minApp` выше 0.25.1 (qa-exe набора - 0.25.x): в нём они серые, плиток не дают."""


def dump_text(data_dirs, runs):
    parts = ["состояние на конец прогона"]
    for d in data_dirs:
        parts.append("папка %s: %s" % (d, ax.listing(d)))
        parts.append("library-previews: %s" % ax.listing(os.path.join(d, "library-previews")))
    for name, run in runs.items():
        parts.append("запуск %s: done=%s alive=%s code=%s %s с, пик %s, снимков %s" % (
            name, run["done"], run["alive"], run["code"], run["seconds"], run["peak"], run["samples"]))
        parts.append("записи %s:\n%s" % (name, "\n".join(json.dumps(r, ensure_ascii=False)[:600] for r in run["records"])))
    return "\n".join(parts)


def read_log(data):
    try:
        with open(os.path.join(data, "log.txt"), encoding="utf-8", errors="replace") as f:
            return f.read()
    except OSError:
        return ""


def extract_exe(kit, dest):
    """exe qa-сборки из zip набора (имя внутри архива проверяется: ровно один файл SosninaAlbums-<версия>.exe)."""
    zp = os.path.join(kit, "zip", "SosninaAlbums-%s-qa-windows-x64.zip" % ck.VERSION)
    want = "SosninaAlbums-%s.exe" % ck.VERSION
    with zipfile.ZipFile(zp) as z:
        names = [n for n in z.namelist() if n.endswith("/" + want) or n == want]
        if len(names) != 1:
            raise RuntimeError("в %s нужен ровно один %s, найдено %d" % (zp, want, len(names)))
        data = z.read(names[0])
    os.makedirs(dest, exist_ok=True)
    exe = os.path.join(dest, want)
    with open(exe, "wb") as f:
        f.write(data)
    return exe


def main():
    ax.use_utf8_streams()
    ap = argparse.ArgumentParser()
    ap.add_argument("--kit", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    if os.name != "nt":
        print("только Windows", file=sys.stderr)
        return 2
    out = os.path.abspath(a.out)
    os.makedirs(out, exist_ok=True)
    roaming, local = os.environ.get("APPDATA", ""), os.environ.get("LOCALAPPDATA", "")
    if not roaming or not local:
        print("нет APPDATA/LOCALAPPDATA", file=sys.stderr)
        return 2
    data, data_local = os.path.join(roaming, DATA), os.path.join(local, DATA)
    rep, runs = Report(), {}
    try:
        scenario(a.kit, out, data, rep, runs)
    except Exception:  # сбой самого сценария - в итог, дамп всё равно соберётся
        import traceback
        rep.add("сценарий", False, "исключение: %s" % traceback.format_exc().replace("\n", " / "))
    return finish(rep, out, [data, data_local], runs, read_log(data))


def stand_leftovers(data):
    """Остатки .dl-*.tmp в обеих папках данных стенда (Roaming и Local)."""
    return ck.tmp_leftovers([data, os.path.join(os.environ["LOCALAPPDATA"], DATA)])


def scenario(kit, out, data, rep, runs):
    work = os.path.join(out, "work")
    exe = extract_exe(kit, os.path.join(work, "exe"))
    env = {k: v for k, v in os.environ.items() if k not in ("SA_LIBRARY_URL", "SA_LIBRARY_KEY", "SA_QA_PROBE", "SA_QA_PROBE_OUT")}
    env["SA_STAND"] = STAND  # источник списка - настоящий GitHub: SA_LIBRARY_URL не задан
    shutil.rmtree(data, ignore_errors=True)
    cfg_path = os.path.join(data, "config.json")
    ax.write_text(cfg_path, stand_config())
    victim = phase_main(exe, env, work, data, rep, runs)
    sha = ((ck.catalog_pack(data, victim) or {}).get("preview") or {}).get("sha256", "") if victim else ""
    sha = sha if ck.valid_sha(sha) else ""  # из каталога (чужие данные): в имя файла идёт только 64 hex-цифры
    path = ck.sheet_path(data, sha) if sha else ""
    if not (sha and os.path.isfile(path)):
        why = "нет набора с листом на диске для подмены (набор %r, лист %r): проверка не выполнена" % (victim, path)
        rep.add("144-2 подменённый лист: первый показ - 404, плитки нет, файл удалён", False, why)
        rep.add("144-2 лист докачан в новом сеансе", False, why)
        return
    # запуск 2: подмена байта листа на диске, первый показ в новом сеансе (кэш разбора холодный)
    set_consent(cfg_path)
    ck.corrupt_sheet(path)
    run = runs["corrupt"] = rn.run_probe(exe, env, work, "corrupt", {"mode": "corrupt", "victim": victim}, 180)
    rep.judge("144-2 подменённый лист (байт, тот же размер): первый показ - 404, плитки нет, файл удалён на NTFS",
              ck.check_corrupt(run["records"], os.path.exists(path)))
    left = stand_leftovers(data)
    rep.add("144-2 после запуска 2 нет остатков .dl-*.tmp", not left, "найдено: %s" % left)
    # запуск 3: ещё один новый сеанс - лист докачивается
    set_consent(cfg_path)
    run = runs["refetch"] = rn.run_probe(exe, env, work, "refetch", {"mode": "refetch", "victim": victim}, 240)
    rep.judge("144-2 лист докачан в новом сеансе, плитки вернулись", ck.check_refetch(run["records"], ck.sheet_matches(path, sha)))
    left = stand_leftovers(data)
    rep.add("144-2 после запуска 3 нет остатков .dl-*.tmp", not left, "найдено: %s" % left)


def phase_main(exe, env, work, data, rep, runs):
    """Запуск 1: всё, что можно за один сеанс. Возвращает набор, чей лист подменяется в запуске 2 ('' - не выбран)."""
    cfg = {"mode": "main", "paths": ck.PATHS, "passes": ck.PASSES, "roundMaxMs": ck.ROUND_MAX_MS}
    run = runs["main"] = rn.run_probe(exe, env, work, "main", cfg, 600)
    recs = run["records"]
    rep.judge("144-0 окно отвечает зонду (WebView2 + Wails, qa-exe %s)" % ck.VERSION, ck.check_ready(run))
    rep.judge("144-0 обновление списка с настоящего GitHub (TLS, согласие из config.json)", ck.check_update(recs))
    rep.judge("144-4 листы докачаны, плитки «Можно скачать» получены", ck.check_tiles(recs))
    rep.judge("144-4 задание образцов и «Скачать» набора рядом не мешают друг другу", ck.check_overlap(recs))
    rep.judge("144-4 папка настроек стенда содержит library-previews\\ с листами после согласия", ck.check_sheets_dir(os.path.join(data, "library-previews")))
    rep.judge("144-1 /media/lp/ через WebView2: годные ответы", ck.check_good(recs))
    rep.judge("144-1 /media/lp/ через WebView2: обходы пути - 404 без ответа другого обработчика", ck.check_paths(recs))
    rep.judge("144-3 быстрое листание: ответы клеток приходят", ck.check_stress(recs))
    rep.judge("144-3 процесс жив после листания", ck.check_alive(run))
    rep.judge("144-3 память не растёт без предела (числа - в подробностях)", ck.check_memory(recs, run))
    rep.judge("144-5 отзыв согласия: плиток нет, клетки 404", ck.check_revoke(recs))
    rep.notes.append(ck.memory_note(recs, run))
    left = stand_leftovers(data)
    rep.add("144-2 после запуска 1 нет остатков .dl-*.tmp", not left, "найдено: %s" % left)
    vic = ck.step_of(recs, "victim") or {}
    return vic.get("victim", "") if vic.get("ok") else ""


def finish(rep, out, data_dirs, runs, log_text):
    """Журнал стенда - пунктом, дамп состояния и итоги в файлы; код 1, если есть провалы."""
    rep.judge("144-6 log.txt стенда без паник, адресов и путей", ck.check_log(log_text))
    rep.notes.append(OWNER_LEFT)
    dump = dump_text(data_dirs, runs) + "\nlog.txt (хвост):\n" + "\n".join(log_text.splitlines()[-60:])
    ax.write_text(os.path.join(out, "state-dump.txt"), dump)
    ax.write_text(os.path.join(out, "summary.md"), rep.markdown())
    if rep.failed():
        print("\n" + dump, flush=True)
    skipped = [r for r in rep.rows if r[1] == "skip"]
    print("\nпунктов: %d, провалов: %d, не проверено: %d" % (len(rep.rows), len(rep.failed()), len(skipped)), flush=True)
    return 1 if rep.failed() else 0


if __name__ == "__main__":
    sys.exit(main())
