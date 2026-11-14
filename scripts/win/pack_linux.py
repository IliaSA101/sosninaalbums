"""tar.gz выпуска Linux: запись и проверка (спека 101, design §3.3). Только stdlib, Python >= 3.9.

Вызывается из pack.py. Архив воспроизводим: порядок записей фиксирован, uid/gid 0, mtime у всех один,
в заголовке gzip времени нет — два прогона на одних входах дают одинаковые байты.
"""

import gzip
import io
import os
import stat
import tarfile

from pack_common import (BIN, CHANGES, GUIDE_EXT, ICON, LEGAL, MAX_UNPACKED, PackError, check_name,
                         folder_name, guide_name)

DIR_MODE, FILE_MODE, EXEC_MODE = 0o755, 0o644, 0o755


def write_tar(path, folder, entries, mtime):
    """entries: [(имя внутри папки, байты, права)] — пишутся в данном порядке после самой папки."""
    raw = io.BytesIO()
    # filename="" и mtime=0: gzip не пишет в заголовок ни имени, ни времени
    with gzip.GzipFile(filename="", fileobj=raw, mode="wb", compresslevel=9, mtime=0) as gz:
        with tarfile.open(fileobj=gz, mode="w", format=tarfile.PAX_FORMAT) as tf:
            tf.addfile(_info(folder, tarfile.DIRTYPE, DIR_MODE, 0, mtime))
            for name, data, mode in entries:
                tf.addfile(_info(folder + "/" + name, tarfile.REGTYPE, mode, len(data), mtime), io.BytesIO(data))
    tmp = path + ".part"
    try:
        with open(tmp, "wb") as f:
            f.write(raw.getvalue())
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.remove(tmp)


def _info(name, kind, mode, size, mtime):
    ti = tarfile.TarInfo(name)
    ti.type, ti.mode, ti.size, ti.mtime = kind, mode, size, int(mtime)
    ti.uid = ti.gid = 0
    ti.uname = ti.gname = "root"
    return ti


def check_tar(path, version):
    """Проверка архива Linux: одна папка, нужный набор файлов, только файлы, права, без выхода наружу."""
    try:
        tf = tarfile.open(path, "r:gz")
        members = tf.getmembers()
    except (tarfile.TarError, OSError, EOFError) as e:
        raise PackError("архив не читается: %s" % e)
    with tf:
        if not members:
            raise PackError("пустой архив")
        seen, total, files, tops = set(), 0, {}, set()
        for m in members:
            parts = check_name(m.name)
            tops.add(parts[0])
            if m.name in seen:
                raise PackError("повтор записи: %r" % m.name)
            seen.add(m.name)
            total += m.size
            if not (m.isfile() or m.isdir()):
                raise PackError("в архиве только файлы и папка, а %r — ссылка или устройство" % m.name)
            if m.mode & (stat.S_ISUID | stat.S_ISGID | stat.S_ISVTX | 0o022):
                raise PackError("недопустимые права %o у %r" % (m.mode, m.name))
            if m.isfile():
                if len(parts) != 2:
                    raise PackError("файл %r не лежит прямо в папке выпуска" % m.name)
                files[parts[1]] = m
            elif len(parts) != 1:
                raise PackError("лишняя папка %r" % m.name)
        if total > MAX_UNPACKED:
            raise PackError("несжатый объём %d байт больше предела %d" % (total, MAX_UNPACKED))
        if len(tops) != 1 or not next(iter(tops)).startswith("Sosnina Albums "):
            raise PackError("на верхнем уровне должна быть одна папка «Sosnina Albums X.Y.Z», найдено: %s"
                            % sorted(tops))
        folder = next(iter(tops))
        if version and folder != folder_name(version):
            raise PackError("папка %r, ожидалась %r" % (folder, folder_name(version)))
        if folder not in seen:
            raise PackError("нет записи самой папки %r" % folder)
        guides = {n for n in files if n in {guide_name("linux", e) for e in GUIDE_EXT}}
        want = {BIN, ICON, CHANGES, *LEGAL}
        if len(guides) != 1 or set(files) != want | guides:
            raise PackError("в папке должно быть ровно: %s и инструкция; найдено: %s"
                            % (", ".join(sorted(want)), sorted(files)))
        if not files[BIN].mode & 0o111:
            raise PackError("нет бита исполнения у %r" % BIN)
        for n, m in files.items():
            if n != BIN and m.mode & 0o111:
                raise PackError("лишний бит исполнения у %r" % n)
    return 0
