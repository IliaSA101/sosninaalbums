"""Логика volume_exe.py (win-qa, 157) без Windows: вердикты по записям зонда, подготовка альбома, ход действий Python,
запуск на поддельном exe с hook, сам volume_probe.js под node с подставным окном. Сам прогон на qa-exe - только
задание volume win-qa.yml."""

import importlib.util
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import textwrap
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
QA = os.path.join(HERE, "qa")


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(QA, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


vc = load("volume_checks")
rn = load("library_run")
vx = load("volume_exe")

TOAST = "Диск Q: снова доступен — найдено 6 фото"


def rec(step, t, **kw):
    return dict(kw, step=step, t=t, w=1000000 + t)


def ev(name, t, **kw):
    return rec("ev", t, name=name, **kw)


def v1_good():
    """Ход В1 как в design: missing 6 к 1,8 с, пустое и drive:back к 4,3 с, тост, миниатюры, холст."""
    return [
        rec("ready", 100, ok=True, ms=100),
        rec("opening", 1000),
        rec("opened", 1100, ok=True),
        ev("missing", 2800, n=6),
        rec("strip", 2900, on=True),
        ev("missing", 5300, n=0),
        ev("drive:back", 5300, drive="Q", photos=6),
        rec("strip", 5400, on=False),
        rec("toast", 5500, text=TOAST),
        rec("thumbs", 6500, n=6, total=6),
        rec("canvas", 6500, imgs=3, spreads=3),
        rec("toast", 9500, text=""),
        rec("done", 16000, ok=True),
    ]


def drop(recs, pred):
    return [r for r in recs if not pred(r)]


class V1Verdicts(unittest.TestCase):
    def all_ok(self, recs):
        return {
            "missing": vc.check_v1_missing(recs),
            "clear": vc.check_v1_clear(recs),
            "back": vc.check_v1_back(recs),
            "toast": vc.check_v1_toast(recs),
            "strip": vc.check_v1_strip(recs),
            "thumbs": vc.check_v1_thumbs(recs),
            "canvas": vc.check_v1_canvas(recs),
        }

    def test_good_run_passes_everything(self):
        for name, v in self.all_ok(v1_good()).items():
            self.assertTrue(v[0], (name, v[1]))

    def test_not_opened_fails_everything_with_reason(self):
        recs = [rec("ready", 1, ok=True), rec("opening", 5), rec("opened", 9, ok=False, err="диск Z: недоступен")]
        for name in ("missing", "clear", "back", "toast", "strip", "thumbs"):
            v = self.all_ok(recs)[name]
            self.assertFalse(v[0], name)
            self.assertIn("недоступен", v[1])
        self.assertFalse(vc.check_v1_missing([])[0])  # пусто: нет ни opening, ни opened

    def test_empty_and_garbage_records(self):
        self.assertFalse(vc.check_v1_canvas([])[0])
        self.assertFalse(vc.check_v1_toast([{"step": "toast"}, {}, {"step": "ev"}])[0])
        self.assertEqual(vc.toast_times([{"step": "toast", "text": None}]), [])

    def test_missing_wrong_count_late_or_absent(self):
        recs = v1_good()
        self.assertFalse(vc.check_v1_missing(drop(recs, lambda r: r.get("n") == 6))[0])
        late = [dict(r, t=5100) if r.get("n") == 6 else r for r in recs]
        v = vc.check_v1_missing(late)
        self.assertFalse(v[0])
        self.assertIn("после открытия", v[1])
        few = [dict(r, n=4) if r.get("n") == 6 else r for r in recs]
        self.assertFalse(vc.check_v1_missing(few)[0])

    def test_strip_after_eight_seconds_fails(self):
        recs = drop(v1_good(), lambda r: r["step"] == "strip") + [rec("strip", 2900, on=True)]
        recs.sort(key=lambda r: r["t"])
        v = vc.check_v1_strip(recs)
        self.assertFalse(v[0])
        self.assertIn("8.0 с", v[1])
        late_off = drop(v1_good(), lambda r: r["step"] == "strip") + [rec("strip", 2900, on=True), rec("strip", 9500, on=False)]
        late_off.sort(key=lambda r: r["t"])
        self.assertFalse(vc.check_v1_strip(late_off)[0])  # исчезла в 8,5 с от открытия
        self.assertFalse(vc.check_v1_clear(drop(v1_good(), lambda r: r.get("n") == 0))[0])
        slow = [dict(r, t=9200) if r.get("n") == 0 else r for r in v1_good()]
        self.assertFalse(vc.check_v1_clear(slow)[0])

    def test_strip_never_shown_still_passes_with_note(self):
        recs = drop(v1_good(), lambda r: r["step"] == "strip")
        v = vc.check_v1_strip(recs)
        self.assertTrue(v[0])
        self.assertIn("не показывалась", v[1])

    def test_no_back_two_backs_wrong_numbers(self):
        recs = v1_good()
        self.assertFalse(vc.check_v1_back(drop(recs, lambda r: r.get("name") == "drive:back"))[0])
        twice = recs + [ev("drive:back", 6000, drive="Q", photos=6)]
        v = vc.check_v1_back(twice)
        self.assertFalse(v[0])
        self.assertIn("2 раза", v[1])
        self.assertFalse(vc.check_v1_back([dict(r, photos=5) if r.get("photos") else r for r in recs])[0])
        self.assertFalse(vc.check_v1_back([dict(r, drive="E") if r.get("photos") else r for r in recs])[0])

    def test_toast_missing_twice_other_letter(self):
        recs = v1_good()
        v = vc.check_v1_toast(drop(recs, lambda r: r["step"] == "toast"))
        self.assertFalse(v[0])
        self.assertIn("не было", v[1])
        twice = recs + [rec("toast", 11000, text=TOAST), rec("toast", 12000, text="")]
        twice.sort(key=lambda r: r["t"])
        v = vc.check_v1_toast(twice)
        self.assertFalse(v[0])
        self.assertIn("2 раза", v[1])
        other = [dict(r, text="Диск E: снова доступен — найдено 6 фото") if r.get("text") == TOAST else r for r in recs]
        self.assertFalse(vc.check_v1_toast(other)[0])
        # тот же тост, замененный другим текстом и вернувшийся, - второе появление
        swap = recs + [rec("toast", 10000, text="Сохранено"), rec("toast", 10500, text=TOAST)]
        swap.sort(key=lambda r: r["t"])
        self.assertFalse(vc.check_v1_toast(swap)[0])

    def test_thumbs_within_ten_seconds(self):
        recs = v1_good()
        slow = [dict(r, t=16000) if r["step"] == "thumbs" else r for r in recs]
        v = vc.check_v1_thumbs(slow)
        self.assertFalse(v[0])
        self.assertIn("naturalWidth", v[1])
        partial = [dict(r, n=3) if r["step"] == "thumbs" else r for r in recs]
        self.assertFalse(vc.check_v1_thumbs(partial)[0])
        self.assertFalse(vc.check_v1_thumbs(drop(recs, lambda r: r["step"] == "thumbs"))[0])
        # без тоста точка отсчёта - drive:back
        notoast = drop(recs, lambda r: r["step"] == "toast")
        self.assertTrue(vc.check_v1_thumbs(notoast)[0])
        self.assertFalse(vc.check_v1_thumbs(drop(notoast, lambda r: r.get("name") == "drive:back"))[0])
        # миниатюры загрузились до тоста: записи после него нет, состояние на тост годное
        before = [dict(r, t=4000) if r["step"] == "thumbs" else r for r in recs]
        self.assertTrue(vc.check_v1_thumbs(before)[0])

    def test_canvas_without_images(self):
        recs = drop(v1_good(), lambda r: r["step"] == "canvas") + [rec("canvas", 7000, imgs=0, spreads=3)]
        v = vc.check_v1_canvas(recs)
        self.assertFalse(v[0])
        self.assertIn("3", v[1])


def v2_good(clear_dt=17000, back_dt=17000, toast_dt=17100):
    """Ход В2: отключение, неудачная проба в 9,0 с, полоса исчезает через clear_dt."""
    fail = 9000
    out = [
        rec("ready", 100, ok=True),
        rec("opening", 1000),
        rec("opened", 1100, ok=True),
        ev("missing", 1500, n=0),
        rec("cut-now", 2500),
        rec("check", 9000, ok=True, full=True),
        ev("missing", fail, n=6),
        rec("strip", fail + 50, on=True),
    ]
    if clear_dt is not None:
        out += [ev("missing", fail + clear_dt, n=0), rec("strip", fail + clear_dt + 50, on=False)]
    if back_dt is not None:
        out.append(ev("drive:back", fail + back_dt, drive="Q", photos=6))
    if toast_dt is not None:
        out += [rec("toast", fail + toast_dt, text=TOAST), rec("toast", fail + toast_dt + 4000, text="")]
    out.append(rec("done", fail + 30000, ok=True))
    return out


class V2Verdicts(unittest.TestCase):
    def test_in_time(self):
        v = vc.check_v2(v2_good())
        self.assertTrue(v[0], v[1])
        self.assertIn("полоса исчезла через 17.0 с", v[1])
        self.assertTrue(vc.check_v2_toast(v2_good())[0])

    def test_boundaries(self):
        self.assertTrue(vc.check_v2(v2_good(19500, 19500))[0])
        slow = vc.check_v2(v2_good(22000, 22000))
        self.assertTrue(slow[0])  # дольше 19,5 с, но не провал
        self.assertIn("дольше ожидаемых", slow[1])
        self.assertTrue(vc.check_v2(v2_good(25000, 25000))[0])
        over = vc.check_v2(v2_good(25100, 25100))
        self.assertFalse(over[0])
        self.assertIn("провал с 25.0 с", over[1])

    def test_never_cleared_or_no_back(self):
        v = vc.check_v2(v2_good(clear_dt=None, back_dt=None, toast_dt=None))
        self.assertFalse(v[0])
        self.assertIn("не исчезла", v[1])
        self.assertFalse(vc.check_v2(v2_good(back_dt=None))[0])
        twice = v2_good() + [ev("drive:back", 30000, drive="Q", photos=6)]
        self.assertFalse(vc.check_v2(twice)[0])
        wrong = [dict(r, photos=2) if r.get("name") == "drive:back" else r for r in v2_good()]
        self.assertFalse(vc.check_v2(wrong)[0])

    def test_toast_checks(self):
        self.assertFalse(vc.check_v2_toast(v2_good(toast_dt=None))[0])
        self.assertFalse(vc.check_v2_toast(v2_good(toast_dt=26000))[0])
        recs = v2_good() + [rec("toast", 40000, text=TOAST)]
        self.assertFalse(vc.check_v2_toast(recs)[0])

    def test_setup_problems_are_failures_not_silence(self):
        self.assertFalse(vc.check_v2([])[0])
        no_cut = drop(v2_good(), lambda r: r["step"] == "cut-now")
        self.assertIn("cut-now", vc.check_v2(no_cut)[1])
        dirty = [dict(r, n=2) if r.get("name") == "missing" and r.get("n") == 0 and r["t"] < 2500 else r for r in v2_good()]
        self.assertIn("до отключения", vc.check_v2(dirty)[1])
        nofail = drop(v2_good(), lambda r: r.get("n") == 6)
        self.assertIn("не появились", vc.check_v2(nofail)[1])

    def test_record_only_never_fails(self):
        for recs in (v2_good(), v2_good(None, None, None), [], drop(v2_good(), lambda r: r.get("n") == 6)):
            v = vc.check_record(recs, "через фокус")
            self.assertTrue(v[0], v[1])
        self.assertIn("через фокус", vc.check_record(v2_good(), "через фокус")[1])
        self.assertEqual(vc.check_record([], "NAS")[2], "skip")
        env = vc.check_record(v2_good(), "NAS", "net use: отказано")
        self.assertEqual(env[2], "skip")
        self.assertIn("не проверено: окружение", env[1])
        self.assertEqual(len(vc.check_record(v2_good(), "NAS")), 2)

    def test_facts_without_clear(self):
        f = vc.v2_facts(v2_good(None, None, None))
        self.assertEqual(f["fail_n"], 6)
        self.assertIn("полоса исчезла через нет", vc.facts_text(f))

    def test_log_panics(self):
        self.assertTrue(vc.check_log_panics("")[0])
        self.assertTrue(vc.check_log_panics("2026 диск Q: снова отвечает")[0])
        v = vc.check_log_panics("a\npanic: runtime error\n")
        self.assertFalse(v[0])
        self.assertFalse(vc.check_log_panics("goroutine 7 [running]:")[0])


class Prepare(unittest.TestCase):
    ALBUM = {
        "version": 19, "folders": [{"path": "photos"}],
        "photos": {"a": {"path": "photos/photo-01.jpg", "w": 1, "h": 1, "size": 5}, "b": {"path": "photos\\photo-02.jpg", "size": 7}},
        "spreads": [{"id": "s", "photos": ["a", "b"]}],
    }

    def test_retarget_moves_photos_to_drive_root(self):
        out = vx.retarget_album(self.ALBUM, "Q", ["Q:\\"])
        self.assertEqual(out["folders"], [{"path": "Q:\\"}])
        self.assertEqual(out["photos"]["a"]["path"], "Q:\\photo-01.jpg")
        self.assertEqual(out["photos"]["b"]["path"], "Q:\\photo-02.jpg")
        self.assertEqual(out["photos"]["a"]["size"], 5)  # размер сохраняется: по нему загрузчик сверяет байты
        self.assertEqual(out["spreads"], self.ALBUM["spreads"])
        self.assertEqual(self.ALBUM["folders"], [{"path": "photos"}])  # исходный не тронут

    def test_retarget_edges(self):
        self.assertEqual(vx.retarget_album({}, "R", [])["folders"], [])
        out = vx.retarget_album({"photos": {"a": {}}}, "R", ["C:\\src"])
        self.assertEqual(out["photos"]["a"]["path"], "R:\\")
        self.assertEqual(out["folders"], [{"path": "C:\\src"}])
        self.assertEqual(vx.retarget_album({"photos": None}, "R", ["x"])["photos"], None)

    def test_write_album_roundtrip_and_config(self):
        with tempfile.TemporaryDirectory() as tmp:
            src = os.path.join(tmp, "Alpha.salbum")
            with open(src, "w", encoding="utf-8") as f:
                json.dump(self.ALBUM, f)
            dst = vx.write_album(src, os.path.join(tmp, "sub", "Album.salbum"), "Q", ["Q:\\"])
            with open(dst, encoding="utf-8") as f:
                self.assertEqual(json.load(f)["photos"]["a"]["path"], "Q:\\photo-01.jpg")
            cfg = json.loads(vx.volume_config(dst))
            self.assertEqual(cfg["recent"], [dst])
            self.assertFalse(cfg["libraryNet"]["allowed"])
            self.assertTrue(cfg["hideStart"])

    def test_ps_quote_and_restore_script(self):
        self.assertEqual(vx.ps_quote("a'b"), "'a''b'")
        s = vx.restore_script(["Netlogon", "bad name;rm", "Browser"])
        self.assertIn("Start-Service Netlogon", s)
        self.assertIn("Start-Service Browser", s)
        self.assertNotIn("rm", s)  # имя службы из вывода PowerShell проверяется
        self.assertTrue(s.startswith("Start-Service LanmanServer"))

    def test_make_set_without_generator(self):
        with tempfile.TemporaryDirectory() as tmp:
            made, why = vx.make_set(tmp, os.path.join(tmp, "x"))
            self.assertIsNone(made)
            self.assertIn("qa-albums", why)


class Hook(unittest.TestCase):
    def test_cut_then_restore_once_in_order(self):
        calls = []
        h = vx.CutHook(lambda: (calls.append("cut") or (0, "")), lambda: (calls.append("restore") or (0, "")))
        h([{"step": "ev", "name": "missing", "n": 6}])  # до cut-now: ничего
        self.assertEqual(calls, [])
        h([{"step": "cut-now"}])
        h([{"step": "cut-now"}])  # повтор не режет второй раз
        h([{"step": "ev", "name": "missing", "n": 0}])  # пустой список не возврат
        self.assertEqual(calls, ["cut"])
        h([{"step": "ev", "name": "missing", "n": 6}, {"step": "ev", "name": "missing", "n": 6}])
        self.assertEqual(calls, ["cut", "restore"])
        self.assertEqual(h.failed(), [])
        self.assertTrue(vx.action_verdict(h)[0])

    def test_failed_and_raising_actions_are_reported(self):
        def boom():
            raise OSError("нет subst")
        h = vx.CutHook(boom, lambda: (1, "занято"))
        h([{"step": "cut-now"}, {"step": "ev", "name": "missing", "n": 1}])
        self.assertEqual([a["name"] for a in h.failed()], ["cut", "restore"])
        v = vx.action_verdict(h)
        self.assertFalse(v[0])
        self.assertIn("нет subst", v[1])
        self.assertFalse(vx.action_verdict(vx.CutHook(None, None))[0])  # действий не было


class Reports(unittest.TestCase):
    def test_markdown_skip_and_fail(self):
        rep = vx.new_report()
        rep.judge("п|1", (True, "ok|х"))
        rep.judge("п2", (False, "плохо"))
        rep.judge("п3", (True, "не проверено: окружение", "skip"))
        rep.notes.append("### заметка")
        md = rep.markdown()
        self.assertIn("| п|1 | ok | ok/х |", md)
        self.assertIn("| п3 | skip |", md)
        self.assertEqual(len(rep.failed()), 1)  # skip не валит задачу
        self.assertTrue(md.rstrip().endswith("### заметка"))

    def test_run_basics_needs_done(self):
        run = {"records": [rec("ready", 10, ok=True, ms=10)], "done": False, "alive": True, "code": None, "seconds": 3, "log": ""}
        self.assertFalse(vx.vc_ready(run)[0])
        run["done"] = True
        self.assertTrue(vx.vc_ready(run)[0])
        self.assertFalse(vx.vc_ready(dict(run, records=[]))[0])


FAKE_EXE = textwrap.dedent(
    """\
    #!{py}
    import json, os, sys, time
    out = os.environ["SA_QA_PROBE_OUT"]
    assert os.path.isfile(os.environ["SA_QA_PROBE"]) and "SA_QA_VOLDELAY" not in os.environ
    def w(r):
        with open(out, "a", encoding="utf-8") as f:
            f.write(json.dumps(r) + "\\n")
    w({{"step": "ready", "ok": True}}); time.sleep(0.3)
    w({{"step": "cut-now"}}); time.sleep(0.6)
    w({{"step": "ev", "name": "missing", "n": 6}}); time.sleep(0.6)
    w({{"step": "done", "ok": True}})
    time.sleep(60)
    """
)


class NullSamplerTest(unittest.TestCase):
    def test_null_sampler(self):
        s = vx.NullSampler(1)
        s.start()
        self.assertIsNone(s.sample_now())
        s.stop()
        self.assertEqual(s.peak, {"own": 0, "tree": 0})


@unittest.skipIf(os.name == "nt", "поддельный exe - скрипт с shebang")
class RunWithHook(unittest.TestCase):
    def test_hook_acts_between_records(self):
        with tempfile.TemporaryDirectory() as tmp:
            exe = os.path.join(tmp, "exe")
            with open(exe, "w", encoding="utf-8") as f:
                f.write(FAKE_EXE.format(py=sys.executable))
            os.chmod(exe, os.stat(exe).st_mode | stat.S_IXUSR)
            calls = []
            h = vx.CutHook(lambda: (calls.append("cut") or (0, "")), lambda: (calls.append("restore") or (0, "")))
            env = {k: v for k, v in os.environ.items() if k != "SA_QA_VOLDELAY"}
            r = vx.rn.run_probe(exe, env, os.path.join(tmp, "w"), "t", dict(vx.CUT_CFG, full=False, album="x"), 20,
                                  sampler_cls=vx.NullSampler, poll=0.05, hook=h)
            self.assertTrue(r["done"])
            self.assertEqual(calls, ["cut", "restore"])
            self.assertEqual([x["step"] for x in r["records"]], ["ready", "cut-now", "ev", "done"])
            with open(os.path.join(tmp, "w", "t.js"), encoding="utf-8") as f:
                js = f.read()
            self.assertIn("async function main(cfg)", js)  # в скрипт вставлен именно зонд 157
            self.assertIn("cut-now", js)
            self.assertTrue(js.rstrip().endswith(");"))

    def test_run_probe_without_hook_still_works(self):
        # hook необязателен: прежние вызовы library_exe.py не меняются
        self.assertIsNone(rn.run_probe.__defaults__[-1])


# Окно подставное: Go-методы и события ведут себя как у программы; DOM - из переменных; печатает записи зонда строками.
HARNESS = textwrap.dedent(
    """\
    const fs = require('fs'), vm = require('vm')
    const mode = process.argv[3], bad = process.env.HARNESS_BAD || ''
    const listeners = {}
    const emit = (n, d) => (listeners[n] || []).forEach((f) => f(d))
    const dom = { strip: false, toast: '', thumbs: 0, imgs: 0 }
    let cut = false, checks = 0, restored = false
    const App = {
      LibraryPacks: async () => ({}),
      OpenRecent: async () => {
        if (mode === 'sleepy') {
          setTimeout(() => { emit('missing', ['a', 'b', 'c', 'd', 'e', 'f']); dom.strip = true }, 15)
          setTimeout(() => {
            emit('missing', []); dom.strip = false
            if (bad !== 'noback') emit('drive:back', { drive: 'Q', photos: 6 })
            if (bad === 'twice') emit('drive:back', { drive: 'Q', photos: 6 })
            if (bad !== 'notoast') dom.toast = 'Диск Q: снова доступен — найдено 6 фото'
            dom.thumbs = bad === 'nothumbs' ? 0 : 6; dom.imgs = 3
          }, 40)
        } else {
          setTimeout(() => emit('missing', []), 5)
        }
        return {}
      },
      CheckFolders: async (full) => {
        checks++
        if (!cut) return
        if (checks === 1) { setTimeout(() => { emit('missing', ['a', 'b', 'c', 'd', 'e', 'f']); dom.strip = true }, 5); return }
        if (checks === 4 && bad !== 'stuck') setTimeout(() => {
          emit('missing', []); dom.strip = false
          emit('drive:back', { drive: 'Q', photos: 6 }); dom.toast = 'Диск Q: снова доступен — найдено 6 фото'
        }, 5)
      },
    }
    const attr = (sel, list) => (sel === '[data-guide="missing-bar"]' ? (dom.strip ? [{}] : []) : sel === '.toast' ? (dom.toast ? [{ textContent: dom.toast }] : []) : list(sel))
    const imgs = (n) => Array.from({ length: n }, () => ({ complete: true, naturalWidth: 100 }))
    const document = {
      querySelector: (sel) => attr(sel, () => [])[0] || null,
      querySelectorAll: (sel) => attr(sel, (s) => (s === '[data-thumb] img' ? imgs(dom.thumbs) : s === '[data-thumb]' ? imgs(6) : s === '[data-spread] img' ? imgs(dom.imgs) : s === '[data-spread]' ? imgs(3) : [])),
    }
    const sandbox = {
      document, window: { go: { app: { App } }, runtime: {
        EventsEmit: (n, d) => {
          if (n !== 'qaprobe') return
          console.log(d)
          if (JSON.parse(d).step === 'cut-now') cut = true // Python отключает том
        },
        EventsOn: (n, f) => { (listeners[n] = listeners[n] || []).push(f); return () => {} } } },
      setTimeout, clearTimeout, setInterval, clearInterval, Math, Date, JSON, Promise, Object, String, Error, Array, console,
    }
    vm.createContext(sandbox)
    vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), sandbox)
    """
)


@unittest.skipUnless(shutil.which("node"), "нужен node")
class ProbeScript(unittest.TestCase):
    """volume_probe.js под node с подставным окном; записи проходят через те же вердикты, что и в прогоне."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def run_js(self, mode, bad=""):
        cfg = {"mode": mode, "album": "x.salbum", "pace": 0.01, "capMs": 14000, "tailMs": 11500,
               "cutAfterMs": 6500, "periodMs": 2500, "limitMs": 28000, "noFailMs": 14000, "full": False}
        harness = os.path.join(self.tmp.name, "harness.js")
        with open(harness, "w", encoding="utf-8") as f:
            f.write(HARNESS)
        script = os.path.join(self.tmp.name, "probe.js")
        with open(script, "w", encoding="utf-8") as f:
            f.write(rn_probe(cfg))
        env = dict(os.environ, HARNESS_BAD=bad)
        r = subprocess.run(["node", harness, script, mode], capture_output=True, timeout=60, env=env)
        self.assertEqual(r.returncode, 0, r.stderr.decode("utf-8", "replace"))
        return [json.loads(ln) for ln in r.stdout.decode("utf-8").splitlines() if ln.startswith("{")]

    def test_syntax(self):
        r = subprocess.run(["node", "--check", os.path.join(QA, "volume_probe.js")], capture_output=True)
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_sleepy_records_pass_v1(self):
        recs = self.run_js("sleepy")
        self.assertEqual(recs[-1]["step"], "done")
        for name in ("missing", "clear", "back", "toast", "strip", "thumbs", "canvas"):
            v = getattr(vc, "check_v1_" + name)(recs)
            self.assertTrue(v[0], (name, v[1]))

    def test_sleepy_breakage_is_seen(self):
        self.assertFalse(vc.check_v1_toast(self.run_js("sleepy", "notoast"))[0])
        self.assertFalse(vc.check_v1_back(self.run_js("sleepy", "noback"))[0])
        self.assertFalse(vc.check_v1_back(self.run_js("sleepy", "twice"))[0])
        self.assertFalse(vc.check_v1_thumbs(self.run_js("sleepy", "nothumbs"))[0])

    def test_cut_mode_asks_python_then_checks_folders(self):
        recs = self.run_js("cut")
        order = [r["step"] for r in recs if r["step"] in ("opening", "opened", "cut-now", "check", "done")]
        self.assertEqual(order[:3], ["opening", "opened", "cut-now"])
        self.assertEqual(order[-1], "done")
        self.assertGreaterEqual(len([r for r in recs if r["step"] == "check"]), 4)
        self.assertTrue(recs[[r["step"] for r in recs].index("check")]["full"])  # первая проверка - полная
        self.assertFalse([r for r in recs if r["step"] == "check"][1]["full"])  # дальше - по cfg.full (False)
        f = vc.v2_facts(recs)
        self.assertEqual(f["fail_n"], 6)
        self.assertIn("dt_back", f)
        self.assertTrue(vc.check_v2(recs)[0], vc.check_v2(recs)[1])
        self.assertTrue(vc.check_v2_toast(recs)[0])

    def test_cut_mode_stuck_ends_by_limit_and_fails(self):
        recs = self.run_js("cut", "stuck")
        self.assertEqual(recs[-1]["step"], "done")
        v = vc.check_v2(recs)
        self.assertFalse(v[0])
        self.assertIn("не исчезла", v[1])


def rn_probe(cfg):
    """Текст для SA_QA_PROBE из volume_probe.js (как probe_script, но не трогая общий путь library_run)."""
    with open(os.path.join(QA, "volume_probe.js"), encoding="utf-8") as f:
        return f.read() + "\nmain(" + json.dumps(cfg, ensure_ascii=False) + ");\n"


if __name__ == "__main__":
    unittest.main()
