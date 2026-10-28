"""Диапазоны compat для заглушек хостов: ^M.m, ~M.m, >=, <=, >, <, M.m, через пробел (и) и || (или).

Общий для заглушек splify2-*/ и steer-*/: у всех осей (steer-api, splify2-api, host-api, steer-abi) диапазон читается одинаково.
"""


def _ver(s):
    p = [int(x) for x in s.strip().lstrip("v").split(".")[:3]]
    return tuple(p + [0] * (3 - len(p)))


def range_has(rng, have):
    """Входит ли версия have (кортеж из двух-трёх чисел) в диапазон rng."""
    have = tuple(list(have) + [0] * (3 - len(have)))
    for alt in rng.split("||"):
        toks = alt.split()
        if not toks:
            continue
        good = True
        try:
            for t in toks:
                if t.startswith("^"):
                    v = _ver(t[1:]); good &= have >= v and have[0] == v[0]
                elif t.startswith("~"):
                    v = _ver(t[1:]); good &= have >= v and have[:2] == v[:2]
                elif t.startswith(">="):
                    good &= have >= _ver(t[2:])
                elif t.startswith("<="):
                    good &= have <= _ver(t[2:])
                elif t.startswith(">"):
                    good &= have > _ver(t[1:])
                elif t.startswith("<"):
                    good &= have < _ver(t[1:])
                else:
                    good &= have == _ver(t.lstrip("="))
        except ValueError:
            return False
        if good:
            return True
    return False
