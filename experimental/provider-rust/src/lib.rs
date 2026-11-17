//! Draft declarations only. No host, provider implementation, safe wrapper or
//! stable API is supplied. Semantic authority: include/steer/draft/provider_v1.h.
//! Borrowing, shard affinity, barriers and module lifetime remain caller duties.
#![allow(non_camel_case_types)]
#![no_std]

use core::ffi::c_void;
use core::marker::{PhantomData, PhantomPinned};

pub const STEER_PV1_DRAFT_LAYOUT: u32 = 1;
pub const STEER_PV1_ENTRY_NAME: &[u8] = b"steer_provider_query_v1\0";

// Only pointers to these types cross FFI. Private fields prevent construction;
// zero-sized layout is NOT the layout of the host's opaque allocation.
macro_rules! opaque {
    ($($name:ident),+ $(,)?) => {$(
        #[repr(C)]
        pub struct $name {
            _private: [u8; 0],
            _marker: PhantomData<(*mut u8, PhantomPinned)>,
        }
    )+};
}
opaque!(steer_pv1_host, steer_pv1_instance, steer_pv1_operation,
    steer_pv1_buffer, steer_pv1_secret, steer_pv1_stream, steer_pv1_datagram,
    steer_pv1_listener, steer_pv1_principal, steer_pv1_route_context);

#[repr(C)]
#[derive(Clone, Copy)]
pub struct steer_pv1_span { pub data: *const u8, pub len: u32 }
#[repr(C)]
#[derive(Clone, Copy)]
pub struct steer_pv1_mut_span { pub data: *mut u8, pub len: u32 }

pub const STEER_PV1_OK: u32 = 0;
pub const STEER_PV1_UNSUPPORTED: u32 = 1;
pub const STEER_PV1_INVALID: u32 = 2;
pub const STEER_PV1_AUTH_FAILED: u32 = 3;
pub const STEER_PV1_POLICY_DENIED: u32 = 4;
pub const STEER_PV1_RESOURCE_EXHAUSTED: u32 = 5;
pub const STEER_PV1_TIMEOUT: u32 = 6;
pub const STEER_PV1_CANCELLED: u32 = 7;
pub const STEER_PV1_IO: u32 = 8;
pub const STEER_PV1_PROTOCOL: u32 = 9;
pub const STEER_PV1_INTERNAL: u32 = 10;

#[repr(C)]
#[derive(Clone, Copy)]
pub struct steer_pv1_error { pub code: u32, pub transient: u32 }
pub const STEER_PV1_REJECTED: u32 = 0;
pub const STEER_PV1_PENDING: u32 = 1;

// extern "C" is the C ABI including Windows __cdecl, NOT extern "system".
// Option uses Rust's nullable C function-pointer representation. A required
// slot/callback still must be checked for Some by the consuming host/provider.
#[repr(C)]
pub struct steer_pv1_operation_api {
    pub size: u32,
    pub version: u32,
    pub cancel: Option<unsafe extern "C" fn(*mut steer_pv1_operation)>,
    pub release: Option<unsafe extern "C" fn(*mut steer_pv1_operation)>,
}
#[repr(C)]
pub struct steer_pv1_operation_ref {
    pub handle: *mut steer_pv1_operation,
    pub api: *const steer_pv1_operation_api,
}
#[repr(C)]
pub struct steer_pv1_async_start {
    pub disposition: u32,
    pub error: steer_pv1_error,
    pub operation: steer_pv1_operation_ref,
}

pub const STEER_PV1_DIRECTION_INBOUND: u32 = 1;
pub const STEER_PV1_DIRECTION_OUTBOUND: u32 = 2;
pub const STEER_PV1_FLOW_STREAM: u32 = 1;
pub const STEER_PV1_FLOW_DATAGRAM: u32 = 2;
pub const STEER_PV1_FLOW_DEVICE: u32 = 3;
pub const STEER_PV1_MODE_TRUSTED_NATIVE: u32 = 1;

#[repr(C)]
pub struct steer_pv1_target {
    pub size: u32,
    pub version: u32,
    pub os: steer_pv1_span,
    pub cpu: steer_pv1_span,
    pub abi: steer_pv1_span,
    pub libc: steer_pv1_span,
    pub pointer_bits: u32,
    pub endian: u32,
}
#[repr(C)]
pub struct steer_pv1_combination {
    pub size: u32,
    pub version: u32,
    pub direction: u32,
    pub flow_class: u32,
    pub transport: steer_pv1_span,
    pub security: steer_pv1_span,
}
#[repr(C)]
pub struct steer_pv1_descriptor {
    pub size: u32,
    pub version: u32,
    pub package_id: steer_pv1_span,
    pub package_version: steer_pv1_span,
    pub provider_id: steer_pv1_span,
    pub api_range: steer_pv1_span,
    pub target: *const steer_pv1_target,
    pub combinations: *const *const steer_pv1_combination,
    pub combination_count: u32,
    pub execution_modes: *const u32,
    pub execution_mode_count: u32,
    pub required_host_capabilities: *const u32,
    pub required_host_capability_count: u32,
    pub options_schema_sha256: [u8; 32],
}

pub const STEER_PV1_ENDPOINT_IPV4: u32 = 1;
pub const STEER_PV1_ENDPOINT_IPV6: u32 = 2;
pub const STEER_PV1_ENDPOINT_DOMAIN: u32 = 3;
#[repr(C)]
pub struct steer_pv1_endpoint {
    pub size: u32,
    pub version: u32,
    pub tag: u32,
    pub address: steer_pv1_span,
    pub port: u16,
    pub reserved: u16,
}

pub const STEER_PV1_TABLE_BUFFERS: u32 = 1;
#[repr(C)]
pub struct steer_pv1_buffer_api {
    pub size: u32,
    pub version: u32,
    pub allocate: Option<unsafe extern "C" fn(
        *mut steer_pv1_host, u32, *mut *mut steer_pv1_buffer) -> steer_pv1_error>,
    pub retain: Option<unsafe extern "C" fn(
        *mut steer_pv1_host, *mut steer_pv1_buffer) -> steer_pv1_error>,
    pub release: Option<unsafe extern "C" fn(*mut steer_pv1_host, *mut steer_pv1_buffer)>,
    pub writable: Option<unsafe extern "C" fn(
        *mut steer_pv1_host, *mut steer_pv1_buffer,
        *mut steer_pv1_mut_span) -> steer_pv1_error>,
}
#[repr(C)]
pub struct steer_pv1_host_api {
    pub size: u32,
    pub version: u32,
    pub api_major: u32,
    pub api_minor: u32,
    pub get_table: Option<unsafe extern "C" fn(
        *mut steer_pv1_host, u32, u32, *mut *const c_void) -> steer_pv1_error>,
    pub monotonic_ns: Option<unsafe extern "C" fn(*mut steer_pv1_host) -> u64>,
    pub log: Option<unsafe extern "C" fn(*mut steer_pv1_host, u32, steer_pv1_span)>,
}
#[repr(C)]
pub struct steer_pv1_config {
    pub size: u32,
    pub version: u32,
    pub generation: u64,
    pub instance_id: steer_pv1_span,
    pub direction: u32,
    pub flow_class: u32,
    pub options_json: steer_pv1_span,
    pub drain_deadline_ns: u64,
}

pub type steer_pv1_prepare_done = Option<unsafe extern "C" fn(
    *mut c_void, steer_pv1_error, *mut steer_pv1_instance)>;
pub type steer_pv1_drain_done = Option<unsafe extern "C" fn(*mut c_void, steer_pv1_error)>;
#[repr(C)]
pub struct steer_pv1_provider_api {
    pub size: u32,
    pub version: u32,
    pub descriptor: *const steer_pv1_descriptor,
    pub validate: Option<unsafe extern "C" fn(
        *mut steer_pv1_host, *const steer_pv1_config) -> steer_pv1_error>,
    pub prepare: Option<unsafe extern "C" fn(
        *mut steer_pv1_host, *const steer_pv1_host_api, *const steer_pv1_config,
        steer_pv1_prepare_done, *mut c_void) -> steer_pv1_async_start>,
    pub activate: Option<unsafe extern "C" fn(*mut steer_pv1_instance) -> steer_pv1_error>,
    pub quiesce: Option<unsafe extern "C" fn(*mut steer_pv1_instance)>,
    pub drain: Option<unsafe extern "C" fn(
        *mut steer_pv1_instance, u64, steer_pv1_drain_done,
        *mut c_void) -> steer_pv1_async_start>,
    pub destroy: Option<unsafe extern "C" fn(*mut steer_pv1_instance)>,
}
pub type steer_pv1_query_fn = Option<unsafe extern "C" fn(
    *mut steer_pv1_host, *const steer_pv1_host_api,
    *mut *const steer_pv1_provider_api) -> steer_pv1_error>;

extern "C" {
    // Declaration only: no #[link], no invocation or exported implementation.
    pub fn steer_provider_query_v1(
        host: *mut steer_pv1_host, api: *const steer_pv1_host_api,
        out: *mut *const steer_pv1_provider_api) -> steer_pv1_error;
}

/// Arithmetic equivalent of the C size test, without dereferencing a table.
/// Caller separately validates pointer, prefix, layout, required slots and caps.
pub const fn field_available(size: u32, offset: usize, field_size: usize) -> bool {
    match offset.checked_add(field_size) {
        Some(end) => (size as usize) >= end,
        None => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use core::mem::{align_of, offset_of, size_of};

    fn aligned(n: usize, align: usize) -> usize { (n + align - 1) / align * align }

    #[test]
    fn table_prefixes_and_short_records() {
        macro_rules! prefix {
            ($($t:ty),+ $(,)?) => {$(
                assert_eq!(offset_of!($t, size), 0);
                assert_eq!(offset_of!($t, version), 4);
                assert!(size_of::<$t>() >= 8);
            )+};
        }
        prefix!(steer_pv1_operation_api, steer_pv1_target, steer_pv1_combination,
            steer_pv1_descriptor, steer_pv1_endpoint, steer_pv1_buffer_api,
            steer_pv1_host_api, steer_pv1_config, steer_pv1_provider_api);
        let offset = offset_of!(steer_pv1_provider_api, destroy);
        let width = size_of::<Option<unsafe extern "C" fn(*mut steer_pv1_instance)>>();
        assert!(!field_available((offset + width - 1) as u32, offset, width));
        assert!(field_available((offset + width) as u32, offset, width));
        assert!(!field_available(u32::MAX, usize::MAX, 1));
    }

    #[test]
    fn values_endpoints_and_generation_offsets() {
        let p = size_of::<*const u8>();
        let pa = align_of::<*const u8>();
        assert_eq!(size_of::<steer_pv1_error>(), 8);
        assert_eq!(offset_of!(steer_pv1_error, transient), 4);
        assert_eq!(offset_of!(steer_pv1_span, len), p);
        assert_eq!(size_of::<steer_pv1_span>(), aligned(p + 4, pa));
        assert_eq!(size_of::<steer_pv1_mut_span>(), size_of::<steer_pv1_span>());
        assert_eq!(size_of::<steer_pv1_operation_ref>(), 2 * p);
        assert_eq!(offset_of!(steer_pv1_async_start, error), 4);
        assert_eq!(offset_of!(steer_pv1_async_start, operation), aligned(12, pa));
        let address = aligned(12, pa);
        assert_eq!(offset_of!(steer_pv1_endpoint, address), address);
        assert_eq!(offset_of!(steer_pv1_endpoint, port), address + size_of::<steer_pv1_span>());
        assert_eq!(offset_of!(steer_pv1_endpoint, reserved), offset_of!(steer_pv1_endpoint, port) + 2);
        assert_eq!(offset_of!(steer_pv1_config, generation), aligned(8, align_of::<u64>()));
        let instance = aligned(offset_of!(steer_pv1_config, generation) + 8, pa);
        assert_eq!(offset_of!(steer_pv1_config, instance_id), instance);
        assert_eq!(offset_of!(steer_pv1_config, direction), instance + size_of::<steer_pv1_span>());
        let options = aligned(offset_of!(steer_pv1_config, direction) + 8, pa);
        assert_eq!(offset_of!(steer_pv1_config, options_json), options);
        assert_eq!(offset_of!(steer_pv1_config, drain_deadline_ns), aligned(options + size_of::<steer_pv1_span>(), align_of::<u64>()));
    }

    #[test]
    fn table_slots_and_target_are_pointers() {
        let p = size_of::<*const c_void>();
        let pa = align_of::<*const c_void>();
        assert_eq!(size_of::<steer_pv1_query_fn>(), p);
        assert_eq!(size_of::<steer_pv1_prepare_done>(), p);
        assert_eq!(size_of::<steer_pv1_drain_done>(), p);
        assert_eq!(offset_of!(steer_pv1_buffer_api, retain), aligned(8, pa) + p);
        assert_eq!(offset_of!(steer_pv1_provider_api, destroy), aligned(8, pa) + 6 * p);
        assert_eq!(size_of::<steer_pv1_provider_api>(), aligned(8, pa) + 7 * p);
        assert_eq!(offset_of!(steer_pv1_descriptor, target), aligned(8, pa) + 4 * size_of::<steer_pv1_span>());
        assert_eq!(offset_of!(steer_pv1_descriptor, combinations), offset_of!(steer_pv1_descriptor, target) + p);
        // Compile-time signature constraints: retain returns error, not void;
        // target is indirect so extending its tail cannot shift descriptor.
        let _: fn(&steer_pv1_buffer_api) -> Option<unsafe extern "C" fn(
            *mut steer_pv1_host, *mut steer_pv1_buffer) -> steer_pv1_error> = |t| t.retain;
        let _: fn(&steer_pv1_descriptor) -> *const steer_pv1_target = |t| t.target;
    }
}
