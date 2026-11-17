/* Experimental companion; not a frozen ABI, no Rust mirror yet. */
#ifndef STEER_DRAFT_DATAGRAM_V1_H
#define STEER_DRAFT_DATAGRAM_V1_H
#include "flow_v1.h"
#ifdef __cplusplus
extern "C" {
#endif
struct steer_pv1_datagram_api;
/* Origin-owned pair. Never cast a provider handle to a native host datagram or
 * submit it to another API. Table/module and generation pins outlive reference,
 * all its operations and callbacks. Success returns one owning reference. */
struct steer_pv1_datagram_ref {
    struct steer_pv1_datagram *handle;
    const struct steer_pv1_datagram_api *api;
};
typedef void (STEER_PV1_CALL *steer_pv1_datagram_done)(void *,
    struct steer_pv1_error,struct steer_pv1_buffer *,uint32_t bytes);
/* Fixed-remote association. Host routing owns any multi-destination association
 * cache and evaluates each original requested destination before DNS/backend
 * open. Backends never silently dial direct when selected detour is unsupported.
 * One pending send and receive, including queued completion, per reference.
 * PENDING transfers the buffer's sole reference until callback entry. Discard
 * writable views first; no access/retain/resubmit/release while transferred.
 * Only that operation's reference returns, including errors/cancellation.
 * Offsets/lengths fit capacity. Send is atomic: success bytes==full length,
 * including a valid zero-length datagram; error bytes==0, never partial success.
 * Receive capacity is nonzero. Zero received bytes is a valid message, NEVER
 * EOF. Oversized messages are consumed/discarded exactly once and complete with
 * RESOURCE_EXHAUSTED/bytes0; the association remains usable, no truncated success.
 * Deadlines are absolute host monotonic nanoseconds, mandatory/in future.
 * Cancel detaches IO/deadline before deferred completion; queued success may win.
 * close is idempotent, detaches socket/IO before returning and cancels pending
 * operations. Release requires close plus all operation releases, after their
 * callbacks return. All calls/callbacks run on the owning shard. */
struct steer_pv1_datagram_api {
    uint32_t size,version;
    /* Zero means closed/unavailable, not a zero-only message transport. */
    uint32_t (STEER_PV1_CALL *message_limit)(struct steer_pv1_datagram *);
    struct steer_pv1_async_start (STEER_PV1_CALL *send)(struct steer_pv1_datagram *,
        struct steer_pv1_buffer *,uint32_t offset,uint32_t length,uint64_t deadline_ns,
        steer_pv1_datagram_done,void *);
    struct steer_pv1_async_start (STEER_PV1_CALL *receive)(struct steer_pv1_datagram *,
        struct steer_pv1_buffer *,uint32_t offset,uint32_t capacity,uint64_t deadline_ns,
        steer_pv1_datagram_done,void *);
    void (STEER_PV1_CALL *close)(struct steer_pv1_datagram *);
    struct steer_pv1_error (STEER_PV1_CALL *release)(struct steer_pv1_datagram *);
};
/* Metadata views are borrowed during callback only. Error => NULL ref/endpoints.
 * Success has a truthful fixed-remote peer endpoint (domain allowed for remote
 * DNS), and optional bound endpoint: NULL means no meaningful local endpoint,
 * never a fabricated address. Native UDP supplies actual numeric endpoints.
 * Original requested destination remains separately owned by routing metadata.
 * UDP open is not a handshake or proof of remote reachability. */
typedef void (STEER_PV1_CALL *steer_pv1_datagram_open_done)(void *,struct steer_pv1_error,
    struct steer_pv1_datagram_ref,const struct steer_pv1_endpoint *bound,
    const struct steer_pv1_endpoint *peer);
struct steer_pv1_datagram_backend_api {
    uint32_t size,version;
    /* Context is an immutable host-selected backend binding, pinned until all
     * returned refs/operations retire. Inputs are copied before PENDING.
     * Open success must be published before the absolute deadline; a locally
     * prepared but late result is closed/released before TIMEOUT callback.
     * This is separate from message IO: an already completed atomic send or
     * consumed receive keeps its result even if callback dispatch is late. */
    struct steer_pv1_async_start (STEER_PV1_CALL *open)(void *context,
        const struct steer_pv1_flow_metadata *,uint64_t deadline_ns,
        steer_pv1_datagram_open_done,void *);
};
#ifdef __cplusplus
}
#endif
#endif
