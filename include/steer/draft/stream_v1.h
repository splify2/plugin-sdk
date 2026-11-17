/* Experimental Linux host transport extension. Not a stable ABI; no Rust mirror. */
#ifndef STEER_DRAFT_STREAM_V1_H
#define STEER_DRAFT_STREAM_V1_H
#include "provider_v1.h"
#ifdef __cplusplus
extern "C" {
#endif
typedef void (STEER_PV1_CALL *steer_pv1_stream_done)(void *,
    struct steer_pv1_error, struct steer_pv1_buffer *, uint32_t bytes);
/* Same owning shard as provider_v1. PENDING transfers the buffer's sole
 * reference to host until terminal callback entry; REJECTED transfers nothing.
 * Caller must discard writable views before submission and must not access,
 * resubmit, retain, request writable or release the buffer while pending.
 * Callback returns only that completed operation's reference,
 * including on error/cancel. Offset/length must fit capacity; length > 0.
 * One pending read and write, including queued terminal callback, per stream.
 * Successful read bytes==0 is EOF; write may succeed partially. Error bytes=0.
 * Cancel detaches IO before deferred completion; a queued success may win.
 * close stops IO before returning, queues cancellation, and is idempotent.
 * Stream and operation references are independent; release operations only
 * after callbacks return. Stream release requires close and all op releases.
 * No listener, TLS, policy dial, principal or provider discovery is implied. */
struct steer_pv1_stream_api {
    uint32_t size, version;
    struct steer_pv1_async_start (STEER_PV1_CALL *read)(struct steer_pv1_stream *,
        struct steer_pv1_buffer *, uint32_t offset, uint32_t length,
        steer_pv1_stream_done, void *);
    struct steer_pv1_async_start (STEER_PV1_CALL *write)(struct steer_pv1_stream *,
        struct steer_pv1_buffer *, uint32_t offset, uint32_t length,
        steer_pv1_stream_done, void *);
    /* Pending write => POLICY_DENIED. Read EOF does not shut down write. */
    struct steer_pv1_error (STEER_PV1_CALL *shutdown_write)(struct steer_pv1_stream *);
    void (STEER_PV1_CALL *close)(struct steer_pv1_stream *);
    struct steer_pv1_error (STEER_PV1_CALL *release)(struct steer_pv1_stream *);
};
#ifdef __cplusplus
}
#endif
#endif
