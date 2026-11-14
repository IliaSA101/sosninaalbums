"""Логика library_exe.py (win-qa, 144) без Windows: разбор записей зонда, память, вердикты, запуск на поддельном exe,
сам library_probe.js под node с подставным окном. Сам прогон на qa-exe - только задание library win-qa.yml."""

import base64
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


ck = load("library_checks")
rn = load("library_run")
lx = load("library_exe")


def jpeg(**kw):
    p = {"status": 200, "ct": "image/jpeg", "nosniff": "nosniff", "cc": "no-cache", "len": 500, "fnv": "ab"}
    p.update(kw)
    return p


def nf(path="x", resolved=None, **kw):
    p = {"p": path, "resolved": resolved or "/media/lp/" + path, "status": 404, "ct": "text/plain; charset=utf-8", "body": "404 page not found\n"}
    p.update(kw)
    return p


class Records(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.f = os.path.join(self.tmp.name, "out.jsonl")

    def test_missing_file_and_partial_line(self):
        self.assertEqual(rn.read_new(self.f, 0), ([], 0))
        with open(self.f, "wb") as f:
            f.write(b'{"step":"a"}\n{"step":"b"')
        recs, off = rn.read_new(self.f, 0)
        self.assertEqual([r["step"] for r in recs], ["a"])
        with open(self.f, "ab") as f:
            f.write(b'}\n')
        recs, off2 = rn.read_new(self.f, off)
        self.assertEqual([r["step"] for r in recs], ["b"])
        self.assertEqual(rn.read_new(self.f, off2), ([], off2))

    def test_garbage_empty_and_non_dict(self):
        with open(self.f, "wb") as f:
            f.write(b'\n\nnot json\n[1,2]\n\xff\xfe\n{"step":"ok"}\n')
        recs, _ = rn.read_new(self.f, 0)
        self.assertEqual([r["step"] for r in recs], ["unparsable", "unparsable", "unparsable", "ok"])

    def test_step_helpers(self):
        recs = [{"step": "a", "n": 1}, {"step": "b"}, {"step": "a", "n": 2}]
        self.assertEqual(ck.step_of(recs, "a")["n"], 2)
        self.assertIsNone(ck.step_of(recs, "zz"))
        self.assertEqual(len(ck.steps_of(recs, "a")), 2)
        self.assertIsNone(ck.step_of([], "a"))


class Memory(unittest.TestCase):
    ROWS = "10|1|SosninaAlbums.exe|1000|900\n11|10|msedgewebview2.exe|500|400\n12|11|msedgewebview2.exe|250|200\n13|1|other.exe|9999|9999\nмусор\n14|x|y|z|w\n"

    def test_tree(self):
        rows = rn.parse_proc_rows(self.ROWS)
        self.assertEqual(len(rows), 4)
        m = rn.mem_from_rows(rows, 10)
        self.assertEqual((m["own"], m["tree"], m["procs"]), (1000, 1750, 3))
        self.assertIsNone(rn.mem_from_rows(rows, 99))
        self.assertEqual(rn.parse_proc_rows(""), [])
        self.assertEqual(rn.parse_proc_rows("1|0|a||")[0][3:], (0, 0))

    def test_sampler_peak_and_failures(self):
        vals = iter([{"own": 5, "tree": 7}, None, {"own": 3, "tree": 20}])

        def sample(pid):
            v = next(vals, None)
            if v is None:
                raise RuntimeError("ps упал")
            return v

        s = rn.MemSampler(1, sample=sample, period=100)
        s.sample_now()
        s.sample_now()  # None
        s.sample_now()
        s.sample_now()  # исключение: проглатывается
        self.assertEqual((s.peak, s.n), ({"own": 5, "tree": 20}, 2))


class Verdicts(unittest.TestCase):
    def test_ready(self):
        run = {"records": [{"step": "ready", "ok": True, "ms": 5}], "alive": True, "code": None}
        self.assertTrue(ck.check_ready(run)[0])
        self.assertFalse(ck.check_ready({"records": [], "alive": False, "code": 1})[0])
        self.assertFalse(ck.check_ready({"records": [{"step": "ready", "ok": False, "err": "x"}], "alive": True, "code": None})[0])

    def test_update_and_tiles(self):
        self.assertTrue(ck.check_update([{"step": "update", "ok": True, "try": 1, "available": 3, "failed": 0}])[0])
        self.assertFalse(ck.check_update([{"step": "update", "ok": False, "err": "нет связи"}])[0])
        self.assertFalse(ck.check_update([])[0])
        good = [{"step": "tiles0", "ok": True, "counts": {"bg": 0}}, {"step": "tiles1", "ok": True, "counts": {"bg": 5}, "packs": ["a"]}]
        self.assertTrue(ck.check_tiles(good)[0])
        self.assertFalse(ck.check_tiles([good[0], {"step": "tiles1", "ok": True, "counts": {"bg": 0}, "packs": []}])[0])  # пусто
        early = [{"step": "tiles0", "ok": True, "counts": {"bg": 2}}, good[1]]
        self.assertFalse(ck.check_tiles(early)[0])  # плитки до задания образцов

    def test_overlap(self):
        ok = {"step": "overlap", "ok": True, "pack": "p", "bytes": 4096, "picked": True, "derr": "", "downloadMs": 10, "previewsMs": 20, "previewsReady": 5, "overlapped": True}
        self.assertTrue(ck.check_overlap([ok])[0])
        self.assertFalse(ck.check_overlap([dict(ok, picked=False, why="Библиотека обновляется")])[0])
        self.assertFalse(ck.check_overlap([dict(ok, derr="boom")])[0])
        self.assertFalse(ck.check_overlap([dict(ok, previewsTimeout=True)])[0])
        self.assertTrue(ck.check_overlap([dict(ok, overlapped=False)])[0])  # не перекрылись - не провал, но в подробностях
        self.assertIn("нет", ck.check_overlap([dict(ok, overlapped=False)])[1])

    def good_rec(self):
        d = jpeg()
        return {"step": "good", "ok": True, "cell": "a/bg/0", "direct": d, "encoded": jpeg(),
                "head": {"status": 200, "len": 0}, "post": {"status": 405, "allow": "GET, HEAD"}, "range": jpeg()}

    def test_good(self):
        self.assertTrue(ck.check_good([self.good_rec()])[0])
        for key, bad in (("direct", jpeg(ct="text/html")), ("direct", jpeg(len=0)), ("encoded", jpeg(status=404)), ("encoded", jpeg(fnv="zz")),
                         ("head", {"status": 200, "len": 5}), ("post", {"status": 200, "allow": ""}), ("range", jpeg(len=10)),
                         ("direct", jpeg(nosniff="")), ("direct", jpeg(cc=""))):
            r = self.good_rec()
            r[key] = bad
            self.assertFalse(ck.check_good([r])[0], (key, bad))
        self.assertFalse(ck.check_good([])[0])

    def test_paths(self):
        base = nf("nope-zz/bg/0")
        slash = nf("nope-zz/", status=400, body="неверный адрес\n")
        items = [nf("a/bg/99"), nf("a/bg/0/", status=400, body="неверный адрес\n"), nf("x", resolved="/media/bg/0", status=200, body="<html>")]
        ok = {"step": "paths", "ok": True, "baseline": base, "baselineSlash": slash, "items": items}
        v = ck.check_paths([ok])
        self.assertTrue(v[0], v[1])
        self.assertIn("схлопнул браузер до отправки: 1", v[1])
        # адрес с «/» в конце: HTML-страница Wails вместо ответа службы (то, что видел прогон winqa/144-exe) - провал с причиной;
        # признак - Content-Type text/html, а не отпечаток index.html приложения (Wails отдаёт свою defaultindex.html)
        for ct in ("text/html; charset=utf-8", "text/html"):
            page = dict(nf("", resolved="/media/lp/"), status=200, ct=ct, body="<!DOCTYPE html>", len=201368)
            v = ck.check_paths([dict(ok, items=[page])])
            self.assertFalse(v[0])
            self.assertIn("подмена ассет-сервера Wails", v[1])
        page = dict(nf("a/index.html", resolved="/media/lp/a/index.html"), status=200, ct="text/html")
        self.assertFalse(ck.check_paths([dict(ok, items=[page])])[0])
        # «/index.html» в конце тоже ждёт 400 (Wails подменяет и его)
        self.assertTrue(ck.check_paths([dict(ok, items=[nf("a/index.html", status=400, body="неверный адрес\n", resolved="/media/lp/a/index.html", ct=slash["ct"])])])[0])
        self.assertFalse(ck.check_paths([dict(ok, items=[nf("a/bg/0/")])])[0])         # 404 на «/» в конце: Wails подменит его index.html
        self.assertFalse(ck.check_paths([dict(ok, baselineSlash=nf("q", status=200))])[0])
        self.assertFalse(ck.check_paths([dict(ok, items=[nf("a/bg/0", status=400, body="x")])])[0])  # 400 без «/» - не наш ответ
        self.assertFalse(ck.check_paths([dict(ok, items=[nf("a/bg/01", status=200)])])[0])       # годный ответ на обход
        self.assertFalse(ck.check_paths([dict(ok, items=[nf("a/bg/01", body="другой")])])[0])    # ответил другой обработчик
        self.assertFalse(ck.check_paths([dict(ok, items=[nf("..", resolved="/media/")])])[0])    # проверять нечего
        self.assertFalse(ck.check_paths([dict(ok, items=[{"p": "z", "resolved": "/media/lp/z", "err": "net"}])])[0])
        self.assertFalse(ck.check_paths([dict(ok, baseline=nf("q", status=200))])[0])
        self.assertFalse(ck.check_paths([])[0])
        self.assertEqual(len(ck.PATHS), len(set(ck.PATHS)))

    def stress(self, **kw):
        r = {"step": "round", "ok": True, "n": 1, "requests": 100, "got": 60, "aborted": 40, "bad": {}, "errors": 0, "done": 40, "ms": 5}
        r.update(kw)
        return r

    def test_stress(self):
        fin = {"step": "final", "ok": True, "probe": jpeg()}
        rounds = [self.stress(), self.stress(n=2)]
        self.assertTrue(ck.check_stress(rounds + [fin])[0])
        self.assertFalse(ck.check_stress([rounds[0], fin])[0])                                        # один круг
        self.assertFalse(ck.check_stress([self.stress(errors=2), self.stress(), fin])[0])             # ошибки запросов
        self.assertFalse(ck.check_stress([self.stress(bad={"500": 3}), self.stress(), fin])[0])      # не 200
        self.assertFalse(ck.check_stress([self.stress(got=0), self.stress(), fin])[0])                # ни одного ответа
        self.assertFalse(ck.check_stress([self.stress(done=2), self.stress(), fin])[0])               # не успел
        self.assertFalse(ck.check_stress(rounds + [{"step": "final", "ok": True, "probe": jpeg(status=404)}])[0])
        self.assertFalse(ck.check_stress([self.stress(ok=False, err="x")] * 2 + [fin])[0])
        self.assertFalse(ck.check_stress([])[0])

    def test_alive(self):
        self.assertTrue(ck.check_alive({"done": True, "alive": True, "code": None, "seconds": 1})[0])
        self.assertFalse(ck.check_alive({"done": True, "alive": False, "code": 3, "seconds": 1})[0])
        self.assertFalse(ck.check_alive({"done": False, "alive": True, "code": None, "seconds": 1})[0])

    def mem(self, own, tree=None):
        return {"own": own * ck.MIB, "own_ws": own * ck.MIB, "tree": (tree or own * 3) * ck.MIB, "tree_ws": 0, "procs": 4}

    def marks(self, b, r1, r2):
        return [{"step": "mark", "name": n, "mem": m} for n, m in (("baseline", b), ("r1", r1), ("r2", r2))]

    def test_memory(self):
        run = {"peak": {"own": 200 * ck.MIB, "tree": 700 * ck.MIB}}
        self.assertTrue(ck.check_memory(self.marks(self.mem(100), self.mem(150), self.mem(160)), run)[0])
        self.assertFalse(ck.check_memory(self.marks(self.mem(100), self.mem(150), self.mem(260)), run)[0])            # плато exe
        self.assertFalse(ck.check_memory(self.marks(self.mem(100, 300), self.mem(150, 400), self.mem(160, 700)), run)[0])  # плато дерева
        self.assertFalse(ck.check_memory(self.marks(self.mem(100), self.mem(300), self.mem(401)), run)[0])            # рост от начала
        self.assertFalse(ck.check_memory(self.marks(self.mem(1), None, self.mem(1)), run)[0])                         # не снято
        self.assertFalse(ck.check_memory([], run)[0])
        self.assertTrue(ck.check_memory(self.marks(self.mem(100), self.mem(90), self.mem(80)), run)[0])               # убыль - не рост
        note = ck.memory_note(self.marks(self.mem(100), self.mem(150), self.mem(160)), run)
        self.assertIn("после круга 2", note)
        self.assertIn("-", ck.memory_note([], run))

    def test_revoke(self):
        self.assertTrue(ck.check_revoke([{"step": "revoke", "ok": True, "counts": {"bg": 0, "sticker": 0}, "probe": {"status": 404}}])[0])
        self.assertFalse(ck.check_revoke([{"step": "revoke", "ok": True, "counts": {"bg": 1}, "probe": {"status": 404}}])[0])
        self.assertFalse(ck.check_revoke([{"step": "revoke", "ok": True, "counts": {"bg": 0}, "probe": {"status": 200}}])[0])
        self.assertFalse(ck.check_revoke([])[0])

    def test_corrupt_and_refetch(self):
        b, c, a = ({"step": "c-before", "ok": True, "n": 2}, {"step": "c-cell", "ok": True, "probe": {"status": 404}}, {"step": "c-after", "ok": True, "n": 0})
        self.assertTrue(ck.check_corrupt([b, c, a], False)[0])
        self.assertFalse(ck.check_corrupt([b, c, a], True)[0])                                        # файл остался
        self.assertFalse(ck.check_corrupt([b, {"step": "c-cell", "ok": True, "probe": {"status": 200}}, a], False)[0])  # подмена не замечена
        self.assertFalse(ck.check_corrupt([b, c, dict(a, n=1)], False)[0])                            # плитки остались
        self.assertFalse(ck.check_corrupt([dict(b, n=0), c, a], False)[0])                            # плиток не было до показа
        self.assertFalse(ck.check_corrupt([], False)[0])
        f, t, cell = ({"step": "r-fetch", "ok": True, "timeout": False, "ready": 1}, {"step": "r-tiles", "ok": True, "n": 3}, {"step": "r-cell", "ok": True, "probe": jpeg()})
        self.assertTrue(ck.check_refetch([f, t, cell], True)[0])
        self.assertFalse(ck.check_refetch([f, t, cell], False)[0])
        self.assertFalse(ck.check_refetch([dict(f, timeout=True), t, cell], True)[0])
        self.assertFalse(ck.check_refetch([f, dict(t, n=0), cell], True)[0])
        self.assertFalse(ck.check_refetch([f, t, {"step": "r-cell", "ok": True, "probe": {"status": 404}}], True)[0])
        self.assertFalse(ck.check_refetch([], True)[0])

    def test_log(self):
        self.assertTrue(ck.check_log("")[0])
        self.assertTrue(ck.check_log("библиотека: образцы — раздача клетки: нет такой (1)\nдругая строка C:\\Users\\x\\a\n")[0])
        self.assertFalse(ck.check_log("panic: runtime error")[0])
        self.assertFalse(ck.check_log("библиотека: образцы — раздача клетки: сбой (string)")[0])
        self.assertFalse(ck.check_log("библиотека: ошибка https://github.com/x")[0])
        self.assertFalse(ck.check_log("библиотека: C:\\Users\\a\\b")[0])


class Disk(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.d = self.tmp.name

    def test_catalog_pack(self):
        body = {"packs": [{"id": "a", "preview": {"sha256": "ff"}}, {"id": "b"}]}
        env = {"sig": "x", "body": base64.b64encode(json.dumps(body).encode()).decode()}
        with open(os.path.join(self.d, "library-catalog.json"), "w") as f:
            json.dump(env, f)
        self.assertEqual(ck.catalog_pack(self.d, "a")["preview"]["sha256"], "ff")
        self.assertIsNone(ck.catalog_pack(self.d, "zz"))
        self.assertIsNone(ck.catalog_pack(os.path.join(self.d, "нет"), "a"))
        for bad in ("{", '{"body": 5}', '{"sig":"x"}', '{"body":"!!!"}', "[]"):
            with open(os.path.join(self.d, "library-catalog.json"), "w") as f:
                f.write(bad)
            self.assertIsNone(ck.catalog_pack(self.d, "a"), bad)

    def test_corrupt_sheet_keeps_size(self):
        p = os.path.join(self.d, "s.jpg")
        data = bytes(range(256)) * 4
        with open(p, "wb") as f:
            f.write(data)
        ck.corrupt_sheet(p)
        with open(p, "rb") as f:
            got = f.read()
        self.assertEqual(len(got), len(data))
        self.assertEqual([i for i in range(len(data)) if got[i] != data[i]], [100])
        for n in (0, 1, 100):
            with open(p, "wb") as f:
                f.write(b"x" * n)
            with self.assertRaises(ValueError):
                ck.corrupt_sheet(p)

    def test_sheets_dir_and_matches(self):
        pv = os.path.join(self.d, "library-previews")
        self.assertFalse(ck.check_sheets_dir(pv)[0])  # папки нет
        os.makedirs(pv)
        self.assertFalse(ck.check_sheets_dir(pv)[0])  # пусто
        data = b"jpeg-bytes"
        import hashlib
        sha = hashlib.sha256(data).hexdigest()
        with open(os.path.join(pv, sha + ".jpg"), "wb") as f:
            f.write(data)
        self.assertTrue(ck.check_sheets_dir(pv)[0])
        self.assertTrue(ck.sheet_matches(ck.sheet_path(self.d, sha), sha))
        with open(os.path.join(pv, "0" * 64 + ".jpg"), "wb") as f:
            f.write(b"other")
        self.assertFalse(ck.check_sheets_dir(pv)[0])  # имя не равно sha содержимого
        self.assertFalse(ck.sheet_matches(os.path.join(pv, "нет.jpg"), sha))

    def test_sha_validation(self):
        good = "a" * 64
        self.assertTrue(ck.valid_sha(good))
        for bad in ("", "A" * 64, "a" * 63, "a" * 65, "../" + "a" * 61, "a" * 63 + "/", None, 5, "a" * 64 + "\n"):
            self.assertFalse(ck.valid_sha(bad), repr(bad))
            with self.assertRaises(ValueError):
                ck.sheet_path(self.d, bad)
        self.assertTrue(ck.sheet_path(self.d, good).endswith(good + ".jpg"))

    def test_tmp_leftovers(self):
        os.makedirs(os.path.join(self.d, "a", "b"))
        for rel in (".dl-x.tmp", "a/.dl-y.tmp", "a/b/.dl-z.tmp", "a/keep.tmp", "a/dl-no.tmp"):
            open(os.path.join(self.d, rel), "w").close()
        found = sorted(os.path.relpath(p, self.d).replace("\\", "/") for p in ck.tmp_leftovers([self.d, os.path.join(self.d, "нет")]))
        self.assertEqual(found, [".dl-x.tmp", "a/.dl-y.tmp", "a/b/.dl-z.tmp"])

    def test_config(self):
        cfg = json.loads(lx.stand_config())
        self.assertIs(cfg["libraryNet"]["allowed"], True)
        self.assertIs(json.loads(lx.stand_config(False))["libraryNet"]["allowed"], False)
        self.assertIs(cfg["update"]["off"], True)
        p = os.path.join(self.d, "config.json")
        with open(p, "w") as f:
            json.dump({"libraryNet": {"allowed": False, "catalogSeq": 15}, "x": 1}, f)
        lx.set_consent(p)
        with open(p, encoding="utf-8") as f:
            got = json.load(f)
        self.assertEqual(got, {"libraryNet": {"allowed": True, "catalogSeq": 15}, "x": 1})
        with open(p, "w") as f:
            f.write("{}")
        lx.set_consent(p)
        with open(p, encoding="utf-8") as f:
            self.assertIs(json.load(f)["libraryNet"]["allowed"], True)

    def test_report_markdown(self):
        rep = lx.Report()
        rep.judge("п|1", (True, "ok|х"))
        rep.judge("п2", (False, "плохо"))
        rep.notes.append("### заметка")
        md = rep.markdown()
        self.assertIn("| п|1 | ok | ok/х |", md)
        self.assertIn("FAIL", md)
        self.assertEqual(len(rep.failed()), 1)
        self.assertTrue(md.rstrip().endswith("### заметка"))


FAKE_EXE = textwrap.dedent(
    """\
    #!{py}
    import json, os, sys, time
    out = os.environ["SA_QA_PROBE_OUT"]
    assert os.path.isfile(os.environ["SA_QA_PROBE"]) and "SA_LIBRARY_URL" not in os.environ
    mode = {mode!r}
    def w(r):
        with open(out, "a", encoding="utf-8") as f:
            f.write(json.dumps(r) + "\\n")
    if mode == "ok":
        w({{"step": "ready", "ok": True}}); w({{"step": "mark", "name": "baseline"}}); w({{"step": "done", "ok": True}})
        time.sleep(60)
    elif mode == "exit":
        w({{"step": "ready", "ok": True}}); sys.exit(7)
    else:
        time.sleep(60)
    """
)


class FakeSampler:
    def __init__(self, pid):
        self.peak, self.n = {"own": 1, "tree": 2}, 3

    def start(self):
        pass

    def stop(self):
        pass

    def sample_now(self):
        return {"own": 1}


@unittest.skipIf(os.name == "nt", "поддельный exe - скрипт с shebang")
class RunProbe(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def fake(self, mode):
        p = os.path.join(self.tmp.name, "exe-" + mode)
        with open(p, "w", encoding="utf-8") as f:
            f.write(FAKE_EXE.format(py=sys.executable, mode=mode))
        os.chmod(p, os.stat(p).st_mode | stat.S_IXUSR)
        return p

    def run_it(self, mode, timeout=20):
        env = dict(os.environ, SA_LIBRARY_URL="")
        env.pop("SA_LIBRARY_URL")
        return rn.run_probe(self.fake(mode), env, os.path.join(self.tmp.name, "w"), mode, {"mode": "main", "passes": 3}, timeout, sampler_cls=FakeSampler, poll=0.05)

    def test_done_alive_and_marks_get_memory(self):
        r = self.run_it("ok")
        self.assertTrue(r["done"] and r["alive"])
        self.assertEqual([x["step"] for x in r["records"]], ["ready", "mark", "done"])
        self.assertEqual(r["records"][1]["mem"], {"own": 1})
        self.assertEqual(r["peak"], {"own": 1, "tree": 2})
        with open(os.path.join(self.tmp.name, "w", "ok.js"), encoding="utf-8") as f:
            js = f.read()
        self.assertTrue(js.rstrip().endswith('main({"mode": "main", "passes": 3});'))

    def test_exit_before_done(self):
        r = self.run_it("exit")
        self.assertFalse(r["done"])
        self.assertFalse(r["alive"])
        self.assertEqual(r["code"], 7)
        self.assertEqual([x["step"] for x in r["records"]], ["ready"])

    def test_timeout_without_records(self):
        r = self.run_it("silent", timeout=1)
        self.assertFalse(r["done"])
        self.assertEqual(r["records"], [])
        self.assertTrue(r["alive"])
        self.assertFalse(ck.check_ready(r)[0])


HARNESS = textwrap.dedent(
    """\
    // Окно подставное: Go-методы, события и /media/lp/ ведут себя как у программы; печатает записи зонда строками.
    const fs = require('fs'), vm = require('vm')
    const mode = process.argv[3], victim = 'pack-a'
    const emitted = []
    const listeners = {}
    const sheetOk = process.env.HARNESS_BAD !== 'cell'
    const tile = (p, k, i) => ({ pack: p, url: '/media/lp/' + p + '/' + k + '/' + i + '?v=abcd1234', bytes: 100 })
    const App = {
      LibraryPacks: async () => ({ packs: [], available: [{ id: 'pack-b', bytes: 50, why: '', donor: '', removed: false }, { id: 'pack-a', bytes: 90, why: '', donor: '', removed: false }] }),
      UpdateLibrary: async () => ({ added: 0, updated: 0, available: 2, failed: [] }),
      DecorCatalog: async (k) => ({ get: k === 'bg' && App._tiles ? [tile('pack-a', 'bg', 0), tile('pack-a', 'bg', 1), tile('pack-c', 'bg', 0)].filter((t) => !(App._gone && t.pack === victim)) : [] }),
      FetchPackPreviews: async () => { App._tiles = true; setTimeout(() => (listeners['library:previews'] || []).forEach((f) => f({ ready: 2 })), 5) },
      DownloadLibraryPack: async () => ({ picked: true, why: '' }),
      SetLibraryNet: async (a) => { App._tiles = a },
    }
    if (mode !== 'main') App._tiles = true
    const bytes = new Uint8Array(500).map((_, i) => i * 7)
    const text = (s) => new TextEncoder().encode(s)
    const resp = (status, headers, body) => ({ status, headers: { get: (h) => headers[h.toLowerCase()] ?? null }, arrayBuffer: async () => body.buffer.slice(body.byteOffset, body.byteOffset + body.length) })
    const fetchMock = async (url, init) => {
      const u = new URL(url, 'http://wails.localhost/')
      if (init && init.signal && init.signal.aborted) throw Object.assign(new Error('aborted'), { name: 'AbortError' })
      const m = /^\\/media\\/lp\\/([^/]+)\\/bg\\/(\\d)$/.exec(decodeURIComponent(u.pathname).replace('%2F', '/'))
      const method = (init && init.method) || 'GET'
      if (u.pathname === '/') return resp(200, { 'content-type': 'text/html' }, text('<html>INDEX</html>'))
      if (u.pathname.startsWith('/media/') && (u.pathname.endsWith('/') || u.pathname.endsWith('/index.html'))) return resp(400, { 'content-type': 'text/plain; charset=utf-8' }, text('неверный адрес\\n'))
      if (mode === 'corrupt' && m && m[1] === victim) App._gone = true // как программа: лист не сошёлся с sha256 и удалён
      if (method === 'POST') return resp(405, { allow: 'GET, HEAD', 'content-type': 'text/plain; charset=utf-8' }, text('метод не поддерживается'))
      if (m && App._tiles && sheetOk && (m[1] === victim || m[1] === 'pack-c') && !(mode === 'corrupt' && m[1] === victim)) {
        return resp(200, { 'content-type': 'image/jpeg', 'x-content-type-options': 'nosniff', 'cache-control': 'no-cache' }, method === 'HEAD' ? new Uint8Array(0) : bytes)
      }
      return resp(404, { 'content-type': 'text/plain; charset=utf-8' }, text('404 page not found\\n'))
    }
    const sandbox = {
      window: { go: { app: { App } }, runtime: {
        EventsEmit: (n, d) => { if (n === 'qaprobe') console.log(d) },
        EventsOn: (n, f) => { (listeners[n] = listeners[n] || []).push(f); return () => {} } } },
      location: { href: 'http://wails.localhost/' }, fetch: fetchMock, URL, TextDecoder, TextEncoder, AbortController, Uint8Array,
      setTimeout, clearTimeout, Math, Date, JSON, Promise, Object, String, Error, decodeURIComponent, console,
    }
    sandbox.window.fetch = fetchMock
    vm.createContext(sandbox)
    vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), sandbox)
    """
)


@unittest.skipUnless(shutil.which("node"), "нужен node")
class ProbeScript(unittest.TestCase):
    """library_probe.js под node с подставным окном; записи проходят через те же check_*, что и в прогоне."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def run_js(self, mode, bad=""):
        cfg = {"mode": mode, "victim": "pack-a", "paths": ck.PATHS, "passes": 4, "roundMaxMs": 5000, "pace": 0.001}
        harness = os.path.join(self.tmp.name, "harness.js")
        with open(harness, "w", encoding="utf-8") as f:
            f.write(HARNESS)
        script = os.path.join(self.tmp.name, "probe.js")
        with open(script, "w", encoding="utf-8") as f:
            f.write(rn.probe_script(cfg))
        env = dict(os.environ, HARNESS_BAD=bad)
        r = subprocess.run(["node", harness, script, mode], capture_output=True, timeout=60, env=env)
        self.assertEqual(r.returncode, 0, r.stderr.decode("utf-8", "replace"))
        return [json.loads(ln) for ln in r.stdout.decode("utf-8").splitlines() if ln.startswith("{")]

    def test_syntax(self):
        r = subprocess.run(["node", "--check", rn.PROBE_JS], capture_output=True)
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_main_records_pass_every_check(self):
        recs = self.run_js("main")
        self.assertEqual(recs[-1], dict(recs[-1], step="done", ok=True))
        marks = [r for r in recs if r["step"] == "mark"]
        self.assertEqual([m["name"] for m in marks], ["baseline", "r1", "r2"])
        mem = {"own": 100 * ck.MIB, "own_ws": 1, "tree": 300 * ck.MIB, "tree_ws": 1, "procs": 3}
        for m in marks:
            m["mem"] = mem
        run = {"records": recs, "done": True, "alive": True, "code": None, "seconds": 1, "peak": {"own": 1, "tree": 1}}
        for name, v in (("ready", ck.check_ready(run)), ("update", ck.check_update(recs)), ("tiles", ck.check_tiles(recs)), ("overlap", ck.check_overlap(recs)),
                        ("good", ck.check_good(recs)), ("paths", ck.check_paths(recs)), ("stress", ck.check_stress(recs, passes=4)),
                        ("memory", ck.check_memory(recs, run)), ("revoke", ck.check_revoke(recs))):
            self.assertTrue(v[0], (name, v[1]))
        self.assertEqual(ck.step_of(recs, "victim")["victim"], "pack-a")
        self.assertEqual(ck.step_of(recs, "overlap")["pack"], "pack-b")  # меньший по размеру набор из «Можно скачать»

    def test_main_breakage_is_seen(self):
        recs = self.run_js("main", bad="cell")  # клетки отвечают 404: годные ответы и листание обязаны провалиться
        self.assertFalse(ck.check_good(recs)[0])
        self.assertFalse(ck.check_stress(recs, passes=4)[0])

    def test_corrupt_and_refetch_modes(self):
        c = self.run_js("corrupt")
        self.assertTrue(ck.check_corrupt(c, False)[0], ck.check_corrupt(c, False)[1])
        r = self.run_js("refetch")
        self.assertTrue(ck.check_refetch(r, True)[0], ck.check_refetch(r, True)[1])


if __name__ == "__main__":
    unittest.main()
