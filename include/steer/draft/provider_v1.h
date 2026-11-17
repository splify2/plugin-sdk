/* DRAFT ONLY: foundation of docs/provider-v1-draft.md, not a published ABI.
 * No production runtime implements this contract. Do not declare stable compatibility from
 * this header. Coverage and unimplemented services: provider-v1-header-coverage.md.
 * No legacy plugin.h/net1 layout or exported symbol is replaced here. */
#ifndef STEER_DRAFT_PROVIDER_V1_H
#define STEER_DRAFT_PROVIDER_V1_H

#include <stddef.h>
#include <stdint.h>

#if defined(_WIN32)
#define STEER_PV1_CALL __cdecl
#else
#define STEER_PV1_CALL
#endif

#ifdef __cplusplus
extern "C" {
#endif

/* Layout revision is draft-local, not an advertised steer-provider-api version.
 * Every table begins with uint32_t size, version. No packing is permitted.
 * Read only fields within size; unknown layout/required capability fails closed.
 * Caller has checked the prefix before using STEER_PV1_HAS. */
#define STEER_PV1_DRAFT_LAYOUT UINT32_C(1)
#define STEER_PV1_HAS(table, type, field) \
    ((table) != NULL && (table)->size >= \
     offsetof(type, field) + sizeof(((type *)0)->field))
#define STEER_PV1_ENTRY_NAME "steer_provider_query_v1"

struct steer_pv1_host;
struct steer_pv1_instance;
struct steer_pv1_operation;
struct steer_pv1_buffer;
struct steer_pv1_secret;
struct steer_pv1_stream;
struct steer_pv1_datagram;
struct steer_pv1_listener;
struct steer_pv1_principal;
struct steer_pv1_route_context;

/* Unless explicitly stated otherwise (descriptor storage, buffer views),
 * borrowed input spans expire on return from the call/callback receiving them.
 * len is bytes, not characters. NULL data is valid only when len == 0.
 * No span is NUL terminated by implication. */
struct steer_pv1_span {
    const uint8_t *data;
    uint32_t len;
};
struct steer_pv1_mut_span {
    uint8_t *data;
    uint32_t len;
};
/* Unversioned value records (span, mut_span, error, operation_ref, async_start)
 * have frozen layouts within this draft layout. Extending them requires a new
 * containing table layout, never appending silently inside an outer record. */

/* Tags are uint32_t on the ABI, never C enums or bool. Values must not be reused.
 * These draft-local values are subject to review before ABI freeze. */
#define STEER_PV1_OK                 UINT32_C(0)
#define STEER_PV1_UNSUPPORTED        UINT32_C(1)
#define STEER_PV1_INVALID            UINT32_C(2)
#define STEER_PV1_AUTH_FAILED        UINT32_C(3)
#define STEER_PV1_POLICY_DENIED      UINT32_C(4)
#define STEER_PV1_RESOURCE_EXHAUSTED UINT32_C(5)
#define STEER_PV1_TIMEOUT            UINT32_C(6)
#define STEER_PV1_CANCELLED          UINT32_C(7)
#define STEER_PV1_IO                 UINT32_C(8)
#define STEER_PV1_PROTOCOL           UINT32_C(9)
#define STEER_PV1_INTERNAL           UINT32_C(10)

struct steer_pv1_error {
    uint32_t code;
    uint32_t transient; /* exactly 0 or 1; OK requires 0 */
};

#define STEER_PV1_REJECTED UINT32_C(0)
#define STEER_PV1_PENDING  UINT32_C(1)

struct steer_pv1_operation_api {
    uint32_t size;
    uint32_t version;
    /* Caller invokes on the initiating shard. Idempotent until release;
     * cancellation need not win over success. Never triggers inline completion. */
    void (STEER_PV1_CALL *cancel)(struct steer_pv1_operation *);
    /* Caller owns one reference; release exactly once AFTER terminal callback
     * returns. No early release or subsequent access/cancel is permitted. */
    void (STEER_PV1_CALL *release)(struct steer_pv1_operation *);
};
struct steer_pv1_operation_ref {
    /* Origin-owned pair: use ONLY the accompanying api for this handle. A
     * provider may wrap a host operation; the host must not cast or submit its
     * handle to an unrelated operation service. Table stays live until release. */
    struct steer_pv1_operation *handle;
    const struct steer_pv1_operation_api *api;
};
struct steer_pv1_async_start {
    uint32_t disposition;
    struct steer_pv1_error error;
    struct steer_pv1_operation_ref operation;
};

/* Async contract for every function returning async_start:
 * REJECTED: non-OK error, NULL handle/api, no callback, no ownership transfer.
 * PENDING: OK error, non-NULL handle/api, exactly one deferred callback, even
 * after cancellation. Success, error and cancelled are mutually exclusive.
 * Callback + userdata remain valid until completion returns. The provider's
 * code/table stays loaded until operation release and instance destruction.
 * Callbacks run serialized on the initiating shard; never inline, never after
 * the corresponding instance is destroyed. No exception/panic crosses FFI.
 * Metadata/config arguments are borrowed on start only; async implementations
 * copy them or retain explicitly permitted handles before returning PENDING. */

#define STEER_PV1_DIRECTION_INBOUND  UINT32_C(1)
#define STEER_PV1_DIRECTION_OUTBOUND UINT32_C(2)
#define STEER_PV1_FLOW_STREAM        UINT32_C(1)
#define STEER_PV1_FLOW_DATAGRAM      UINT32_C(2)
#define STEER_PV1_FLOW_DEVICE        UINT32_C(3)
#define STEER_PV1_MODE_TRUSTED_NATIVE UINT32_C(1)

struct steer_pv1_target {
    uint32_t size;
    uint32_t version;
    struct steer_pv1_span os;
    struct steer_pv1_span cpu;
    struct steer_pv1_span abi;
    struct steer_pv1_span libc; /* explicit value, including none for no libc */
    uint32_t pointer_bits;
    uint32_t endian; /* 1 little, 2 big; no host-native shorthand */
};
/* A tuple is one supported combination, not independent lists whose cartesian
 * product the host may assume. Transport/security/provider IDs are ASCII. */
struct steer_pv1_combination {
    uint32_t size;
    uint32_t version;
    uint32_t direction;
    uint32_t flow_class;
    struct steer_pv1_span transport;
    struct steer_pv1_span security;
};
struct steer_pv1_descriptor {
    uint32_t size;
    uint32_t version;
    struct steer_pv1_span package_id;
    struct steer_pv1_span package_version;
    struct steer_pv1_span provider_id;
    struct steer_pv1_span api_range;
    const struct steer_pv1_target *target; /* independently size/version checked */
    /* Pointer arrays allow tail extension of records without changing stride.
     * All descriptor pointers/spans have module lifetime, immutable. */
    const struct steer_pv1_combination *const *combinations;
    uint32_t combination_count;
    const uint32_t *execution_modes;
    uint32_t execution_mode_count;
    const uint32_t *required_host_capabilities;
    uint32_t required_host_capability_count;
    uint8_t options_schema_sha256[32];
};

/* Endpoint tag. Exactly 4/16 bytes for IP; domain is bounded UTF-8 bytes
 * subject to host IDNA validation. No sockaddr/fd/scope_id crosses this ABI.
 * Scoped IPv6 must return unsupported until a scope contract is negotiated. */
#define STEER_PV1_ENDPOINT_IPV4   UINT32_C(1)
#define STEER_PV1_ENDPOINT_IPV6   UINT32_C(2)
#define STEER_PV1_ENDPOINT_DOMAIN UINT32_C(3)
struct steer_pv1_endpoint {
    uint32_t size;
    uint32_t version;
    uint32_t tag;
    struct steer_pv1_span address;
    uint16_t port; /* host byte order; address IP bytes are network order */
    uint16_t reserved; /* zero; not future caller-owned flags */
};

/* Only the buffer service is declared in this foundation. No transport/TLS/
 * auth/device service is implied by opaque handles or a descriptor tuple. */
#define STEER_PV1_TABLE_BUFFERS UINT32_C(1)
struct steer_pv1_buffer_api {
    uint32_t size;
    uint32_t version;
    /* OK gives caller one host-owned reference. On error out is NULL.
     * Capacity bounded by host policy; no implicit overcommit/unbounded queue. */
    struct steer_pv1_error (STEER_PV1_CALL *allocate)(
        struct steer_pv1_host *, uint32_t capacity, struct steer_pv1_buffer **out);
    /* OK adds one reference. Error leaves ownership unchanged; counter overflow
     * returns RESOURCE_EXHAUSTED. A failed retain never permits async handoff. */
    struct steer_pv1_error (STEER_PV1_CALL *retain)(
        struct steer_pv1_host *, struct steer_pv1_buffer *);
    void (STEER_PV1_CALL *release)(struct steer_pv1_host *, struct steer_pv1_buffer *);
    /* Borrowed writable capacity, only while caller owns the sole reference.
     * Not available for secrets. Never mutate once shared or passed to async IO.
     * On OK view survives return, valid until retain/share/release, IO submission,
     * or the next writable/mutating service call for this buffer. Caller must
     * stop using it BEFORE any such operation.
     * On error out is {NULL,0}. The view itself owns no buffer reference. */
    struct steer_pv1_error (STEER_PV1_CALL *writable)(
        struct steer_pv1_host *, struct steer_pv1_buffer *, struct steer_pv1_mut_span *out);
};
struct steer_pv1_host_api {
    uint32_t size;
    uint32_t version;
    uint32_t api_major;
    uint32_t api_minor;
    /* Host tables have host lifetime. Unknown table/version => unsupported,
     * *out=NULL. Caller checks returned size/version before dereferencing. */
    struct steer_pv1_error (STEER_PV1_CALL *get_table)(
        struct steer_pv1_host *, uint32_t table_id, uint32_t layout_version,
        const void **out);
    uint64_t (STEER_PV1_CALL *monotonic_ns)(struct steer_pv1_host *);
    /* Message is borrowed on call; must not contain secrets or credentials.
     * severity 3 error, 4 warning, 6 info, 7 debug. Host applies log bounds. */
    void (STEER_PV1_CALL *log)(struct steer_pv1_host *, uint32_t severity,
                             struct steer_pv1_span message);
};

struct steer_pv1_config {
    uint32_t size;
    uint32_t version;
    uint64_t generation;
    struct steer_pv1_span instance_id;
    uint32_t direction;
    uint32_t flow_class;
    struct steer_pv1_span options_json; /* one schema-validated object, no secrets */
    uint64_t drain_deadline_ns; /* monotonic host clock */
};

typedef void (STEER_PV1_CALL *steer_pv1_prepare_done)(
    void *userdata, struct steer_pv1_error,
    struct steer_pv1_instance *instance);
typedef void (STEER_PV1_CALL *steer_pv1_drain_done)(
    void *userdata, struct steer_pv1_error);

struct steer_pv1_provider_api {
    uint32_t size;
    uint32_t version;
    const struct steer_pv1_descriptor *descriptor;
    /* Pure synchronous check: no IO, listeners, worker threads or mutations. */
    struct steer_pv1_error (STEER_PV1_CALL *validate)(
        struct steer_pv1_host *, const struct steer_pv1_config *);
    /* Callback OK transfers one instance to host; error has NULL instance and
     * provider already unwound partial allocations in reverse order.
     * Host retains host_api/context until instance destroyed and op released. */
    struct steer_pv1_async_start (STEER_PV1_CALL *prepare)(
        struct steer_pv1_host *, const struct steer_pv1_host_api *,
        const struct steer_pv1_config *, steer_pv1_prepare_done, void *userdata);
    /* Host publishes generation/listeners only after all prepare steps succeed.
     * activate may fail; host must quiesce/drain/destroy any activated instances.
     * This call must not perform irreversible network/device mutations. */
    struct steer_pv1_error (STEER_PV1_CALL *activate)(struct steer_pv1_instance *);
    /* Stops accepting new flows; existing flows remain. Idempotent and sync. */
    void (STEER_PV1_CALL *quiesce)(struct steer_pv1_instance *);
    /* Begins after quiesce. Complete as soon as the barrier is reached;
     * deadline is the cancellation cutoff, never a mandatory waiting period.
     * Completion means all flow/operation callbacks and retained data reached barrier.
     * The drain operation itself is excluded from that barrier and must be
     * released after callback returns, BEFORE destroy. Deadline cancels flows;
     * it never permits destroy while a callback can still arrive. REJECTED is
     * not a barrier: host must retain instance and retry/recover explicitly. */
    struct steer_pv1_async_start (STEER_PV1_CALL *drain)(
        struct steer_pv1_instance *, uint64_t deadline_ns,
        steer_pv1_drain_done, void *userdata);
    /* Exactly once, on owning shard, after drain barrier and operation releases;
     * also valid for prepared-but-never-activated instance after quiesce/drain.
     * Consumes instance. No callbacks are possible afterwards. */
    void (STEER_PV1_CALL *destroy)(struct steer_pv1_instance *);
};

/* Entrypoint is pure: no listener/socket creation, background work or loading
 * fallback. Caller checks signed schema2 identity/target/axes BEFORE loading
 * native code. query validates host prefix/layout/major and required functions.
 * On OK *out points to immutable module-lifetime table; on error *out=NULL.
 * Export visibility is a future packaging responsibility; this declaration
 * deliberately does not mark consumer imports as exported on Windows. */
typedef struct steer_pv1_error (STEER_PV1_CALL *steer_pv1_query_fn)(
    struct steer_pv1_host *, const struct steer_pv1_host_api *,
    const struct steer_pv1_provider_api **out);
struct steer_pv1_error STEER_PV1_CALL steer_provider_query_v1(
    struct steer_pv1_host *, const struct steer_pv1_host_api *,
    const struct steer_pv1_provider_api **out);

#ifdef __cplusplus
}
#endif
#endif
