use cosmwasm_vm::{
    Backend, BackendApi, BackendError, BackendResult, Checksum, FunctionMetadata, GasInfo,
    InstanceOptions, WasmerVal,
};
use std::convert::TryInto;
use std::mem::MaybeUninit;

use crate::cache::{cache_t, to_cache};
use crate::db::Db;
use crate::error::GoResult;
use crate::memory::{U8SliceView, UnmanagedVector};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

// this represents something passed in from the caller side of FFI
// in this case a struct with go function pointers
#[repr(C)]
pub struct api_t {
    _private: [u8; 0],
}

// These functions should return GoResult but because we don't trust them here, we treat the return value as i32
// and then check it when converting to GoResult manually
#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi_vtable {
    pub humanize_address: extern "C" fn(
        *const api_t,
        U8SliceView,
        *mut UnmanagedVector, // human output
        *mut UnmanagedVector, // error message output
        *mut u64,
    ) -> i32,
    pub canonicalize_address: extern "C" fn(
        *const api_t,
        U8SliceView,
        *mut UnmanagedVector, // canonical output
        *mut UnmanagedVector, // error message output
        *mut u64,
    ) -> i32,
    pub get_contract_env: extern "C" fn(
        *const api_t,
        U8SliceView,
        *mut *mut cache_t,
        *mut Db,
        *mut GoQuerier,
        *mut UnmanagedVector, // checksum output
        *mut UnmanagedVector, // error message output
        *mut u64,
    ) -> i32,
}

#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi {
    pub state: *const api_t,
    pub vtable: GoApi_vtable,
}

// We must declare that these are safe to Send, to use in wasm.
// The known go caller passes in immutable function pointers, but this is indeed
// unsafe for possible other callers.
//
// see: https://stackoverflow.com/questions/50258359/can-a-struct-containing-a-raw-pointer-implement-send-and-be-ffi-safe
unsafe impl Send for GoApi {}

impl BackendApi for GoApi {
    fn canonical_address(&self, human: &str) -> BackendResult<Vec<u8>> {
        let mut output = UnmanagedVector::default();
        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.vtable.canonicalize_address)(
            self.state,
            U8SliceView::new(Some(human.as_bytes())),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || format!("Failed to canonicalize the address: {}", human);
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let result = output
            .consume()
            .ok_or_else(|| BackendError::unknown("Unset output"));
        (result, gas_info)
    }

    fn human_address(&self, canonical: &[u8]) -> BackendResult<String> {
        let mut output = UnmanagedVector::default();
        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.vtable.humanize_address)(
            self.state,
            U8SliceView::new(Some(canonical)),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed to humanize the address: {}",
                hex::encode_upper(canonical)
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let result = output
            .consume()
            .ok_or_else(|| BackendError::unknown("Unset output"))
            .and_then(|human_data| String::from_utf8(human_data).map_err(BackendError::from));
        (result, gas_info)
    }

    fn contract_call(
        &self,
        contract_addr: &str,
        func_info: &FunctionMetadata,
        args: &[WasmerVal],
        gas: u64,
    ) -> BackendResult<Box<[WasmerVal]>> {
        let mut error_msg = UnmanagedVector::default();
        let mut cache_ptr_out = MaybeUninit::uninit();
        let mut db_out = MaybeUninit::uninit();
        let mut querier_out = MaybeUninit::uninit();
        let mut checksum_out = UnmanagedVector::default();
        let mut used_gas = 0_u64;

        let go_result: GoResult = (self.vtable.get_contract_env)(
            self.state,
            U8SliceView::new(Some(contract_addr.as_bytes())),
            cache_ptr_out.as_mut_ptr(),
            db_out.as_mut_ptr(),
            querier_out.as_mut_ptr(),
            &mut checksum_out as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed contract call to : {}",
                hex::encode_upper(contract_addr)
            )
        };
        unsafe {
            if let Err(err) = go_result.into_ffi_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let cache_ptr = unsafe { cache_ptr_out.assume_init() };
        let db = unsafe { db_out.assume_init() };
        let querier = unsafe { querier_out.assume_init() };

        let cache = match to_cache(cache_ptr) {
            Some(c) => c,
            None => return (Err(BackendError::foreign_panic()), gas_info),
        };

        let checksum: Checksum = match checksum_out.consume() {
            Some(c) => c.as_slice().try_into().unwrap(),
            None => return (Err(BackendError::foreign_panic()), gas_info),
        };
        let backend = into_backend(db, self.clone(), querier);
        let gas_limit = match gas.checked_sub(used_gas) {
            Some(renaming) => renaming,
            None => return (Err(BackendError::out_of_gas()), gas_info),
        };

        let print_debug = false;
        let options = InstanceOptions {
            gas_limit,
            print_debug,
        };
        let mut instance = match cache.get_instance(&checksum, backend, options) {
            Ok(ins) => ins,
            Err(e) => return (Err(BackendError::foreign_panic()), gas_info),
        };

        let call_ret =
            match instance.call_function_strict(&func_info.signature, &func_info.name, args) {
                Ok(ret) => ret,
                Err(e) => return (Err(BackendError::foreign_panic()), gas_info),
            };

        (Ok(call_ret), gas_info)
    }
}

fn into_backend(db: Db, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}
