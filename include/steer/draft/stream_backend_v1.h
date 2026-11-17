/* DRAFT companion: origin-owned streams, distinct from stream_v1 native-fd IO.
 * Not a stable ABI and not a provider loader/runtime availability claim. */
#ifndef STEER_DRAFT_STREAM_BACKEND_V1_H
#define STEER_DRAFT_STREAM_BACKEND_V1_H
#include "flow_v1.h"
#ifdef __cplusplus
extern "C" {
#endif
#define STEER_PV1_STREAM_BACKEND_LAYOUT UINT32_C(1)
#define STEER_PV1_STREAM_CAP_HALF_CLOSE UINT64_C(1)
struct steer_pv1_backend_stream;
struct steer_pv1_backend_stream_api;
/* One owning reference, copied only for ownership transfer. Never cast handle
 * to a native stream/fd, and invoke only its accompanying origin API. The API
 * and provider/generation stay alive through stream AND all operation releases. */
struct steer_pv1_stream_ref {
    struct steer_pv1_backend_stream *handle;
    const struct steer_pv1_backend_stream_api *api;
};
typedef void (STEER_PV1_CALL *steer_pv1_backend_io_done)(void *,
    struct steer_pv1_error,struct steer_pv1_buffer *,uint32_t bytes);
typedef void (STEER_PV1_CALL *steer_pv1_backend_finish_done)(void *,struct steer_pv1_error);
/* Same owner-shard and async_start rules as provider_v1. Never inline callbacks.
 * Each PENDING operation returns its own origin-owned operation_ref. Buffer
 * ownership transfers only on PENDING and returns at its terminal callback entry.
 * Read/write length>0, range within capacity, one read and one write in flight
 * including queued completion. Read OK/0 is clean EOF; errors have bytes0.
 * A successful partial write reports exactly its accepted prefix. Accepted
 * prefixes remain ordered; there are no unbounded hidden payload copies.
 *
 * HALF_CLOSE must be advertised per handle, never inferred from table presence.
 * finish_write rejects POLICY_DENIED while a write is pending, including its
 * queued callback: caller must learn its partial byte count and submit any
 * remaining suffix first. At terminal callback entry that count is known.
 * Completed write operation references need not be released before finish;
 * they still count against the explicit operation-admission budget.
 * finish_write PENDING installs an irreversible write barrier immediately:
 * later writes are rejected without ownership transfer. It waits for previously
 * accepted writes and their ordered protocol output before sending one EOF.
 * Success does not mean remote acknowledgement. Read remains usable throughout.
 * A second finish is rejected; it cannot emit another EOF. Unsupported returns
 * REJECTED/UNSUPPORTED and never masquerades as clean EOF. Cancellation of an
 * accepted finish aborts the stream unless its queued terminal result already
 * won. It never reopens write. Other operation cancellation detaches IO before
 * deferred completion; a queued successful result may win.
 *
 * close is idempotent abort: detach IO/readiness/timers before return, then queue
 * pending cancellations. No new IO accepted. release consumes the sole stream
 * reference after close and every EXPORTED operation release; otherwise returns
 * POLICY_DENIED. No callback may access a released handle. Caller may not release
 * an operation until its terminal callback returns. No panic/exception across FFI. */
struct steer_pv1_backend_stream_api {
    uint32_t size,version;
    uint64_t (STEER_PV1_CALL *capabilities)(const struct steer_pv1_backend_stream *);
    struct steer_pv1_async_start (STEER_PV1_CALL *read)(struct steer_pv1_backend_stream *,
        struct steer_pv1_buffer *,uint32_t offset,uint32_t length,
        steer_pv1_backend_io_done,void *);
    struct steer_pv1_async_start (STEER_PV1_CALL *write)(struct steer_pv1_backend_stream *,
        struct steer_pv1_buffer *,uint32_t offset,uint32_t length,
        steer_pv1_backend_io_done,void *);
    struct steer_pv1_async_start (STEER_PV1_CALL *finish_write)(struct steer_pv1_backend_stream *,
        steer_pv1_backend_finish_done,void *);
    void (STEER_PV1_CALL *close)(struct steer_pv1_backend_stream *);
    struct steer_pv1_error (STEER_PV1_CALL *release)(struct steer_pv1_backend_stream *);
};
/* When those public prerequisites hold, release MUST consume the public
 * reference. Invisible private jobs retain independent object/provider/module/
 * generation ownership through callback return/quiescence; they must not block
 * public consumption. They may not access the consumed public handle. An open
 * result cannot transfer preexisting exported operations to its recipient. */
/* PENDING copies the borrowed flow/deadline inputs and yields exactly one
 * deferred open callback on its owner shard. Generation/identity belongs to the
 * immutable host-selected context, never a peer-provided pointer. Host checks
 * exact flow generation before open and retains the binding/provider until all
 * open operations, returned streams AND IO operations have been released.
 * Success transfers one origin stream_ref. Error transfers none (empty ref,
 * NULL endpoints). A queued success may win cancellation; its recipient must
 * close/release the returned reference if it no longer wants that result.
 * Bound/peer are optional truthful borrowed views valid only during callback;
 * NULL means unavailable, never a fabricated socket identity. Original target
 * remains in separately owned flow metadata. Open means a usable protocol
 * stream; protocols without target-connect acknowledgements cannot claim proof
 * of final target reachability. Deadline is mandatory absolute monotonic ns,
 * strictly in the future. No raw errno or independent socket/DNS authority. */
#define STEER_PV1_STREAM_DIAL_NONE UINT32_C(0)
#define STEER_PV1_STREAM_DIAL_REFUSED UINT32_C(1)
#define STEER_PV1_STREAM_DIAL_NETWORK_UNREACHABLE UINT32_C(2)
#define STEER_PV1_STREAM_DIAL_HOST_UNREACHABLE UINT32_C(3)
#define STEER_PV1_STREAM_DIAL_OTHER UINT32_C(4)
/* Optional protocol-mapping diagnostic; borrowed on callback, never errno.
 * Unknown size/version/kind means unavailable, not a guessed failure reason. */
struct steer_pv1_stream_open_diagnostic {uint32_t size,version,dial_failure;};
typedef void (STEER_PV1_CALL *steer_pv1_stream_open_done)(void *,struct steer_pv1_error,
    struct steer_pv1_stream_ref,const struct steer_pv1_endpoint *bound,
    const struct steer_pv1_endpoint *peer,const struct steer_pv1_stream_open_diagnostic *);
struct steer_pv1_stream_backend_api {
    uint32_t size,version;
    struct steer_pv1_async_start (STEER_PV1_CALL *open)(void *context,
        const struct steer_pv1_flow_metadata *,uint64_t deadline_ns,
        steer_pv1_stream_open_done,void *);
};
#ifdef __cplusplus
}
#endif
#endif
