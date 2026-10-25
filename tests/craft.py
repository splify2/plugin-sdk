#!/usr/bin/env python3
"""Враждебные архивы для стенда: из годового .s2p делает варианты с одним изъяном каждый.

    craft.py <годный.s2p> <каталог>

Пишет файлы в каталог и список «файл<TAB>фрагмент сообщения» в variants.txt: verify обязан отказать на
каждом и назвать причину. Только stdlib; нужен лишь стенду, самому s2p python не нужен."""
import gzip, hashlib, io, os, sys, tarfile

src, out = sys.argv[1], sys.argv[2]
os.makedirs(out, exist_ok=True)

def read(path):
    raw = gzip.decompress(open(path, 'rb').read())
    tf = tarfile.open(fileobj=io.BytesIO(raw))
    return [(m, tf.extractfile(m).read() if m.isfile() else b'') for m in tf.getmembers()]

def info(name, mode, typ=tarfile.REGTYPE, link=''):
    t = tarfile.TarInfo(name)
    t.mode, t.mtime, t.uid, t.gid, t.type, t.linkname = mode, 0, 0, 0, typ, link
    return t

def pack(entries, fmt=tarfile.USTAR_FORMAT):
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode='w', format=fmt) as tf:
        for t, data in entries:
            t.size = len(data) if t.isfile() else 0
            tf.addfile(t, io.BytesIO(data) if t.isfile() else None)
    z = io.BytesIO()
    with gzip.GzipFile(fileobj=z, mode='wb', mtime=0) as g:
        g.write(raw.getvalue())
    return z.getvalue()

def manifest_with(entries, extra):
    """MANIFEST с лишними строками extra = {путь: (режим, данные)}, строки по порядку."""
    for i, (t, d) in enumerate(entries):
        if t.name == 'MANIFEST':
            head, *lines = d.decode().splitlines()
            rows = {l.split(' ', 3)[3]: l for l in lines}
            for p, (mode, data) in extra.items():
                rows[p] = '%s %d %04o %s' % (hashlib.sha256(data).hexdigest(), len(data), mode, p)
            return i, ('\n'.join([head] + [rows[k] for k in sorted(rows)]) + '\n').encode()

good = read(src)
names = [t.name for t, _ in good]
variants = []

def emit(name, data, frag):
    open(os.path.join(out, name), 'wb').write(data)
    variants.append((name, frag))

def with_manifest_row(entry_list, extra):
    i, m = manifest_with(entry_list, extra)
    el = list(entry_list)
    el[i] = (el[i][0], m)
    return el

for name, frag in [('../evil', r'\.\.'), ('a/../../evil', r'\.\.'), ('/etc/evil', 'абсолютный'),
                   ('./evil', r'\./'), ('a//evil', 'двойной')]:
    payload = b'x'
    # перечислена в MANIFEST — и всё равно отказ: путь проверяется до сверки сумм
    emit('path-listed-%d.s2p' % len(variants), pack(with_manifest_row(good + [(info(name, 0o644), payload)], {name: (0o644, payload)})), frag)
    # не перечислена — отказ по тому же пути, а не только за «лишний файл»
    emit('path-unlisted-%d.s2p' % len(variants), pack(good + [(info(name, 0o644), payload)]), frag)

emit('symlink.s2p', pack(good + [(info('bin/link', 0o777, tarfile.SYMTYPE, '/etc/passwd'), b'')]), 'ссылка')
emit('hardlink.s2p', pack(good + [(info('bin/hl', 0o644, tarfile.LNKTYPE, 'plugin.json'), b'')]), 'ссылка')
emit('fifo.s2p', pack(good + [(info('bin/fifo', 0o644, tarfile.FIFOTYPE), b'')]), 'тип')

def with_mode(mode):
    el = []
    for t, d in good:
        if t.isfile() and t.name not in ('plugin.json', 'MANIFEST', 'MANIFEST.sig'):
            t = info(t.name, mode)
        el.append((t, d))
    return el
emit('setuid.s2p', pack(with_mode(0o4755)), 'setuid')
emit('mode-0666.s2p', pack(with_mode(0o666)), 'режим файла')
emit('pax.s2p', pack(good + [(info('i18n/пример-файл.json', 0o644), b'{}')], tarfile.PAX_FORMAT), 'не ustar')
emit('garbage-after-gzip.s2p', open(src, 'rb').read() + b'junk', 'мусор')
emit('two-members.s2p', open(src, 'rb').read() * 2, 'мусор|член')
tar_raw = gzip.decompress(open(src, 'rb').read())
emit('garbage-after-tar.s2p', gzip.compress(tar_raw + b'garbage after the end', mtime=0), 'мусор после конца tar')

by = {t.name: (t, d) for t, d in good}
order = [by['MANIFEST']] + [x for x in good if x[0].name != 'MANIFEST']
emit('order.s2p', pack(order), 'первые три записи')
emit('unlisted.s2p', pack(good + [(info('extra.txt', 0o644), b'surprise')]), 'не описан в MANIFEST')
victim = next(t.name for t, _ in good if t.isfile() and t.name not in ('plugin.json', 'MANIFEST', 'MANIFEST.sig'))
emit('missing.s2p', pack([x for x in good if x[0].name != victim]), 'нет в архиве')
emit('tampered.s2p', pack([(t, d + b' ' if t.name == victim else d) for t, d in good]), 'размер|sha256')
emit('tampered-same-size.s2p', pack([(t, (b'X' + d[1:]) if t.name == victim else d) for t, d in good]), 'sha256')
emit('header-id.s2p', pack([(t, d.replace(b'\n', b'\n', 1).replace(b's2p 1 ', b's2p 1 evil-', 1) if t.name == 'MANIFEST' else d) for t, d in good]), 'заголовок MANIFEST')

open(os.path.join(out, 'variants.txt'), 'w').write(''.join('%s\t%s\n' % v for v in variants))
print(len(variants), 'вариантов в', out)
