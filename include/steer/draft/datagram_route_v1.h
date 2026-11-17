/* Experimental host routing companion; not a frozen ABI, no Rust mirror. */
#ifndef STEER_DRAFT_DATAGRAM_ROUTE_V1_H
#define STEER_DRAFT_DATAGRAM_ROUTE_V1_H
#include "datagram_v1.h"
#ifdef __cplusplus
extern "C" {
#endif
struct steer_pv1_datagram_association;
struct steer_pv1_datagram_route_api;
struct steer_pv1_datagram_association_ref {
    struct steer_pv1_datagram_association *handle;
    const struct steer_pv1_datagram_route_api *api;
};
/* One context per ACTUAL protocol UDP session/association lifetime. Identical
 * principal/source metadata never merges two contexts or their cached replies.
 * create copies host-trusted identity/source/generation; initial destination is
 * validated metadata, never implicit route permission. Each open_target applies
 * policy to that requested destination before DNS/backend work.
 * A target is the requested destination plus its host-selected backend binding.
 * Each new open re-evaluates selection; an exported ref retains its origin.
 * One active returned ref or pending open per target in a context; reuse that
 * ref for message IO. A duplicate open is rejected with bounded backpressure.
 * Inactive cached origins may retain UDP port identity until idle/budget eviction.
 * Active refs and completed-unreleased operations prevent eviction/free.
 * quiesce stops new target opens while returned refs/IO drain. close additionally
 * closes returned refs/cancels operations, but cannot release caller-owned refs.
 * release requires close plus all child refs and operation releases, then
 * consumes the public handle. Private cleanup retains independent generation
 * ownership and bounded admission until its provider barriers complete.
 * Success is published only before the absolute deadline, including cached
 * opens; timer/fd ordering cannot publish a late success. All APIs
 * are serialized on the owning shard, with origin-owned operation/ref pairs. */
struct steer_pv1_datagram_route_api {
    uint32_t size,version;
    struct steer_pv1_error (STEER_PV1_CALL *create)(void *host_selected_context,
        const struct steer_pv1_flow_metadata *,struct steer_pv1_datagram_association_ref *);
    struct steer_pv1_async_start (STEER_PV1_CALL *open_target)(struct steer_pv1_datagram_association *,
        const struct steer_pv1_endpoint *,uint64_t deadline_ns,steer_pv1_datagram_open_done,void *);
    void (STEER_PV1_CALL *quiesce)(struct steer_pv1_datagram_association *);
    void (STEER_PV1_CALL *close)(struct steer_pv1_datagram_association *);
    struct steer_pv1_error (STEER_PV1_CALL *release)(struct steer_pv1_datagram_association *);
};
#ifdef __cplusplus
}
#endif
#endif
