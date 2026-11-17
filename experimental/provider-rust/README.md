# Experimental Rust mirror of the provider foundation

Draft-only raw FFI declarations for
[`include/steer/draft/provider_v1.h`](../../include/steer/draft/provider_v1.h).
Semantic contract: [provider-v1-draft.md](../../docs/provider-v1-draft.md);
coverage: [provider-v1-header-coverage.md](../../docs/provider-v1-header-coverage.md).
This is an ABI consumer fixture, not a functional provider or host. It does not
advertise stable compatibility, alter packaging, or replace legacy APIs/net1.

No dependencies; declarations use `no_std` and `core`, without an allocator.
MSRV is explicitly Rust 1.77 (edition 2021), chosen for stable
`core::mem::offset_of!`. No claim is made that this version is installed locally.
`publish = false`; the crate is separate from stable SDK build/CI entry points.

## Mapping and static audit

All 14 concrete C records are mirrored with the same field order and `repr(C)`:
span/mut_span, error, operation_api/ref, async_start, target, combination,
descriptor, endpoint, buffer_api, host_api, config, provider_api. The two
callback typedefs and query function typedef are separate aliases. The ten
opaque handle declarations are private, zero-sized Rust marker types used only
behind raw pointers; they do not describe a native allocation's size.

`uint8_t/16_t/32_t/64_t` map to `u8/u16/u32/u64`, C pointers retain constness and
indirection. No Rust enum/bool/Vec/String, reference or trait object crosses FFI.
Function slots and callback typedefs use nullable `Option<unsafe extern "C" fn>`;
`extern "C"` follows Windows `__cdecl`, not Windows `system`/stdcall ABI.
Required callbacks/functions still require explicit non-null validation.
By-value records/return values follow the target C ABI, not a wire format.

Descriptor target is a pointer; combinations are pointers to pointers to const
records. Buffer `retain` returns `steer_pv1_error`, including overflow failure.
Endpoint port/reserved are u16; generation/deadline and monotonic time are u64.
All versioned tables keep u32 size/version prefixes. The entrypoint is declared,
never implemented or called here, without a `link` library attribute. Layout
tests reference no entrypoint symbol and need no provider runtime library.

Raw pointers and Copy spans do not enforce lifetime, ownership, shard affinity,
size/version validation, principal authenticity or cancellation barriers.
Caller must honor the C header rules: check tables before field access, retain
successfully before async handoff, end writable borrows before sharing/mutation,
release operations after terminal callback and destroy instances after barrier.
Rust implementations must not unwind/panic across FFI. This crate supplies no
safe wrapper, allocator, runtime, async executor, secret access or data IO.

## Prepared checks, not executed

Source tests check versioned prefixes, short-field rejection and arithmetic
overflow, nullable callback slot widths, pointer layout, endpoint offsets,
config u64 alignment, retain signature, and target indirection. Formulas account
for target pointer/u64 alignment rather than assuming every 32-bit target is the
same. These are Rust-side ABI regression assertions, not independent C/Rust
layout comparisons or native cross-language execution evidence.

On 2026-11-17 Rust remained unavailable: no rustc/cargo on Windows PATH,
Windows ~/.cargo/bin absent, and neither tool found in inspected WSL Ubuntu
PATH or user installation locations. No toolchain was installed. Cargo tests,
rustfmt, Rust compilation and cross-language comparison remain unverified.

## Independent C/Rust output fixtures (C emitter run only)

`layout.c` includes the actual SDK header and uses C11 `sizeof`, `_Alignof` and
`offsetof`. `src/bin/layout.rs` imports the Rust declarations and uses
`core::mem::{size_of, align_of, offset_of}` with `std` printing in this standalone
fixture binary. The library itself remains `no_std`. Neither emitter references
or invokes a provider entrypoint; no runtime library is needed.

Both explicitly enumerate all 14 concrete records and every current field,
including function slots, target indirection, descriptor digest and config u64
generation/deadline. They also emit widths/alignments of u8/u16/u32/u64,
void pointers and the three callback/query typedefs. Opaque allocations cannot
be measured and are excluded. Output has one decimal value per line:

```text
Type.size=record size
Type.align=record alignment
Type.offset.FIELD=field offset
constant.NAME=numeric ABI constant
```

The `offset` namespace avoids colliding with the real field named `size`.
No timestamps, machine paths or addresses are emitted. Values come from the
compiled target ABI, not shared expected-layout formulas.
All 24 current numeric ABI constants are emitted too: layout, status,
disposition, direction, flow, mode, endpoint and table tags. The string entrypoint
name and preprocessor helper macros are excluded.

Future validation must compile both emitters for the **same** OS/CPU/ABI/libc,
pointer width, endianness, calling convention and alignment/packing flags, then
run both on that target or a validated runner. Comparing a native C executable
with a cross-target Rust executable is invalid. Record exact compiler versions,
target triples and flags alongside each saved pair, outside the key/value output.
C requires a compiler supporting C11 `_Alignof` and the header calling convention;
Rust requires the declared MSRV. Cross-compilation alone is not runtime evidence.

After explicit authorization, a native Linux example from this directory is:

```sh
cc -std=c11 layout.c -o layout-c
cargo build --bin layout
./layout-c > c-layout.txt
./target/debug/layout > rust-layout.txt
python3 compare_layout.py c-layout.txt rust-layout.txt
```

`compare_layout.py` (Python 3 standard library only) reads the two saved files;
it never compiles or executes anything else. It requires exact key sets and
values, rejects empty output, duplicate keys, malformed keys/integers and values
outside uint64. Exit codes: 0 match, 1 layout mismatch, 2 invalid input/IO/usage.
The comparator cannot establish provenance or notice a field omitted identically
from both emitters; source enumeration must track header changes during review.
Equal layouts alone do not prove compatible callback signatures or semantics.

On 2026-11-17 the C emitter compiled and ran on WSL Ubuntu x86_64 Linux,
GCC 15.2.0 (Ubuntu 15.2.0-16ubuntu1), x86_64-linux-gnu, 64-bit:

```sh
cc -std=c11 -Wall -Wextra -Werror -pedantic layout.c -o /tmp/steer-provider-validation/layout-c
/tmp/steer-provider-validation/layout-c > /tmp/steer-provider-validation/c-layout.txt
```

It emitted 147 entries and exited 0. Rust compilation/tests and the independent
C/Rust comparison remain unverified because Rust is unavailable. No dependency
download or Cargo.lock generation occurred. C-only results do not prove matching
Rust ABI, callback execution or 32-bit portability.