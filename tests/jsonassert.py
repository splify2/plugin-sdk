#!/usr/bin/env python3
"""jsonassert.py файл.json 'выражение от d' — ноль, если выражение истинно (только для стенда)."""
import json, sys
d = json.load(open(sys.argv[1]))
ok = eval(sys.argv[2], {'d': d, 'len': len, 'sorted': sorted, 'set': set, 'all': all, 'any': any})
sys.exit(0 if ok else 1)
