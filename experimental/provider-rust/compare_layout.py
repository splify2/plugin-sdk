"""Compare saved same-target layout output; never builds or runs emitters."""

import argparse
from pathlib import Path
import re
import sys


LINE = re.compile(r"((?:[A-Za-z_][A-Za-z0-9_]*\.(?:size|align|offset\.[A-Za-z_][A-Za-z0-9_]*))|(?:constant\.[A-Za-z_][A-Za-z0-9_]*))=(0|[1-9][0-9]*)", re.ASCII)
MAX_BYTES = 1024 * 1024


def read_layout(path):
    with path.open("rb") as source:
        raw = source.read(MAX_BYTES + 1)
    if len(raw) > MAX_BYTES:
        raise ValueError(f"{path}: layout output exceeds {MAX_BYTES} bytes")
    text = raw.decode("ascii")
    result = {}
    for number, line in enumerate(text.splitlines(), 1):
        match = LINE.fullmatch(line)
        if match is None:
            raise ValueError(f"{path}:{number}: expected key=nonnegative decimal integer")
        key, digits = match.groups()
        if key in result:
            raise ValueError(f"{path}:{number}: duplicate key {key}")
        # Layout offsets/sizes must fit a target's unsigned 64-bit size_t.
        if len(digits) > 20 or int(digits) > (1 << 64) - 1:
            raise ValueError(f"{path}:{number}: value exceeds uint64")
        result[key] = int(digits)
    if not result:
        raise ValueError(f"{path}: empty layout output")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("c_output", type=Path)
    parser.add_argument("rust_output", type=Path)
    args = parser.parse_args()
    try:
        c_values = read_layout(args.c_output)
        rust_values = read_layout(args.rust_output)
    except (OSError, ValueError) as error:
        print(str(error), file=sys.stderr)
        return 2
    mismatches = []
    for key in sorted(c_values.keys() | rust_values.keys()):
        if key not in c_values:
            mismatches.append(f"{key}: missing from C output")
        elif key not in rust_values:
            mismatches.append(f"{key}: missing from Rust output")
        elif c_values[key] != rust_values[key]:
            mismatches.append(f"{key}: C={c_values[key]} Rust={rust_values[key]}")
    if mismatches:
        print("\n".join(mismatches), file=sys.stderr)
        return 1
    print(f"matching layout: {len(c_values)} keys")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
