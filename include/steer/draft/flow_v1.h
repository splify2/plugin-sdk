#ifndef STEER_DRAFT_FLOW_V1_H
#define STEER_DRAFT_FLOW_V1_H
#include "provider_v1.h"
#ifdef __cplusplus
extern "C" {
#endif
/* Host-issued flow identity, never credentials or peer-selected routing authority.
 * Record/spans/endpoints are borrowed only through open/start return. Pending
 * implementations copy metadata before returning. Generation/binding lifetime
 * is pinned separately by the host; copying this record does not retain handles.
 * Inbound/principal IDs are ASCII [A-Za-z0-9_.-], at most 64 bytes. Empty principal
 * ID plus NULL principal means anonymous. A non-NULL principal is opaque and
 * requires an explicitly supported host authentication service, never a cast.
 * STREAM/DATAGRAM require numeric source and nonzero source/destination ports.
 * Destination may be original validated ASCII DNS (<=253 bytes + terminal dot),
 * and remains a domain through policy evaluation. DEVICE packets have a separate
 * port/protocol contract and are not interpreted as these transport flows.
 * Backend selection/detour chain is a separate host-owned binding, not fields
 * supplied by an inbound peer. This remains experimental; no Rust mirror yet. */
struct steer_pv1_flow_metadata {
    uint32_t size, version;
    uint64_t generation;
    struct steer_pv1_span inbound_id, principal_id;
    struct steer_pv1_endpoint source, destination;
    struct steer_pv1_principal *principal;
    uint32_t flow_class;
};
#ifdef __cplusplus
}
#endif
#endif
