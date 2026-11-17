/* Independent C11 layout emitter; no provider code is called or linked.
 * Build only when explicitly authorized, against the same target as Rust. */
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include "../../include/steer/draft/provider_v1.h"

#define VALUE_TYPE(name, type) do { \
    printf(name ".size=%zu\n", sizeof(type)); \
    printf(name ".align=%zu\n", _Alignof(type)); \
} while (0)
#define RECORD(type) VALUE_TYPE(#type, struct type)
/* 'size' is also a real field: offsets have a distinct namespace. */
#define FIELD(type, field) \
    printf(#type ".offset." #field "=%zu\n", offsetof(struct type, field))
#define CONSTANT(name) printf("constant." #name "=%lu\n", (unsigned long)(name))

int main(void) {
    CONSTANT(STEER_PV1_DRAFT_LAYOUT);
    CONSTANT(STEER_PV1_OK); CONSTANT(STEER_PV1_UNSUPPORTED);
    CONSTANT(STEER_PV1_INVALID); CONSTANT(STEER_PV1_AUTH_FAILED);
    CONSTANT(STEER_PV1_POLICY_DENIED); CONSTANT(STEER_PV1_RESOURCE_EXHAUSTED);
    CONSTANT(STEER_PV1_TIMEOUT); CONSTANT(STEER_PV1_CANCELLED);
    CONSTANT(STEER_PV1_IO); CONSTANT(STEER_PV1_PROTOCOL); CONSTANT(STEER_PV1_INTERNAL);
    CONSTANT(STEER_PV1_REJECTED); CONSTANT(STEER_PV1_PENDING);
    CONSTANT(STEER_PV1_DIRECTION_INBOUND); CONSTANT(STEER_PV1_DIRECTION_OUTBOUND);
    CONSTANT(STEER_PV1_FLOW_STREAM); CONSTANT(STEER_PV1_FLOW_DATAGRAM);
    CONSTANT(STEER_PV1_FLOW_DEVICE); CONSTANT(STEER_PV1_MODE_TRUSTED_NATIVE);
    CONSTANT(STEER_PV1_ENDPOINT_IPV4); CONSTANT(STEER_PV1_ENDPOINT_IPV6);
    CONSTANT(STEER_PV1_ENDPOINT_DOMAIN); CONSTANT(STEER_PV1_TABLE_BUFFERS);
    VALUE_TYPE("uint8_t", uint8_t);
    VALUE_TYPE("uint16_t", uint16_t);
    VALUE_TYPE("uint32_t", uint32_t);
    VALUE_TYPE("uint64_t", uint64_t);
    VALUE_TYPE("void_pointer", void *);
    VALUE_TYPE("steer_pv1_prepare_done", steer_pv1_prepare_done);
    VALUE_TYPE("steer_pv1_drain_done", steer_pv1_drain_done);
    VALUE_TYPE("steer_pv1_query_fn", steer_pv1_query_fn);

    RECORD(steer_pv1_span);
    FIELD(steer_pv1_span, data); FIELD(steer_pv1_span, len);
    RECORD(steer_pv1_mut_span);
    FIELD(steer_pv1_mut_span, data); FIELD(steer_pv1_mut_span, len);
    RECORD(steer_pv1_error);
    FIELD(steer_pv1_error, code); FIELD(steer_pv1_error, transient);
    RECORD(steer_pv1_operation_api);
    FIELD(steer_pv1_operation_api, size); FIELD(steer_pv1_operation_api, version);
    FIELD(steer_pv1_operation_api, cancel); FIELD(steer_pv1_operation_api, release);
    RECORD(steer_pv1_operation_ref);
    FIELD(steer_pv1_operation_ref, handle); FIELD(steer_pv1_operation_ref, api);
    RECORD(steer_pv1_async_start);
    FIELD(steer_pv1_async_start, disposition); FIELD(steer_pv1_async_start, error);
    FIELD(steer_pv1_async_start, operation);
    RECORD(steer_pv1_target);
    FIELD(steer_pv1_target, size); FIELD(steer_pv1_target, version);
    FIELD(steer_pv1_target, os); FIELD(steer_pv1_target, cpu);
    FIELD(steer_pv1_target, abi); FIELD(steer_pv1_target, libc);
    FIELD(steer_pv1_target, pointer_bits); FIELD(steer_pv1_target, endian);
    RECORD(steer_pv1_combination);
    FIELD(steer_pv1_combination, size); FIELD(steer_pv1_combination, version);
    FIELD(steer_pv1_combination, direction); FIELD(steer_pv1_combination, flow_class);
    FIELD(steer_pv1_combination, transport); FIELD(steer_pv1_combination, security);
    RECORD(steer_pv1_descriptor);
    FIELD(steer_pv1_descriptor, size); FIELD(steer_pv1_descriptor, version);
    FIELD(steer_pv1_descriptor, package_id); FIELD(steer_pv1_descriptor, package_version);
    FIELD(steer_pv1_descriptor, provider_id); FIELD(steer_pv1_descriptor, api_range);
    FIELD(steer_pv1_descriptor, target); FIELD(steer_pv1_descriptor, combinations);
    FIELD(steer_pv1_descriptor, combination_count); FIELD(steer_pv1_descriptor, execution_modes);
    FIELD(steer_pv1_descriptor, execution_mode_count);
    FIELD(steer_pv1_descriptor, required_host_capabilities);
    FIELD(steer_pv1_descriptor, required_host_capability_count);
    FIELD(steer_pv1_descriptor, options_schema_sha256);
    RECORD(steer_pv1_endpoint);
    FIELD(steer_pv1_endpoint, size); FIELD(steer_pv1_endpoint, version);
    FIELD(steer_pv1_endpoint, tag); FIELD(steer_pv1_endpoint, address);
    FIELD(steer_pv1_endpoint, port); FIELD(steer_pv1_endpoint, reserved);
    RECORD(steer_pv1_buffer_api);
    FIELD(steer_pv1_buffer_api, size); FIELD(steer_pv1_buffer_api, version);
    FIELD(steer_pv1_buffer_api, allocate); FIELD(steer_pv1_buffer_api, retain);
    FIELD(steer_pv1_buffer_api, release); FIELD(steer_pv1_buffer_api, writable);
    RECORD(steer_pv1_host_api);
    FIELD(steer_pv1_host_api, size); FIELD(steer_pv1_host_api, version);
    FIELD(steer_pv1_host_api, api_major); FIELD(steer_pv1_host_api, api_minor);
    FIELD(steer_pv1_host_api, get_table); FIELD(steer_pv1_host_api, monotonic_ns);
    FIELD(steer_pv1_host_api, log);
    RECORD(steer_pv1_config);
    FIELD(steer_pv1_config, size); FIELD(steer_pv1_config, version);
    FIELD(steer_pv1_config, generation); FIELD(steer_pv1_config, instance_id);
    FIELD(steer_pv1_config, direction); FIELD(steer_pv1_config, flow_class);
    FIELD(steer_pv1_config, options_json); FIELD(steer_pv1_config, drain_deadline_ns);
    RECORD(steer_pv1_provider_api);
    FIELD(steer_pv1_provider_api, size); FIELD(steer_pv1_provider_api, version);
    FIELD(steer_pv1_provider_api, descriptor); FIELD(steer_pv1_provider_api, validate);
    FIELD(steer_pv1_provider_api, prepare); FIELD(steer_pv1_provider_api, activate);
    FIELD(steer_pv1_provider_api, quiesce); FIELD(steer_pv1_provider_api, drain);
    FIELD(steer_pv1_provider_api, destroy);
    return ferror(stdout) ? 1 : 0;
}
