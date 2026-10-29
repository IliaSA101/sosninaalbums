#!/usr/bin/env python3
"""Ролики справочника F1 в собранной программе (win-qa и задание macOS, спека 140 Т7; образец - library_exe.py, 144).

Что видно только в сборке: играет ли окно программы ролик на настоящем движке - WebView2 (Windows, webm VP9) и WKWebView
(macOS, mp4 H.264). Linux проверен в Т0 (WebKitGTK). Окно снаружи не принимает запросов, поэтому наблюдения снимает зонд
внутри окна (videos_probe.js, тег qa, SA_QA_PROBE); здесь - сценарий запусков, локальный сервер каталога, отчёт.

Каталог и ролики отдаёт локальный сервер 127.0.0.1:34190 из папки --site (её собирает задание videos-site: library-testsets.mjs
и library-testvideos.mjs, подпись тестовым ключом, открытый ключ - site/key.txt). Два запуска одной программы с общей папкой стенда qa:
  main     согласие, список, скачивание службой Go, игра каждого ролика путём окна (fetch -> Blob -> <video>) и в окне справочника;
  corrupt  между запусками испорчено начало файла ролика при том же размере: «Ролик не открылся», «Скачать заново», игра после замены.
Один прогон собирает все провалы; при нехватке входов зависимые пункты падают с причиной, независимые идут дальше.
Снимок экрана конца каждого запуска - только macOS (screencapture); итог и дамп состояния - в --out.
Запуск: Windows - --kit <набор qa-kit>; macOS - --app <папка .app>. Только Windows и macOS; stdlib. Файл в UTF-8.
"""

import argparse
import functools
import http.server
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import threading

HERE = os.path.dirname(os.path.abspath(__file__))


def _sibling(name):
    """Соседний модуль по пути: под `python -I` папка скрипта в sys.path не входит."""
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


vc = _sibling("videos_checks")
lx = _sibling("library_exe")  # extract_exe, stand_config, Report; тянет agents_exe и library_run
ax = lx.ax
rn = _sibling("library_run")
rn.PROBE_JS = os.path.join(HERE, "videos_probe.js")  # probe_script() читает этот путь при вызове

STAND = "qa"
DATA = "SosninaAlbums-stand-" + STAND
PORT = 34190  # на него смотрит BASE в scripts/library-testvideos.mjs
IDS = ["filter", "export"]
TITLES = {"filter": "Фильтр ленты", "export": "Экспорт в JPEG"}
VICTIM = "export"


def dirs_for(platform, environ=None):
    """(папка настроек стенда, папка кэша роликов, где искать log.txt) по ОС; пусто - ОС не поддерживается."""
    env = os.environ if environ is None else environ
    if platform == "win32":
        roaming, local = env.get("APPDATA", ""), env.get("LOCALAPPDATA", "")
        if not roaming or not local:
            return None
        return os.path.join(roaming, DATA), os.path.join(local, DATA, "guide-videos")
    if platform == "darwin":
        home = env.get("HOME", "")
        if not home:
            return None
        return os.path.join(home, "Library", "Application Support", DATA), os.path.join(home, "Library", "Caches", DATA, "guide-videos")
    return None


# ---------- локальный сервер каталога ----------

class Feed:
    """Раздача папки site на 127.0.0.1:PORT с журналом GET (пути без параметров)."""

    def __init__(self, site, port=PORT):
        self.requests = []
        feed = self

        class Handler(http.server.SimpleHTTPRequestHandler):
            def log_message(self, fmt, *args):  # журнал - в feed.requests, а не в stderr
                pass

            def do_GET(self):
                feed.requests.append(self.path.split("?")[0])
                super().do_GET()

        self.httpd = http.server.ThreadingHTTPServer(("127.0.0.1", port), functools.partial(Handler, directory=site))
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)

    def start(self):
        self.thread.start()

    def stop(self):
        self.httpd.shutdown()
        self.httpd.server_close()


# ---------- запуск с зондом ----------

class SnapSampler:
    """Вместо замера памяти: stop() вызывается до снятия процесса - на macOS здесь снимок экрана с окном программы."""

    snap_dir = ""
    name = ""

    def __init__(self, pid):
        self.pid, self.peak, self.n = pid, {"own": 0, "tree": 0}, 0

    def start(self):
        pass

    def sample_now(self):
        return None

    def stop(self):
        if sys.platform != "darwin" or not self.snap_dir:
            return
        try:
            subprocess.run(["screencapture", "-x", os.path.join(self.snap_dir, "%s-end.png" % self.name)], timeout=30, capture_output=True)
        except (OSError, subprocess.SubprocessError):
            pass


def run_probe(exe, env, work, name, cfg, timeout):
    SnapSampler.snap_dir, SnapSampler.name = work, name
    return rn.run_probe(exe, env, work, name, cfg, timeout, sampler_cls=SnapSampler)


def exe_of_app(app):
    """Единственный исполняемый файл внутри <программа>.app/Contents/MacOS."""
    d = os.path.join(app, "Contents", "MacOS")
    names = sorted(n for n in os.listdir(d) if os.path.isfile(os.path.join(d, n)))
    if len(names) != 1:
        raise RuntimeError("в %s нужен ровно один файл, найдено %s" % (d, names))
    return os.path.join(d, names[0])


# ---------- отчёт ----------

class Report(lx.Report):
    def markdown(self):
        out = ["## Ролики справочника F1 в программе (140 Т7, %s)" % self.platform, "", "| пункт | итог | подробности |", "|---|---|---|"]
        for rid, st, d in self.rows:
            out.append("| %s | %s | %s |" % (rid, st, d.replace("|", "/").replace("\n", " ")))
        return "\n".join(out + [""] + self.notes) + "\n"

    platform = ""


OWNER_LEFT = """### Остаток владельцу (в CI не проверяется)

- Чёткость, плавность и вид кнопок плеера глазами; Esc закрывает только ролик (mac-manual M8): здесь проверено, что ролик декодируется, время идёт, кадр верного размера.
- Звук: ролики без звука, автоплей с `muted`.
- Другие версии macOS и Windows, чем на раннере; антивирус."""


def read_log(data):
    try:
        with open(os.path.join(data, "log.txt"), encoding="utf-8", errors="replace") as f:
            return f.read()
    except OSError:
        return ""


def dump_text(dirs, runs, feed):
    parts = ["состояние на конец прогона"]
    for d in dirs:
        parts.append("папка %s: %s" % (d, ax.listing(d)))
    parts.append("запросы к серверу каталога: %s" % feed.requests)
    for name, run in runs.items():
        parts.append("запуск %s: done=%s alive=%s code=%s %s с" % (name, run["done"], run["alive"], run["code"], run["seconds"]))
        parts.append("записи %s:\n%s" % (name, "\n".join(json.dumps(r, ensure_ascii=False)[:700] for r in run["records"])))
    return "\n".join(parts)


# ---------- сценарий ----------

def scenario(exe, site, out, platform, data, cache, rep, runs, feed):
    kind = vc.KIND[platform]
    work = os.path.join(out, "work")
    with open(os.path.join(site, "key.txt"), encoding="utf-8") as f:
        key = f.read().strip()
    env = {k: v for k, v in os.environ.items() if not k.startswith("SA_")}
    env.update(SA_STAND=STAND, SA_LIBRARY_URL="http://127.0.0.1:%d/" % PORT, SA_LIBRARY_KEY=key)
    shutil.rmtree(data, ignore_errors=True)
    shutil.rmtree(os.path.dirname(cache), ignore_errors=True)
    ax.write_text(os.path.join(data, "config.json"), lx.stand_config())

    cfg = {"mode": "main", "ids": IDS, "titles": TITLES}
    run = runs["main"] = run_probe(exe, env, work, "main", cfg, 420)
    recs = run["records"]
    rep.judge("140-0 окно отвечает зонду (%s)" % rep.platform, vc.check_ready(run))
    rep.judge("140-1 согласие и список с локального сервера", vc.check_update(recs))
    rep.judge("140-1 до скачивания готовых роликов нет", vc.check_list_before(recs, IDS))
    rep.judge("140-2 скачивание службой Go: ролики вида %s готовы" % kind, vc.check_download(recs, IDS))
    sizes = vc.sizes_of(recs)
    for i in IDS:
        rep.judge("140-3 ролик %s играет в окне (%s): кадр, время идёт, ошибок нет" % (i, kind), vc.check_play(recs, i, kind, sizes.get(i)))
    rep.judge("140-3 ролик в окне справочника (Modal): играет", vc.check_ui_play(recs, kind))
    rep.judge("140-4 сервер каталога: запрошен свой вид (%s), чужого нет" % kind, vc.check_requests(feed.requests, IDS, kind))
    rep.judge("140-4 кэш роликов: байты равны раздаче, остатков нет", vc.check_cache(cache, site, IDS, kind))

    victim = vc.cache_file(cache, VICTIM, kind)
    if not victim:
        why = "нет файла ролика %s в кэше %s: проверка испорченного не выполнена" % (VICTIM, cache)
        for rid in ("140-5 испорченный ролик не открывается", "140-5 карточка: «Ролик не открылся», «Показать файл», «Скачать заново»",
                    "140-6 «Скачать заново» заменила файл", "140-6 после замены ролик играет", "140-6 после замены ролик играет в окне справочника"):
            rep.add(rid, False, why)
        return
    n = vc.corrupt_file(victim)
    cfg = {"mode": "corrupt", "ids": IDS, "titles": TITLES, "victim": VICTIM}
    run = runs["corrupt"] = run_probe(exe, env, work, "corrupt", cfg, 300)
    recs = run["records"]
    rep.add("140-5 запуск 2: испорчено %d байт начала файла, размер тот же" % n, True, os.path.basename(victim))
    rep.judge("140-5 запуск 2: окно отвечает зонду", vc.check_ready(run))
    rep.judge("140-5 испорченный ролик не открывается (путь окна)", vc.check_corrupt_blob(recs, kind))
    rep.judge("140-5 карточка: «Ролик не открылся», «Показать файл», «Скачать заново»", vc.check_ui_broken(recs))
    rep.judge("140-6 «Скачать заново» заменила файл", vc.check_redo(recs, VICTIM, sizes.get(VICTIM)))
    rep.judge("140-6 после замены ролик играет", vc.check_play(recs, VICTIM, kind, sizes.get(VICTIM), "play-redo"))
    rep.judge("140-6 после замены ролик играет в окне справочника", vc.check_ui_play(recs, kind, "ui-play-redo"))
    rep.judge("140-6 кэш после замены: байты равны раздаче", vc.check_cache(cache, site, IDS, kind))


def finish(rep, out, dirs, runs, feed, log_text):
    rep.judge("140-7 log.txt стенда без паник, адресов и путей в строках про ролики", vc.check_log(log_text))
    rep.notes.append(OWNER_LEFT)
    dump = dump_text(dirs, runs, feed) + "\nlog.txt (хвост):\n" + "\n".join(log_text.splitlines()[-60:])
    ax.write_text(os.path.join(out, "state-dump.txt"), dump)
    ax.write_text(os.path.join(out, "summary.md"), rep.markdown())
    if rep.failed():
        print("\n" + dump, flush=True)
    skipped = [r for r in rep.rows if r[1] == "skip"]
    print("\nпунктов: %d, провалов: %d, не проверено: %d" % (len(rep.rows), len(rep.failed()), len(skipped)), flush=True)
    return 1 if rep.failed() else 0


def main():
    ax.use_utf8_streams()
    ap = argparse.ArgumentParser()
    ap.add_argument("--kit", help="набор qa-kit (Windows)")
    ap.add_argument("--app", help="папка .app (macOS)")
    ap.add_argument("--site", required=True, help="папка раздачи: catalog.json, ролики, key.txt")
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    platform = sys.platform
    got = dirs_for(platform)
    if got is None or not (a.kit if platform == "win32" else a.app):
        print("нужны Windows с --kit или macOS с --app (и APPDATA/LOCALAPPDATA или HOME)", file=sys.stderr)
        return 2
    data, cache = got
    out = os.path.abspath(a.out)
    os.makedirs(out, exist_ok=True)
    rep, runs = Report(), {}
    rep.platform = "Windows, WebView2, webm" if platform == "win32" else "macOS, WKWebView, mp4"
    feed = Feed(os.path.abspath(a.site))
    feed.start()
    try:
        exe = lx.extract_exe(a.kit, os.path.join(out, "work", "exe")) if platform == "win32" else exe_of_app(os.path.abspath(a.app))
        scenario(exe, os.path.abspath(a.site), out, platform, data, cache, rep, runs, feed)
    except Exception:  # сбой самого сценария - в итог, дамп всё равно соберётся
        import traceback
        rep.add("сценарий", False, "исключение: %s" % traceback.format_exc().replace("\n", " / "))
    finally:
        feed.stop()
    return finish(rep, out, [data, os.path.dirname(cache)], runs, feed, read_log(data))


if __name__ == "__main__":
    sys.exit(main())
