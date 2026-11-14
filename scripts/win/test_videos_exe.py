"""Логика videos_exe.py (ролики справочника F1 в программе, спека 140 Т7) без Windows и macOS: вердикты над записями зонда,
кэш и порча файла, журнал сервера, запуск на поддельной программе, сам videos_probe.js под node с подставным окном.
Сам прогон на настоящей сборке - задания videos-win и videos-mac в win-qa.yml."""

import importlib.util
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
QA = os.path.join(HERE, "qa")


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(QA, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


vc = load("videos_checks")
vx = load("videos_exe")

GOOD = {"status": 200, "ct": "video/webm", "bytes": 1000, "readyState": 4, "w": 1280, "h": 720, "duration": 6.0,
        "times": [0.1, 1.1, 2.1, 3.1], "error": None, "events": ["playing"]}


def rec(step, ok=True, **kw):
    return dict({"step": step, "ok": ok}, **kw)


def item(i, ready):
    return {"id": i, "version": 1, "seconds": 6, "size": 1000, "ready": ready}


class Verdicts(unittest.TestCase):
    def test_good_play_passes_for_both_kinds(self):
        self.assertTrue(vc.judge_playing(GOOD, "webm", 1000)[0])
        self.assertTrue(vc.judge_playing(dict(GOOD, ct="video/mp4"), "mp4", 1000)[0])

    def test_every_play_breakage_is_named(self):
        for bad, word in ((dict(GOOD, readyState=1), "readyState"), (dict(GOOD, times=[0, 0, 0, 0]), "время"),
                          (dict(GOOD, error={"code": 4}), "video.error"), (dict(GOOD, w=0), "кадр"),
                          (dict(GOOD, ct="video/mp4"), "Content-Type"), (dict(GOOD, bytes=5), "байт"),
                          (dict(GOOD, status=404), "404"), (dict(GOOD, times=[]), "время"), (dict(GOOD, playErr="x"), "play()")):
            ok, d = vc.judge_playing(bad, "webm", 1000)
            self.assertFalse(ok, bad)
            self.assertIn(word, d)

    def test_missing_or_failed_step_is_a_failure_with_reason(self):
        self.assertFalse(vc.check_play([], "filter", "webm")[0])
        ok, d = vc.check_play([rec("play:filter", False, err="boom", screen="экран")], "filter", "webm")
        self.assertFalse(ok)
        self.assertIn("boom", d)

    def test_list_and_download(self):
        self.assertTrue(vc.check_list_before([rec("list-before", items=[item("a", False), item("b", False)])], ["a", "b"])[0])
        self.assertFalse(vc.check_list_before([rec("list-before", items=[item("a", True), item("b", False)])], ["a", "b"])[0])
        self.assertFalse(vc.check_list_before([rec("list-before", items=[])], ["a", "b"])[0])  # пустой список: каталог не принят
        both = [rec("download", ms=5, items=[item("a", True), item("b", True)]), rec("list-after", items=[item("a", True), item("b", True)])]
        self.assertTrue(vc.check_download(both, ["a", "b"])[0])
        self.assertFalse(vc.check_download([both[0], rec("list-after", items=[item("a", True), item("b", False)])], ["a", "b"])[0])

    def test_corrupt_blob_verdict(self):
        broken = {"status": 200, "error": {"code": 4}, "readyState": 0, "w": 0, "h": 0}
        self.assertTrue(vc.check_corrupt_blob([rec("play-corrupt", **broken)], "webm")[0])
        self.assertFalse(vc.check_corrupt_blob([rec("play-corrupt", **GOOD)], "webm")[0])  # сыграл: сбой не пойман
        self.assertFalse(vc.check_corrupt_blob([rec("play-corrupt", **dict(broken, status=404))], "webm")[0])

    def test_ui_broken_needs_all_parts(self):
        good = dict(broken=True, reveal=True, redo=True, watch=False, windowGone=True)
        self.assertTrue(vc.check_ui_broken([rec("ui-broken", **good)])[0])
        for k, v in (("broken", False), ("reveal", False), ("redo", False), ("watch", True), ("windowGone", False)):
            self.assertFalse(vc.check_ui_broken([rec("ui-broken", **dict(good, **{k: v}))])[0], k)

    def test_redo_and_ui_play(self):
        it = item("b", True)
        self.assertTrue(vc.check_redo([rec("ui-redo", back=True, items=[it])], "b", 1000)[0])
        self.assertFalse(vc.check_redo([rec("ui-redo", back=False, items=[it])], "b", 1000)[0])
        self.assertFalse(vc.check_redo([rec("ui-redo", back=True, items=[item("b", False)])], "b", 1000)[0])
        self.assertTrue(vc.check_ui_play([rec("ui-play", **dict(GOOD, inDialog=True))], "webm")[0])
        self.assertFalse(vc.check_ui_play([rec("ui-play", **dict(GOOD, inDialog=False))], "webm")[0])

    def test_requests_kinds(self):
        ok = ["/catalog.json", "/a-1.webm", "/b-1.webm"]
        self.assertTrue(vc.check_requests(ok, ["a", "b"], "webm")[0])
        self.assertFalse(vc.check_requests(ok, ["a", "b"], "mp4")[0])  # mac получил webm: вид перепутан
        self.assertFalse(vc.check_requests(["/catalog.json", "/a-1.webm"], ["a", "b"], "webm")[0])
        self.assertFalse(vc.check_requests([], ["a"], "webm")[0])

    def test_log(self):
        self.assertTrue(vc.check_log("ролик скачан\nнормально")[0])
        self.assertFalse(vc.check_log("panic: runtime error")[0])
        self.assertFalse(vc.check_log("ролик: http://127.0.0.1:34190/a.webm")[0])
        self.assertTrue(vc.check_log("")[0])


class Disk(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.site, self.cache = os.path.join(self.tmp.name, "site"), os.path.join(self.tmp.name, "cache")
        os.makedirs(self.site)
        os.makedirs(self.cache)

    def put(self, d, name, data):
        with open(os.path.join(d, name), "wb") as f:
            f.write(data)

    def test_corrupt_keeps_size_and_changes_bytes(self):
        data = bytes(range(256)) * 4
        self.put(self.cache, "a-1.webm", data)
        p = os.path.join(self.cache, "a-1.webm")
        self.assertEqual(vc.corrupt_file(p), 256)
        with open(p, "rb") as f:
            new = f.read()
        self.assertEqual(len(new), len(data))
        self.assertNotEqual(new[:256], data[:256])
        self.assertEqual(new[256:], data[256:])
        self.put(self.cache, "tiny.webm", b"abc")  # короче предела: портится всё, размер тот же
        self.assertEqual(vc.corrupt_file(os.path.join(self.cache, "tiny.webm")), 3)
        self.assertEqual(os.path.getsize(os.path.join(self.cache, "tiny.webm")), 3)
        self.put(self.cache, "empty.webm", b"")
        self.assertEqual(vc.corrupt_file(os.path.join(self.cache, "empty.webm")), 0)

    def test_cache_verdicts(self):
        for i in ("a", "b"):
            self.put(self.site, i + "-1.webm", i.encode() * 10)
            self.put(self.cache, i + "-1.webm", i.encode() * 10)
        self.assertTrue(vc.check_cache(self.cache, self.site, ["a", "b"], "webm")[0])
        self.put(self.cache, "gv-1.tmp", b"x")
        self.assertFalse(vc.check_cache(self.cache, self.site, ["a", "b"], "webm")[0])  # остаток скачивания
        os.remove(os.path.join(self.cache, "gv-1.tmp"))
        self.put(self.cache, "b-1.webm", b"B" * 10)
        self.assertFalse(vc.check_cache(self.cache, self.site, ["a", "b"], "webm")[0])  # байты разошлись
        self.put(self.cache, "b-1.webm", b"b" * 10)
        self.put(self.cache, "a-1.mp4", b"x")
        self.assertFalse(vc.check_cache(self.cache, self.site, ["a", "b"], "webm")[0])  # чужой вид
        self.assertFalse(vc.check_cache(os.path.join(self.tmp.name, "net"), self.site, ["a"], "webm")[0])
        self.assertEqual(vc.cache_file(self.cache, "a", "webm"), os.path.join(self.cache, "a-1.webm"))
        self.assertEqual(vc.cache_file(self.cache, "zz", "webm"), "")

    def test_dirs_for(self):
        self.assertEqual(vx.dirs_for("darwin", {"HOME": "/h"}), ("/h/Library/Application Support/SosninaAlbums-stand-qa", "/h/Library/Caches/SosninaAlbums-stand-qa/guide-videos"))
        self.assertIsNone(vx.dirs_for("darwin", {}))
        self.assertIsNone(vx.dirs_for("win32", {"APPDATA": "C:\\r"}))
        self.assertIsNone(vx.dirs_for("linux", {"HOME": "/h"}))
        w = vx.dirs_for("win32", {"APPDATA": "R", "LOCALAPPDATA": "L"})
        self.assertTrue(w[0].endswith("SosninaAlbums-stand-qa") and w[1].endswith("guide-videos"))

    def test_feed_logs_get_requests(self):
        self.put(self.site, "catalog.json", b"{}")
        feed = vx.Feed(self.site, port=0)
        port = feed.httpd.server_address[1]
        feed.start()
        try:
            with urllib.request.urlopen("http://127.0.0.1:%d/catalog.json?x=1" % port, timeout=5) as r:
                self.assertEqual(r.read(), b"{}")
            with self.assertRaises(Exception):
                urllib.request.urlopen("http://127.0.0.1:%d/nope.webm" % port, timeout=5)
        finally:
            feed.stop()
        self.assertEqual(feed.requests, ["/catalog.json", "/nope.webm"])

    def test_exe_of_app(self):
        d = os.path.join(self.tmp.name, "x.app", "Contents", "MacOS")
        os.makedirs(d)
        self.put(d, "prog", b"x")
        self.assertEqual(vx.exe_of_app(os.path.join(self.tmp.name, "x.app")), os.path.join(d, "prog"))
        self.put(d, "second", b"x")
        with self.assertRaises(RuntimeError):
            vx.exe_of_app(os.path.join(self.tmp.name, "x.app"))


FAKE_EXE = """#!{py}
import json, os
out = os.environ["SA_QA_PROBE_OUT"]
with open(out, "a", encoding="utf-8") as f:
    f.write(json.dumps({{"step": "ready", "ok": True, "ms": 5}}) + "\\n")
    f.write(json.dumps({{"step": "done", "ok": True}}) + "\\n")
"""


class RunProbe(unittest.TestCase):
    def test_probe_script_is_videos_probe_and_run_collects_records(self):
        tmp = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, tmp, True)
        exe = os.path.join(tmp, "exe")
        with open(exe, "w", encoding="utf-8") as f:
            f.write(FAKE_EXE.format(py=sys.executable))
        os.chmod(exe, os.stat(exe).st_mode | stat.S_IXUSR)
        run = vx.run_probe(exe, dict(os.environ), os.path.join(tmp, "w"), "main", {"mode": "main", "ids": ["a"], "titles": {}}, 20)
        self.assertTrue(run["done"])
        self.assertTrue(vc.check_ready(run)[0])
        with open(os.path.join(tmp, "w", "main.js"), encoding="utf-8") as f:
            js = f.read()
        self.assertIn("DownloadGuideVideos", js)  # скрипт ролика, а не библиотеки
        self.assertNotIn("FetchPackPreviews", js)


HARNESS = r"""
// Подставное окно: Go-привязки, журнал событий и маленькая модель справочника
const [, , script, mode] = process.argv
const bad = process.env.HARNESS_BAD || ''
const ui = { guide: false, video: false, broken: false, fixed: false, q: false, sel: false }
const corrupt = () => mode === 'corrupt' && !ui.fixed
const mk = (text, onclick) => ({ textContent: text, click: onclick || (() => {}) })
const items = (ready) => ['filter', 'export'].map((id) => ({ id, version: 1, seconds: 6, size: 1000, ready }))
let downloaded = mode === 'corrupt'
let tick = 0
const videoEl = () => {
  const broken = corrupt()
  const v = {
    muted: false, playsInline: false, preload: '', src: '', paused: true, networkState: 2,
    addEventListener() {}, setAttribute() {}, remove() {}, closest: () => ({}),
    play: async () => { v.paused = false },
    get readyState() { return broken ? 0 : 4 },
    get videoWidth() { return broken ? 0 : 1280 },
    get videoHeight() { return broken ? 0 : 720 },
    get duration() { return broken ? NaN : 6 },
    get error() { return broken ? { code: 4, message: 'decode' } : null },
    get currentTime() { return broken || bad === 'still' ? 0 : ++tick },
  }
  return v
}
globalThis.window = {
  go: { app: { App: {
    GuideVideos: async () => items(downloaded),
    SetLibraryNet: async () => ({ allowed: true }),
    UpdateLibrary: async () => ({ available: 2 }),
    DownloadGuideVideos: async () => { downloaded = true; return items(true) },
  } } },
  runtime: { EventsOn() {}, EventsEmit: (n, s) => console.log(s) },
}
globalThis.navigator = { userAgent: 'harness' }
globalThis.fetch = async () => ({ ok: true, status: 200, headers: { get: () => (bad === 'type' ? 'text/plain' : 'video/' + (process.env.HARNESS_KIND || 'webm')) }, blob: async () => ({ size: 1000 }) })
globalThis.URL = { createObjectURL: () => 'blob:x', revokeObjectURL() {} }
globalThis.Blob = class { constructor() { this.size = 1000 } }
globalThis.Event = class { constructor(t, o) { this.type = t; Object.assign(this, o) } }
globalThis.KeyboardEvent = globalThis.Event
const pane = () => ({
  get textContent() { return 'Ролик' + (ui.broken ? ' Ролик не открылся в окне программы' : '') },
  querySelectorAll: () => {
    if (ui.broken) return [mk('Показать файл'), mk('Скачать заново', () => { ui.fixed = true; ui.broken = false })]
    return [mk('▶ Смотреть', () => { if (corrupt()) ui.broken = true; else ui.video = true })]
  },
})
globalThis.document = {
  body: {
    appendChild() {}, innerText: 'экран',
    dispatchEvent(ev) {
      if (ev.key === 'F1') ui.guide = true
      if (ev.key === 'Escape') { if (ui.video) ui.video = false; else ui.guide = false }
    },
  },
  createElement: () => videoEl(),
  querySelector(sel) {
    if (sel === '.guide') return ui.guide ? {} : null
    if (sel === '.guide .q') return ui.guide && !process.env.HARNESS_NOQ ? { set value(v) {}, dispatchEvent() { ui.q = true } } : null
    if (sel === '.guide .pane') return ui.guide && ui.sel ? pane() : null
    if (sel === 'video') return ui.video ? videoEl() : null
    return null
  },
  querySelectorAll(sel) {
    if (sel === '.guide [role=option]' && ui.guide && ui.q) return [mk('Фильтр ленты', () => { ui.sel = true }), mk('Экспорт в JPEG', () => { ui.sel = true })]
    return []
  },
}
eval(require('fs').readFileSync(script, 'utf8'))
"""


@unittest.skipUnless(shutil.which("node"), "нужен node")
class ProbeScript(unittest.TestCase):
    """videos_probe.js под node с подставным окном; записи проходят через те же check_*, что и в прогоне."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def run_js(self, mode, **extra_env):
        cfg = {"mode": mode, "ids": vx.IDS, "titles": vx.TITLES, "victim": vx.VICTIM, "pace": 0.001}
        harness = os.path.join(self.tmp.name, "harness.js")
        with open(harness, "w", encoding="utf-8") as f:
            f.write(HARNESS)
        script = os.path.join(self.tmp.name, "probe.js")
        with open(script, "w", encoding="utf-8") as f:
            f.write(vx.rn.probe_script(cfg))
        r = subprocess.run(["node", harness, script, mode], capture_output=True, timeout=60, env=dict(os.environ, **extra_env))
        self.assertEqual(r.returncode, 0, r.stderr.decode("utf-8", "replace"))
        return [json.loads(ln) for ln in r.stdout.decode("utf-8").splitlines() if ln.startswith("{")]

    def test_syntax(self):
        r = subprocess.run(["node", "--check", vx.rn.PROBE_JS], capture_output=True)
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_main_records_pass_every_check(self):
        recs = self.run_js("main")
        self.assertEqual(recs[-1], dict(recs[-1], step="done", ok=True))
        run = {"records": recs, "done": True, "alive": True, "code": None}
        self.assertEqual(vc.sizes_of(recs), {"filter": 1000, "export": 1000})
        for name, v in (("ready", vc.check_ready(run)), ("update", vc.check_update(recs)), ("before", vc.check_list_before(recs, vx.IDS)),
                        ("download", vc.check_download(recs, vx.IDS)), ("play filter", vc.check_play(recs, "filter", "webm", 1000)),
                        ("play export", vc.check_play(recs, "export", "webm", 1000)), ("ui", vc.check_ui_play(recs, "webm"))):
            self.assertTrue(v[0], (name, v[1]))

    def test_mp4_kind_is_judged_by_content_type(self):
        recs = self.run_js("main", HARNESS_KIND="mp4")
        self.assertTrue(vc.check_play(recs, "filter", "mp4", 1000)[0])
        self.assertFalse(vc.check_play(recs, "filter", "webm", 1000)[0])

    def test_main_breakage_is_seen(self):
        recs = self.run_js("main", HARNESS_BAD="still")  # время в кадре стоит
        self.assertFalse(vc.check_play(recs, "filter", "webm", 1000)[0])
        self.assertFalse(vc.check_ui_play(recs, "webm")[0])
        recs = self.run_js("main", HARNESS_BAD="type")  # Content-Type не видео
        self.assertFalse(vc.check_play(recs, "export", "webm", 1000)[0])

    def test_corrupt_mode_records_pass_every_check(self):
        recs = self.run_js("corrupt")
        self.assertEqual(recs[-1], dict(recs[-1], step="done", ok=True))
        for name, v in (("blob", vc.check_corrupt_blob(recs, "webm")), ("broken", vc.check_ui_broken(recs)),
                        ("redo", vc.check_redo(recs, vx.VICTIM, 1000)), ("replay", vc.check_play(recs, vx.VICTIM, "webm", 1000, "play-redo")),
                        ("ui replay", vc.check_ui_play(recs, "webm", "ui-play-redo"))):
            self.assertTrue(v[0], (name, v[1]))

    def test_missing_gui_is_a_failed_step_not_a_crash(self):
        recs = self.run_js("main", HARNESS_NOQ="1")  # окно без поля поиска: UI-шаг падает, остальные идут
        self.assertFalse(vc.check_ui_play(recs, "webm")[0])
        self.assertTrue(vc.check_play(recs, "filter", "webm", 1000)[0])  # независимый блок не прерван
        self.assertEqual(recs[-1]["step"], "done")


if __name__ == "__main__":
    unittest.main()
