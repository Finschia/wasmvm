use cosmwasm_vm::{
    read_region_vals_from_env, write_value_to_env, Backend, BackendApi, BackendError,
    BackendResult, Checksum, Environment, FunctionMetadata, GasInfo, InstanceOptions, Querier,
    Storage, WasmerVal,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::convert::TryInto;
use std::mem::MaybeUninit;
use wasmer::{Module, Type};

use crate::cache::{cache_t, to_cache};
use crate::db::Db;
use crate::error::GoError;
use crate::memory::{U8SliceView, UnmanagedVector};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

// A mibi (mega binary)
const MI: usize = 1024 * 1024;

// limit of sum of regions length dynamic link's input/output
// these are defined as enough big size
// input size is also limited by instantiate gas cost
const MAX_REGIONS_LENGTH_INPUT: usize = 64 * MI;
const MAX_REGIONS_LENGTH_OUTPUT: usize = 64 * MI;

// this represents something passed in from the caller side of FFI
// in this case a struct with go function pointers
#[repr(C)]
pub struct api_t {
    _private: [u8; 0],
}

#[derive(Serialize, Deserialize)]
struct CalleeDict {
    #[serde(flatten)]
    inner: std::collections::HashMap<String, bool>,
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
        u64,
        *mut UnmanagedVector, // env output
        *mut *mut cache_t,
        *mut Db,
        *mut GoQuerier,
        *mut UnmanagedVector, // checksum output
        *mut UnmanagedVector, // error message output
        *mut u64,
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
        arg_ptrs: &[WasmerVal],
    ) -> BackendResult<Box<[WasmerVal]>>
    where
        A: BackendApi + 'static,
        S: Storage + 'static,
        Q: Querier + 'static,
    {
        // read inputs
        let input_datas = match read_region_vals_from_env(
            caller_env,
            arg_ptrs,
            MAX_REGIONS_LENGTH_INPUT,
            false,
        ) {
            Ok(v) => v,
            Err(e) => return (Err(BackendError::dynamic_link_err(e)), GasInfo::free()),
        };
        let input_length = input_datas.iter().fold(0, |sum, x| sum + x.len());

        // get env from wasm module go api
        let cache_t_null_ptr: *mut cache_t = std::ptr::null_mut();
        let input_length_u64 = match input_length.try_into() {
            Ok(v) => v,
            Err(e) => return (Err(BackendError::dynamic_link_err(e)), GasInfo::free()),
        };
        let mut error_msg = UnmanagedVector::default();
        let mut contract_env_out = UnmanagedVector::default();
        let mut cache_ptr_out = MaybeUninit::new(cache_t_null_ptr);
        let mut db_out = MaybeUninit::uninit();
        let mut querier_out = MaybeUninit::uninit();
        let mut checksum_out = UnmanagedVector::default();
        let mut instantiate_cost = 0_u64;
        let mut used_gas = 0_u64;

        let go_result: GoError = (self.vtable.get_contract_env)(
            self.state,
            U8SliceView::new(Some(contract_addr.as_bytes())),
            input_length_u64,
            &mut contract_env_out as *mut UnmanagedVector,
            cache_ptr_out.as_mut_ptr(),
            db_out.as_mut_ptr(),
            querier_out.as_mut_ptr(),
            &mut checksum_out as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut instantiate_cost as *mut u64,
            &mut used_gas as *mut u64,
        )
        .into();
        let mut gas_info = GasInfo::with_cost(used_gas);

        // out of gas if instantiate cannot the limit now,
        // will not instantiate vm and not cost instantiate cost
        let gas_limit = match caller_env
            .get_gas_left()
            .checked_sub(used_gas + instantiate_cost)
        {
            Some(remaining) => remaining,
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

        // make instance
        let mut callee_instance = match cache.get_instance(&checksum, backend, options) {
            Ok(ins) => ins,
            Err(e) => return (Err(BackendError::unknown(e.to_string())), gas_info),
        };
        callee_instance.env.set_serialized_env(&contract_env);
        gas_info.cost += instantiate_cost;
        // set the callee_instance to read-only before calling _list_callable_points
        // This is because it is possible to define read-write functions in the _list_callable_points function
        // and change the storage
        callee_instance.set_storage_readonly(true);
        // set read-write permission to callee instance
        let callee_info = FunctionMetadata {
            module_name: String::from(&func_info.module_name),
            name: "_list_callable_points".to_string(),
            signature: ([], [Type::I32]).into(),
        };
        let callee_ret = match callee_instance.call_function_strict(
            &callee_info.signature,
            &callee_info.name,
            &[],
        ) {
            Ok(ret) => {
                let ret_datas = match read_region_vals_from_env(
                    &callee_instance.env,
                    &ret,
                    MAX_REGIONS_LENGTH_OUTPUT,
                    true,
                ) {
                    Ok(v) => v,
                    Err(e) => return (Err(BackendError::dynamic_link_err(e)), gas_info),
                };
                Ok(ret_datas)
            }
            Err(e) => Err(BackendError::dynamic_link_err(e.to_string())),
        };
        let callee_ret = match callee_ret {
            Ok(ret) => ret,
            Err(e) => return (Err(e), gas_info),
        };
        let callee_func_map: HashMap<String, bool> = match serde_json::from_slice(&callee_ret[0]) {
            Ok(ret) => ret,
            Err(e) => return (Err(BackendError::dynamic_link_err(e.to_string())), gas_info),
        };
        let is_read_write_permission = match callee_func_map.get(&func_info.name) {
            Some(val) => *val,
            None => {
                return (
                    Err(BackendError::dynamic_link_err(format!(
                        "callee_func_map has not key:{}",
                        &func_info.name
                    ))),
                    gas_info,
                )
            }
        };
        if caller_env.is_storage_readonly() {
            // if caller_env.is_storage_readonly() is true, funtion of dynamic linked caller has read-only permission

            if !is_read_write_permission {
                // if is_read_write_permission is false, error occurs
                // because it is not possible to inherit from read-only permission to read-write permission
                let msg = "It is not possible to inherit from read-only permission to read-write permission";
                return (Err(BackendError::dynamic_link_err(msg)), gas_info);
            }
            callee_instance.set_storage_readonly(true);
        } else {
            // if caller_env.is_storage_readonly() is false, funtion of dynamic linked caller has read-write permission
            // then, read-only and read-write are determined by the function of callee
            callee_instance.set_storage_readonly(is_read_write_permission);
        }

        // check callstack
        match caller_env.try_pass_callstack(&mut callee_instance.env) {
            Ok(_) => {}
            Err(e) => return (Err(BackendError::user_err(e.to_string())), gas_info),
        }

        // prepare inputs (+1 is for env)
        let mut arg_region_ptrs = Vec::<WasmerVal>::with_capacity(input_datas.len() + 1);

        // write env
        let env_ptr = match write_value_to_env(&callee_instance.env, &contract_env) {
            Ok(v) => v,
            Err(e) => return (Err(BackendError::dynamic_link_err(e)), gas_info),
        };
        arg_region_ptrs.push(env_ptr);

        // write inputs
        for data in input_datas {
            let ptr = match write_value_to_env(&callee_instance.env, &data) {
                Ok(v) => v,
                Err(e) => return (Err(BackendError::dynamic_link_err(e)), gas_info),
            };
            arg_region_ptrs.push(ptr);
        }

        // call
        let call_ret = match callee_instance.call_function_strict(
            &func_info.signature,
            &func_info.name,
            &arg_region_ptrs,
        ) {
            Ok(rets) => {
                let ret_datas = match read_region_vals_from_env(
                    &callee_instance.env,
                    &rets,
                    MAX_REGIONS_LENGTH_OUTPUT,
                    true,
                ) {
                    Ok(v) => v,
                    Err(e) => return (Err(BackendError::dynamic_link_err(e)), gas_info),
                };
                let mut ret_region_ptrs = Vec::<WasmerVal>::with_capacity(ret_datas.len());
                for data in ret_datas {
                    let ptr = match write_value_to_env(caller_env, &data) {
                        Ok(v) => v,
                        Err(e) => return (Err(BackendError::dynamic_link_err(e)), gas_info),
                    };
                    ret_region_ptrs.push(ptr);
                }
                Ok(ret_region_ptrs.into_boxed_slice())
            }
            Err(e) => Err(BackendError::dynamic_link_err(e.to_string())),
        };
        gas_info.cost += callee_instance.create_gas_report().used_internally;

        (call_ret, gas_info)
    }

    fn get_wasmer_module(&self, contract_addr: &str) -> BackendResult<Module> {
        let cache_t_null_ptr: *mut cache_t = std::ptr::null_mut();
        let mut error_msg = UnmanagedVector::default();
        let mut contract_env_out = UnmanagedVector::default();
        let mut cache_ptr_out = MaybeUninit::new(cache_t_null_ptr);
        let mut db_out = MaybeUninit::uninit();
        let mut querier_out = MaybeUninit::uninit();
        let mut checksum_out = UnmanagedVector::default();
        let mut instantiate_cost = 0_u64;
        let mut used_gas = 0_u64;

        let go_result: GoError = (self.vtable.get_contract_env)(
            self.state,
            U8SliceView::new(Some(contract_addr.as_bytes())),
            0,
            &mut contract_env_out as *mut UnmanagedVector,
            cache_ptr_out.as_mut_ptr(),
            db_out.as_mut_ptr(),
            querier_out.as_mut_ptr(),
            &mut checksum_out as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut instantiate_cost as *mut u64,
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
            if let Err(err) = go_result.into_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        }

        let cache_ptr = unsafe { cache_ptr_out.assume_init() };

        let checksum: Checksum = match checksum_out.consume() {
            Some(c) => c.as_slice().try_into().unwrap(),
            None => return (Err(BackendError::unknown("invalid checksum")), gas_info),
        };

        let cache = match to_cache(cache_ptr) {
            Some(c) => c,
            None => return (Err(BackendError::unknown("failed to_cache")), gas_info),
        };

        let module = match cache.get_module(&checksum) {
            Ok(module) => module,
            Err(_) => return (Err(BackendError::unknown("cannot get module")), gas_info),
        };

        (Ok(module), gas_info)
    }
}

fn into_backend(db: Db, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const C_API_T: api_t = api_t { _private: [] };

    #[no_mangle]
    extern "C" fn mock_address(
        _api: *const api_t,
        _addr: U8SliceView,
        output: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _gas_used: *mut u64,
    ) -> i32 {
        let dummy_human = String::from("dummy_address");
        unsafe { *output = UnmanagedVector::new(Some(dummy_human.into_bytes())) };

        // ok
        0
    }

    #[no_mangle]
    extern "C" fn mock_address_panic(
        _api: *const api_t,
        _addr: U8SliceView,
        _output: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _gas_used: *mut u64,
    ) -> i32 {
        // panic
        1
    }

    #[no_mangle]
    extern "C" fn mock_address_with_none_output(
        _api: *const api_t,
        _addr: U8SliceView,
        _output: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _gas_used: *mut u64,
    ) -> i32 {
        // ok
        0
    }

    #[no_mangle]
    extern "C" fn mock_get_contract_env_with_none_outputs(
        _api: *const api_t,
        _addr: U8SliceView,
        _input_len: u64,
        _env: *mut UnmanagedVector,
        _cache: *mut *mut cache_t,
        _db: *mut Db,
        _go_querier: *mut GoQuerier,
        _checksum: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _instantiate_cost: *mut u64,
        _gas_used: *mut u64,
    ) -> i32 {
        // ok
        0
    }

    #[no_mangle]
    extern "C" fn mock_get_contract_env_with_checksum(
        _api: *const api_t,
        _addr: U8SliceView,
        _input_len: u64,
        _env: *mut UnmanagedVector,
        _cache: *mut *mut cache_t,
        _db: *mut Db,
        _go_querier: *mut GoQuerier,
        checksum: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _instantiate_cost: *mut u64,
        _gas_used: *mut u64,
    ) -> i32 {
        let dummy_wasm = b"dummy_wasm";
        let dummy_checksum = Checksum::generate(dummy_wasm);
        unsafe { *checksum = UnmanagedVector::new(Some(dummy_checksum.into())) };

        // ok
        0
    }

    #[no_mangle]
    extern "C" fn mock_get_contract_env_panic(
        _api: *const api_t,
        _addr: U8SliceView,
        _input_len: u64,
        _env: *mut UnmanagedVector,
        _cache: *mut *mut cache_t,
        _db: *mut Db,
        _go_querier: *mut GoQuerier,
        _checksum: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _instantiate_cost: *mut u64,
        _gas_used: *mut u64,
    ) -> i32 {
        // panic
        1
    }

    #[test]
    fn test_canonical_address() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.canonical_address("human");
        assert_eq!(canonical_address.unwrap(), b"dummy_address")
    }

    #[test]
    #[should_panic(expected = "ForeignPanic")]
    fn test_canonical_address_panic() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address_panic,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.canonical_address("human");
        // should panic
        canonical_address.unwrap();
    }

    #[test]
    #[should_panic(expected = "Unset output")]
    fn test_canonical_address_with_none_output() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address_with_none_output,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.canonical_address("human");
        // should panic
        canonical_address.unwrap();
    }

    #[test]
    fn test_human_address() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.human_address(b"canonical");
        assert_eq!(canonical_address.unwrap(), "dummy_address")
    }

    #[test]
    #[should_panic(expected = "ForeignPanic")]
    fn test_human_address_err_with_panic() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address_panic,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.human_address(b"canonical");
        canonical_address.unwrap();
    }

    #[test]
    #[should_panic(expected = "Unset output")]
    fn test_human_address_err_with_none_output() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address_with_none_output,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.human_address(b"canonical");
        canonical_address.unwrap();
    }

    #[test]
    #[should_panic(expected = "ForeignPanic")]
    fn test_get_wasmer_module_err_with_panic() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_panic,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (module, _) = mock_go_api.get_wasmer_module("address");

        module.unwrap();
    }

    #[test]
    #[should_panic(expected = "failed to_cache")]
    fn test_get_wasmer_module_err_with_nones() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_with_checksum,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (module, _) = mock_go_api.get_wasmer_module("address");

        module.unwrap();
    }

    #[test]
    #[should_panic(expected = "invalid checksum")]
    fn test_get_wasmer_module_err_with_cache_and_nones() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address,
            get_contract_env: mock_get_contract_env_with_none_outputs,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (module, _) = mock_go_api.get_wasmer_module("address");

        module.unwrap();
    }
}
