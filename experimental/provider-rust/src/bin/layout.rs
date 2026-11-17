//! Independent Rust layout emitter. No external provider symbol is referenced.
use core::ffi::c_void;
use core::mem::{align_of, offset_of, size_of};
use steer_provider_draft_ffi::*;

macro_rules! value_type {
    ($name:literal, $t:ty) => {
        println!(concat!($name, ".size={}"), size_of::<$t>());
        println!(concat!($name, ".align={}"), align_of::<$t>());
    };
}
macro_rules! record {
    ($t:ident, $($field:ident),+ $(,)?) => {
        println!(concat!(stringify!($t), ".size={}"), size_of::<$t>());
        println!(concat!(stringify!($t), ".align={}"), align_of::<$t>());
        $(println!(concat!(stringify!($t), ".offset.", stringify!($field), "={}"),
            offset_of!($t, $field));)+
    };
}
macro_rules! constants {
    ($($name:ident),+ $(,)?) => {$(
        println!(concat!("constant.", stringify!($name), "={}"), $name);
    )+};
}

fn main() {
    constants!(STEER_PV1_DRAFT_LAYOUT, STEER_PV1_OK, STEER_PV1_UNSUPPORTED,
        STEER_PV1_INVALID, STEER_PV1_AUTH_FAILED, STEER_PV1_POLICY_DENIED,
        STEER_PV1_RESOURCE_EXHAUSTED, STEER_PV1_TIMEOUT, STEER_PV1_CANCELLED,
        STEER_PV1_IO, STEER_PV1_PROTOCOL, STEER_PV1_INTERNAL, STEER_PV1_REJECTED,
        STEER_PV1_PENDING, STEER_PV1_DIRECTION_INBOUND, STEER_PV1_DIRECTION_OUTBOUND,
        STEER_PV1_FLOW_STREAM, STEER_PV1_FLOW_DATAGRAM, STEER_PV1_FLOW_DEVICE,
        STEER_PV1_MODE_TRUSTED_NATIVE, STEER_PV1_ENDPOINT_IPV4,
        STEER_PV1_ENDPOINT_IPV6, STEER_PV1_ENDPOINT_DOMAIN, STEER_PV1_TABLE_BUFFERS);
    value_type!("uint8_t", u8);
    value_type!("uint16_t", u16);
    value_type!("uint32_t", u32);
    value_type!("uint64_t", u64);
    value_type!("void_pointer", *mut c_void);
    value_type!("steer_pv1_prepare_done", steer_pv1_prepare_done);
    value_type!("steer_pv1_drain_done", steer_pv1_drain_done);
    value_type!("steer_pv1_query_fn", steer_pv1_query_fn);

    record!(steer_pv1_span, data, len);
    record!(steer_pv1_mut_span, data, len);
    record!(steer_pv1_error, code, transient);
    record!(steer_pv1_operation_api, size, version, cancel, release);
    record!(steer_pv1_operation_ref, handle, api);
    record!(steer_pv1_async_start, disposition, error, operation);
    record!(steer_pv1_target, size, version, os, cpu, abi, libc, pointer_bits, endian);
    record!(steer_pv1_combination, size, version, direction, flow_class, transport, security);
    record!(steer_pv1_descriptor, size, version, package_id, package_version,
        provider_id, api_range, target, combinations, combination_count,
        execution_modes, execution_mode_count, required_host_capabilities,
        required_host_capability_count, options_schema_sha256);
    record!(steer_pv1_endpoint, size, version, tag, address, port, reserved);
    record!(steer_pv1_buffer_api, size, version, allocate, retain, release, writable);
    record!(steer_pv1_host_api, size, version, api_major, api_minor,
        get_table, monotonic_ns, log);
    record!(steer_pv1_config, size, version, generation, instance_id, direction,
        flow_class, options_json, drain_deadline_ns);
    record!(steer_pv1_provider_api, size, version, descriptor, validate, prepare,
        activate, quiesce, drain, destroy);
}
