"""Упаковка выпуска архивом с папкой (спека 039, design §2). Только stdlib, Python >= 3.9.

  pack.py windows --exe E --version X.Y.Z --guide G --changelog C --notices N --out D [--date ISO]
  pack.py macos   --app-zip Z --version X.Y.Z --guide G --changelog C --notices N --out D [--date ISO]
  pack.py check   ZIP [--os windows|macos] [--version X.Y.Z]

Упаковка: последняя строка stdout — путь к архиву; коды 0 — готово, 1 — ошибка входа или
проверки (текст в stderr), 2 — неверные аргументы.
check: 0 — упакован по design §2.2; 3 — «голый» архив прежнего вида (.app или exe в корне);
1 — что-то третье или небезопасное.

Раскладка и имена — только здесь (константы ниже): остальные скрипты зовут pack.py.
"""

import argparse
import datetime
import hashlib
import os
import posixpath
import re
import stat
import sys
import time
import zipfile

APP = "sosninaalbums.app"
EXE = "SosninaAlbums.exe"
NOTICES = "THIRD_PARTY_NOTICES.txt"
CHANGES = "Что нового.txt"
RSRC = "__MACOSX"
GUIDE_STEM = {"windows": "Как установить (Windows)", "macos": "Как установить (Mac)"}
GUIDE_EXT = (".pdf", ".docx")
ARCHIVE = {
    "windows": "SosninaAlbums-{v}-windows-x64.zip",
    "macos": "SosninaAlbums-{v}-macos-universal.zip",
}
REPO_URL = "https://github.com/IliaSA101/sosninaalbums"
MAX_UNPACKED = 1 << 30  # несжатый объём, 1 ГБ
MAX_LINK = 4096
BARE, BAD = 3, 1


class PackError(Exception):
    """Ошибка входа или проверки: текст уходит в stderr, код 1."""


def folder_name(version):
    return "Sosnina Albums " + version


def guide_name(osname, ext):
    return GUIDE_STEM[osname] + ext


# ---------- безопасность путей ----------

def check_name(name):
    """Имя записи: относительное, без «..», «.», пустых частей, обратных косых, диска, управляющих."""
    if not name or name.startswith("/") or "\\" in name:
        raise PackError("небезопасное имя записи: %r" % name)
    if any(ord(c) < 32 or c == "\x7f" for c in name):
        raise PackError("управляющий символ в имени записи: %r" % name)
    parts = name.rstrip("/").split("/")
    if any(p in ("", ".", "..") for p in parts) or re.match(r"^[A-Za-z]:", parts[0]):
        raise PackError("небезопасное имя записи: %r" % name)
    return parts


def mode_of(info):
    return (info.external_attr >> 16) & 0xFFFF if info.create_system == 3 else 0


def is_link(info):
    return stat.S_ISLNK(mode_of(info))


def check_link(zf, info, root):
    """Ссылка допустима, если цель относительная и остаётся внутри root (.app)."""
    if info.file_size > MAX_LINK:
        raise PackError("слишком длинная символьная ссылка: %r" % info.filename)
    target = zf.read(info).decode("utf-8", "replace")
    if not target or target.startswith("/") or "\\" in target or "\x00" in target:
        raise PackError("небезопасная ссылка %r -> %r" % (info.filename, target))
    joined = posixpath.normpath(posixpath.join(posixpath.dirname(info.filename), target))
    if joined != root and not joined.startswith(root + "/"):
        raise PackError("ссылка %r -> %r выходит из %s" % (info.filename, target, root))


def app_root_of(name):
    """Корень .app, внутри которого может лежать ссылка: «X.app», «Sosnina Albums V/X.app», «__MACOSX»."""
    parts = name.split("/")
    if parts[0] == RSRC:
        return RSRC
    if parts[0].endswith(".app"):
        return parts[0]
    if len(parts) > 1 and parts[0].startswith("Sosnina Albums ") and parts[1].endswith(".app"):
        return parts[0] + "/" + parts[1]
    return None


def check_entries(zf):
    """Общие правила путей, ссылок и объёма для всех записей."""
    seen = set()
    total = 0
    for info in zf.infolist():
        check_name(info.filename)
        if info.filename in seen:
            raise PackError("повтор записи: %r" % info.filename)
        seen.add(info.filename)
        total += info.file_size
        if is_link(info):
            root = app_root_of(info.filename)
            if root is None:
                raise PackError("символьная ссылка вне .app: %r" % info.filename)
            check_link(zf, info, root)
    if total > MAX_UNPACKED:
        raise PackError("несжатый объём %d байт больше предела %d" % (total, MAX_UNPACKED))


def check_macos_app(zf, app_prefix):
    """Contents/Info.plist и хотя бы один исполняемый файл в Contents/MacOS/, все — с битом x."""
    names = {i.filename: i for i in zf.infolist()}
    if app_prefix + "Contents/Info.plist" not in names:
        raise PackError("нет Contents/Info.plist")
    macos = [i for i in zf.infolist()
             if i.filename.startswith(app_prefix + "Contents/MacOS/") and not i.filename.endswith("/")]
    if not macos:
        raise PackError("нет файлов в Contents/MacOS/")
    for i in macos:
        if not mode_of(i) & 0o111:
            raise PackError("нет бита исполнения у %r" % i.filename)


# ---------- время и записи ----------

def parse_date(text):
    t = text.strip()
    if t.endswith(("Z", "z")):
        t = t[:-1] + "+00:00"
    try:
        dt = datetime.datetime.fromisoformat(t)
    except ValueError:
        raise argparse.ArgumentTypeError("не ISO-время: %r" % text)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=datetime.timezone.utc)
    dt = dt.astimezone(datetime.timezone.utc)
    if dt.year < 1980:
        raise argparse.ArgumentTypeError("время раньше 1980: %r" % text)
    return (dt.year, dt.month, dt.day, dt.hour, dt.minute, dt.second)


def new_info(name, date_time, mode):
    zi = zipfile.ZipInfo(name, date_time)
    zi.create_system = 3
    zi.external_attr = (stat.S_IFREG | mode) << 16
    zi.compress_type = zipfile.ZIP_DEFLATED
    return zi


class Writer:
    """Пишет архив во временный файл рядом и переименовывает при успехе."""

    def __init__(self, path):
        self.path = path
        self.tmp = path + ".part"
        self.zf = zipfile.ZipFile(self.tmp, "w", zipfile.ZIP_DEFLATED, compresslevel=9)
        self.listing = []

    def add(self, info, data):
        self.zf.writestr(info, data, compresslevel=9)
        self.listing.append((info.filename, len(data)))

    def close(self, ok):
        self.zf.close()
        if ok:
            os.replace(self.tmp, self.path)
        elif os.path.exists(self.tmp):
            os.remove(self.tmp)


# ---------- «Что нового.txt» ----------

def changes_section(text, version):
    """Раздел «## X.Y.Z …» до следующего «## » (как awk в release.yml); None — раздела нет."""
    head = "## " + version
    body, on = [], False
    for line in text.splitlines():
        if line.startswith("## "):
            if on:
                break
            on = line == head or line.startswith(head + " ")
            continue
        if on:
            body.append(line.rstrip())
    while body and not body[0].strip():
        body.pop(0)
    while body and not body[-1].strip():
        body.pop()
    return body if body else None


def plain_line(line):
    m = re.match(r"^#{3,6}\s+(.*?)\s*$", line)
    if m:
        return m.group(1) + ":"
    line = re.sub(r"\[([^\]]*)\]\(([^)\s]*)\)", r"\1 (\2)", line)
    line = line.replace("**", "").replace("`", "")
    return line


def changes_text(changelog_path, version, newline):
    """Текст файла «Что нового»: (байты UTF-8 с BOM, предупреждение|None)."""
    with open(changelog_path, encoding="utf-8-sig") as f:
        src = f.read()
    head = "Sosnina Albums %s — что нового" % version
    url = "%s/blob/v%s/CHANGELOG.md" % (REPO_URL, version)
    sec = changes_section(src, version)
    if sec is None:
        lines, warn = [head, url], "В CHANGELOG.md нет раздела «## %s» — «Что нового.txt» без списка" % version
    else:
        lines, warn = [head, ""] + [plain_line(x) for x in sec] + ["", url], None
    return ("\ufeff" + newline.join(lines) + newline).encode("utf-8"), warn


# ---------- упаковка ----------

def read_file(path, what):
    if not path or not os.path.isfile(path):
        raise PackError("нет входного файла (%s): %s" % (what, path))
    with open(path, "rb") as f:
        return f.read()


def guide_entry(osname, path):
    data = read_file(path, "инструкция")
    ext = os.path.splitext(path)[1].lower()
    if ext not in GUIDE_EXT:
        raise PackError("инструкция должна быть .pdf или .docx: %s" % path)
    warn = None
    if ext == ".docx":
        warn = "Инструкция в архиве — docx вместо pdf (сбой конвертации)"
    return guide_name(osname, ext), data, warn


def pack(args):
    osname, v = args.cmd, args.version
    folder = folder_name(v)
    now = time.localtime()[:6]
    stamp = args.date or now
    notices = read_file(args.notices, "уведомления")
    guide, guide_data, warn_guide = guide_entry(osname, args.guide)
    newline = "\r\n" if osname == "windows" else "\n"
    if not os.path.isfile(args.changelog):
        raise PackError("нет входного файла (CHANGELOG): %s" % args.changelog)
    changes, warn_changes = changes_text(args.changelog, v, newline)
    if osname == "windows":
        src = read_file(args.exe, "exe")
    else:
        if not args.app_zip or not os.path.isfile(args.app_zip):
            raise PackError("нет входного файла (zip программы): %s" % args.app_zip)
    os.makedirs(args.out, exist_ok=True)
    out = os.path.join(args.out, ARCHIVE[osname].format(v=v))
    w = Writer(out)
    ok = False
    try:
        if osname == "windows":
            w.add(new_info(folder + "/" + EXE, stamp, 0o755), src)
        else:
            copy_app(args.app_zip, folder, args.date, w)
        w.add(new_info(folder + "/" + guide, stamp, 0o644), guide_data)
        w.add(new_info(folder + "/" + CHANGES, stamp, 0o644), changes)
        w.add(new_info(folder + "/" + NOTICES, stamp, 0o644), notices)
        ok = True
    finally:
        w.close(ok)
    for warn in (warn_guide, warn_changes):
        if warn:
            print("::warning::" + warn)
    for name, size in w.listing:
        print("%10d  %s" % (size, name))
    with open(out, "rb") as f:
        print("sha256  " + hashlib.sha256(f.read()).hexdigest())
    print(out)
    return 0


def copy_app(app_zip, folder, date_time, w):
    """Записи zip от ditto — в новый архив под папку выпуска, без распаковки (права, ссылки целы)."""
    try:
        zin = zipfile.ZipFile(app_zip)
    except zipfile.BadZipFile as e:
        raise PackError("zip программы не читается: %s" % e)
    with zin:
        infos = zin.infolist()
        tops = set()
        for i in infos:
            parts = check_name(i.filename)
            if parts[0] == RSRC and len(parts) == 1 and not i.filename.endswith("/"):
                raise PackError("файл %s вместо папки" % RSRC)
            if parts[0] != RSRC:
                tops.add(parts[0] if len(parts) > 1 or i.filename.endswith("/") else "/" + parts[0])
        if tops != {APP}:
            raise PackError("в zip программы должен быть ровно один %s на верхнем уровне, найдено: %s"
                            % (APP, sorted(tops)))
        check_entries(zin)
        check_macos_app(zin, APP + "/")
        for i in infos:
            if i.filename == RSRC + "/":
                new = i.filename
            elif i.filename.startswith(RSRC + "/"):
                new = RSRC + "/" + folder + "/" + i.filename[len(RSRC) + 1:]
            else:
                new = folder + "/" + i.filename
            zi = zipfile.ZipInfo(new, date_time or i.date_time)
            zi.create_system = i.create_system
            zi.external_attr = i.external_attr
            zi.compress_type = zipfile.ZIP_STORED if i.is_dir() else i.compress_type
            w.add(zi, b"" if i.is_dir() else zin.read(i))


# ---------- check ----------

def check(path, osname, version):
    """Код 0 — упакован; BARE — «голый» архив; ошибки — PackError."""
    try:
        zf = zipfile.ZipFile(path)
    except (zipfile.BadZipFile, OSError) as e:
        raise PackError("архив не читается: %s" % e)
    with zf:
        infos = zf.infolist()
        if not infos:
            raise PackError("пустой архив")
        tops = {}
        for i in infos:
            parts = check_name(i.filename)
            if parts[0] != RSRC:
                tops.setdefault(parts[0], []).append(i)
        check_entries(zf)
        # «голый» прежний вид: единственная запись верхнего уровня — .app или exe
        if len(tops) == 1:
            only = next(iter(tops))
            kind = "macos" if only.endswith(".app") else "windows" if only.lower().endswith(".exe") else None
            if kind and (osname is None or osname == kind):
                if kind == "macos":
                    check_macos_app(zf, only + "/")
                return BARE
        if len(tops) != 1 or not next(iter(tops)).startswith("Sosnina Albums "):
            raise PackError("на верхнем уровне должна быть одна папка «Sosnina Albums X.Y.Z», найдено: %s"
                            % sorted(tops))
        folder = next(iter(tops))
        if version and folder != folder_name(version):
            raise PackError("папка %r, ожидалась %r" % (folder, folder_name(version)))
        prefix = folder + "/"
        inside = {}
        for i in tops[folder]:
            rest = i.filename[len(prefix):] if i.filename != prefix else ""
            if rest:
                inside.setdefault(rest.split("/")[0], []).append(i)
        kind = osname or ("macos" if APP in inside else "windows")
        main_item = APP if kind == "macos" else EXE
        want = {main_item, CHANGES, NOTICES}
        guides = {n for n in inside if n in {guide_name(kind, e) for e in GUIDE_EXT}}
        if len(guides) != 1 or set(inside) != want | guides:
            raise PackError("в папке должно быть ровно: %s и инструкция; найдено: %s"
                            % (", ".join(sorted(want)), sorted(inside)))
        for n, items in inside.items():
            plain = n != APP
            if plain and (len(items) != 1 or items[0].filename.endswith("/")):
                raise PackError("%r должен быть файлом" % n)
        if kind == "macos":
            check_macos_app(zf, prefix + APP + "/")
            for i in infos:
                if i.filename.startswith(RSRC + "/") and i.filename != RSRC + "/" \
                        and not i.filename.startswith(RSRC + "/" + prefix):
                    raise PackError("запись %r не под __MACOSX/%s" % (i.filename, folder))
        elif RSRC in {check_name(i.filename)[0] for i in infos}:
            raise PackError("__MACOSX в архиве Windows")
    return 0


def build_parser():
    p = argparse.ArgumentParser(prog="pack.py", description=__doc__.split("\n")[0])
    sub = p.add_subparsers(dest="cmd", required=True)
    for name in ("windows", "macos"):
        s = sub.add_parser(name)
        s.add_argument("--exe" if name == "windows" else "--app-zip", required=True)
        s.add_argument("--version", required=True, type=version_arg)
        s.add_argument("--guide", required=True)
        s.add_argument("--changelog", required=True)
        s.add_argument("--notices", required=True)
        s.add_argument("--out", required=True)
        s.add_argument("--date", type=parse_date)
    c = sub.add_parser("check")
    c.add_argument("zip")
    c.add_argument("--os", choices=("windows", "macos"))
    c.add_argument("--version", type=version_arg)
    return p


def version_arg(text):
    if not re.match(r"^[0-9A-Za-z][0-9A-Za-z.+-]*$", text):
        raise argparse.ArgumentTypeError("неверная версия: %r" % text)
    return text


def main(argv=None):
    try:
        args = build_parser().parse_args(argv)
    except SystemExit as e:
        return e.code if isinstance(e.code, int) else 2
    try:
        if args.cmd == "check":
            code = check(args.zip, args.os, args.version)
            print({0: "упакован верно", BARE: "прежний вид: %s в корне" % "программа"}[code])
            return code
        return pack(args)
    except PackError as e:
        print("pack.py: " + str(e), file=sys.stderr)
        return BAD
    except OSError as e:
        print("pack.py: " + str(e), file=sys.stderr)
        return BAD


if __name__ == "__main__":
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")
    sys.exit(main())
