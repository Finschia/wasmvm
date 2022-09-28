use cosmwasm_vm::{
    copy_region_vals_between_env, write_value_to_env, Backend, BackendApi, BackendError,
    BackendResult, Checksum, Environment, FunctionMetadata, GasInfo, InstanceOptions, Querier,
    Storage, WasmerVal,
};
use std::convert::TryInto;
use std::mem::MaybeUninit;

use crate::cache::{cache_t, to_cache};
use crate::db::Db;
use crate::error::GoError;
use crate::memory::{U8SliceView, UnmanagedVector};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

// this represents something passed in from the caller side of FFI
// in this case a struct with go function pointers
#[repr(C)]
pub struct api_t {
    _private: [u8; 0],
}

// These functions should return GoError but because we don't trust them here, we treat the return value as i32
// and then check it when converting to GoError manually
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
        *mut UnmanagedVector, // env output
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
        let go_error: GoError = (self.vtable.canonicalize_address)(
            self.state,
            U8SliceView::new(Some(human.as_bytes())),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        // We destruct the UnmanagedVector here, no matter if we need the data.
        let output = output.consume();

        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoError::Other)
        let default = || format!("Failed to canonicalize the address: {}", human);
        unsafe {
            if let Err(err) = go_error.into_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let result = output.ok_or_else(|| BackendError::unknown("Unset output"));
        (result, gas_info)
    }

    fn human_address(&self, canonical: &[u8]) -> BackendResult<String> {
        let mut output = UnmanagedVector::default();
        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let go_error: GoError = (self.vtable.humanize_address)(
            self.state,
            U8SliceView::new(Some(canonical)),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        // We destruct the UnmanagedVector here, no matter if we need the data.
        let output = output.consume();

        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoError::Other)
        let default = || {
            format!(
                "Failed to humanize the address: {}",
                hex::encode_upper(canonical)
            )
        };
        unsafe {
            if let Err(err) = go_error.into_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let result = output
            .ok_or_else(|| BackendError::unknown("Unset output"))
            .and_then(|human_data| String::from_utf8(human_data).map_err(BackendError::from));
        (result, gas_info)
    }

    fn contract_call<A, S, Q>(
        &self,
        caller_env: &Environment<A, S, Q>,
        contract_addr: &str,
        func_info: &FunctionMetadata,
        args: &[WasmerVal],
    ) -> BackendResult<Box<[WasmerVal]>>
    where
        A: BackendApi + 'static,
        S: Storage + 'static,
        Q: Querier + 'static,
    {
        let mut error_msg = UnmanagedVector::default();
        let mut contract_env_out = UnmanagedVector::default();
        let mut cache_ptr_out = MaybeUninit::uninit();
        let mut db_out = MaybeUninit::uninit();
        let mut querier_out = MaybeUninit::uninit();
        let mut checksum_out = UnmanagedVector::default();
        let mut used_gas = 0_u64;

        let go_result: GoError = (self.vtable.get_contract_env)(
            self.state,
            U8SliceView::new(Some(contract_addr.as_bytes())),
            &mut contract_env_out as *mut UnmanagedVector,
            cache_ptr_out.as_mut_ptr(),
            db_out.as_mut_ptr(),
            querier_out.as_mut_ptr(),
            &mut checksum_out as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        let mut gas_info = GasInfo::with_cost(used_gas);
        let gas_limit = match caller_env.get_gas_left().checked_sub(used_gas) {
            Some(renaming) => renaming,
            None => return (Err(BackendError::out_of_gas()), gas_info),
        };

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || {
            format!(
                "Failed contract call to : {}",
                hex::encode_upper(contract_addr)
            )
        };
        unsafe {
            if let Err(err) = go_result.into_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let contract_env = match contract_env_out.consume() {
            Some(c) => c,
            None => return (Err(BackendError::unknown("invalid contract env")), gas_info),
        };

        let cache_ptr = unsafe { cache_ptr_out.assume_init() };
        let db = unsafe { db_out.assume_init() };
        let querier = unsafe { querier_out.assume_init() };

        let cache = match to_cache(cache_ptr) {
            Some(c) => c,
            None => return (Err(BackendError::unknown("failed to_cache")), gas_info),
        };

        let checksum: Checksum = match checksum_out.consume() {
            Some(c) => c.as_slice().try_into().unwrap(),
            None => return (Err(BackendError::unknown("invalid checksum")), gas_info),
        };
        let backend = into_backend(db, *self, querier);

        let print_debug = false;
        let options = InstanceOptions {
            gas_limit,
            print_debug,
        };
        let mut callee_instance = match cache.get_instance(&checksum, backend, options) {
            Ok(ins) => ins,
            Err(e) => return (Err(BackendError::unknown(e.to_string())), gas_info),
        };
        callee_instance.env.set_serialized_env(&contract_env);
        callee_instance.set_storage_readonly(caller_env.is_storage_readonly());
        match caller_env.try_pass_callstack(&mut callee_instance.env) {
            Ok(_) => {}
            Err(e) => return (Err(BackendError::user_err(e.to_string())), gas_info),
        }

        let env_arg_region_ptr = write_value_to_env(&callee_instance.env, &contract_env).unwrap();
        let mut copied_region_ptrs: Vec<WasmerVal> =
            copy_region_vals_between_env(caller_env, &callee_instance.env, args, false)
                .unwrap()
                .into();
        let mut arg_region_ptrs = vec![env_arg_region_ptr];
        arg_region_ptrs.append(&mut copied_region_ptrs);

        let call_ret = match callee_instance.call_function_strict(
            &func_info.signature,
            &func_info.name,
            &arg_region_ptrs,
        ) {
            Ok(rets) => {
                Ok(
                    copy_region_vals_between_env(&callee_instance.env, caller_env, &rets, true)
                        .unwrap(),
                )
            }
            Err(e) => Err(BackendError::dynamic_link_err(e.to_string())),
        };
        gas_info.cost += callee_instance.create_gas_report().used_internally;

        (call_ret, gas_info)
    }
}

fn into_backend(db: Db, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}
