/* Experimental source-only device lifecycle companion. Not a frozen ABI;
 * no automatic Rust mirror, provider_v1 layout changes or loader exports. */
#ifndef STEER_DRAFT_DEVICE_V1_H
#define STEER_DRAFT_DEVICE_V1_H
#include "provider_v1.h"
#ifdef __cplusplus
extern "C" {
#endif
struct steer_pv1_device;
struct steer_pv1_device_api;
struct steer_pv1_device_ref {
    struct steer_pv1_device *handle;
    const struct steer_pv1_device_api *api;
};
#define STEER_PV1_DEVICE_RECOVERY_MAX UINT32_C(4096)
#define STEER_PV1_DEVICE_INBOUND UINT32_C(1)
#define STEER_PV1_DEVICE_OUTBOUND UINT32_C(2)
#define STEER_PV1_DEVICE_QUIESCENT UINT32_C(1)
#define STEER_PV1_DEVICE_ACTIVE UINT32_C(2)
#define STEER_PV1_DEVICE_RETIRING UINT32_C(3)
#define STEER_PV1_DEVICE_UNCERTAIN UINT32_C(4)
#define STEER_PV1_DEVICE_RETIRED UINT32_C(5)
#define STEER_PV1_DEVICE_UNCHANGED UINT32_C(0)
#define STEER_PV1_DEVICE_CHANGED UINT32_C(1)
#define STEER_PV1_DEVICE_EFFECT_UNCERTAIN UINT32_C(2)
/* Cached owner-loop facts, not authority to mutate a same-named external
 * resource. Provider freshly verifies boot/namespace/owned kernel identity on
 * every mutation. No keys, kernel descriptors or secret values cross this API.
 * provider_id is borrowed while caller owns the device reference. */
struct steer_pv1_device_info {
    uint32_t size,version;
    uint64_t generation,instance_token;
    struct steer_pv1_span provider_id;
    uint32_t roles,phase;
    /* Nonsecret provider record, <=RECOVERY_MAX; host copies before the next
     * device API call. Common journal stores this opaque record with its typed
     * provider/version/plan/generation/transaction identity BEFORE commit.
     * Provider defines and validates its record format/version and stable intent
     * identity. Record bytes contain no pointers, descriptors or key material;
     * coordinator persists exact bytes and never logs them by default. They
     * include planned unique kernel markers and boot/namespace identity.
     * Creator must atomically attach the marker when creating each resource;
     * later observed indices/handles are diagnostic, not name-only authority.
     * A completion may update the record for the next journal progress entry. */
    struct steer_pv1_span recovery_record;
};
typedef void (STEER_PV1_CALL *steer_pv1_device_done)(void *,
    struct steer_pv1_error,uint32_t effect,uint32_t phase);
/* Origin-owned reference, never a stream/datagram cast. Host-selected module,
 * prepared inputs, namespace identity and journal/dependency guards remain
 * pinned through all operations and foreign callback returns.
 *
 * All starts, notifications and info/release calls serialize on the owner.
 * Blocking file/netlink/tool work uses a bounded host worker, not this loop or
 * a thread per device. Exactly one operation may be pending, including a queued
 * completion. PENDING yields one deferred callback and an origin operation
 * reference, released only AFTER that callback returns. REJECTED has no callback
 * and makes no new kernel effects.
 *
 * transaction_id is nonzero, minted by the common host transaction
 * coordinator BEFORE writing durable intent, and authorized/passed to provider
 * operations only AFTER the exact intent is durably recorded. intent_sequence
 * is the exact nonzero durable record key, never a latest-tuple lookup. It is not a credential or
 * proof by itself. Providers do not independently mutate user config files.
 * deadline_ns is absolute host monotonic, mandatory and future on start.
 * Cancel/expiry before effects returns UNCHANGED. Once a mutation committed,
 * result reports factual CHANGED or EFFECT_UNCERTAIN even on error, cancellation
 * or late completion. Never synthesize an unchanged rollback from response loss.
 *
 * commit starts from QUIESCENT and may perform fallible activation/policy work.
 * Only known matching state permits the coordinator's later memory-only
 * publication. UNCERTAIN retains owned partial state for explicit reconciliation;
 * neither automatic adoption nor blind repeated mutation is allowed.
 * Allowed state transitions:
 * - Bind makes no NEW external mutation and supplies QUIESCENT: either newly staged resources or an already owned
 *   instance quiesced for resume. In both cases admission is stopped; cached
 *   phase alone never authorizes ownership or implies that no resource exists.
 * - commit: QUIESCENT -> ACTIVE on known matching activation. Failure may leave
 *   known QUIESCENT or report UNCERTAIN. Other start phases are rejected.
 * - quiesce: ACTIVE -> QUIESCENT; already QUIESCENT is an idempotent unchanged
 *   operation. It stops admission before guards may be removed.
 * - retire: QUIESCENT/ACTIVE/RETIRING/UNCERTAIN -> RETIRED when all owned effects
 *   are absent. Partial known cleanup may leave RETIRING. Unknown result leaves
 *   UNCERTAIN. Already RETIRED is idempotent and makes no new kernel mutation.
 * - release consumes only RETIRED. No transition directly consumes UNCERTAIN.
 *
 * Recovery is explicitly retire-from-UNCERTAIN: first reconcile fresh kernel
 * observations against the durable journal's boot/namespace and owned resource
 * marker/identity; only then perform guarded cleanup of verified owned effects.
 * Failure to prove ownership leaves it retained and never deletes a foreign
 * replacement. There is no info/sidechannel bypass: info remains cached.
 * Restart factories reconstruct handles read-only from journal-proven identity,
 * never by name/index/kind alone. An intent alone does NOT prove QUIESCENT:
 * after a possibly lost commit result, reconstructed phase is UNCERTAIN until
 * fresh observation/recovery proves otherwise. Such a recovery instance cannot
 * enter normal commit/publication until reconciled. Repeated recovery is
 * idempotent even after a
 * lost callback following kernel commit. No automatic repeated activation.
 *
 * effect describes THIS operation, separately from cached phase. Thus a
 * read-only recovery may observe RETIRED with UNCHANGED, while a rejected
 * recovery can retain UNCERTAIN with UNCHANGED. An indeterminate new mutation
 * reports EFFECT_UNCERTAIN/UNCERTAIN with nonzero error. CHANGED never implies
 * config durability or permission to report rollback.
 */
struct steer_pv1_device_api {
    uint32_t size,version;
    struct steer_pv1_error (STEER_PV1_CALL *info)(struct steer_pv1_device *,struct steer_pv1_device_info *);
    struct steer_pv1_async_start (STEER_PV1_CALL *commit)(struct steer_pv1_device *,uint64_t transaction_id,uint32_t intent_sequence,
        uint64_t deadline_ns,steer_pv1_device_done,void *);
    struct steer_pv1_async_start (STEER_PV1_CALL *quiesce)(struct steer_pv1_device *,uint64_t transaction_id,uint32_t intent_sequence,
        uint64_t deadline_ns,steer_pv1_device_done,void *);
    struct steer_pv1_async_start (STEER_PV1_CALL *retire)(struct steer_pv1_device *,uint64_t transaction_id,uint32_t intent_sequence,
        uint64_t deadline_ns,steer_pv1_device_done,void *);
    /* Success consumes only RETIRED device with all ops released/callbacks
     * returned and no uncertain kernel ownership. Failure retains reference.
     * No kernel IO or fresh mutation is performed by release. */
    struct steer_pv1_error (STEER_PV1_CALL *release)(struct steer_pv1_device *);
};
#ifdef __cplusplus
}
#endif
#endif
