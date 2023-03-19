use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};

use cosmwasm_std::{Addr, Binary};
use cosmwasm_vm::{
    read_region_vals_from_env, set_callee_permission, write_value_to_env, Backend, Cache, Checksum,
    InstanceOptions, WasmerVal,
};
use serde_json::{from_slice, to_vec};

use crate::api::GoApi;
use crate::args::{CACHE_ARG, CHECKSUM_ARG, GAS_USED_ARG};
use crate::cache::{cache_t, to_cache};
use crate::db::Db;
use crate::error::{handle_c_error_default, Error};
use crate::memory::{ByteSliceView, UnmanagedVector};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

// A mibi (mega binary)
const MI: usize = 1024 * 1024;

// limit of sum of regions length dynamic link's input/output
// these are defined as enough big size
// input size is also limited by instantiate gas cost
const MAX_REGIONS_LENGTH_OUTPUT: usize = 64 * MI;

fn into_backend(db: Db, api: GoApi, querier: GoQuerier) -> Backend<GoApi, GoStorage, GoQuerier> {
    Backend {
        api,
        storage: GoStorage::new(db),
        querier,
    }
}

// gas_used: used gas excepted instantiate cost of the callee instance
// callstack: serialized `Vec<Addr>`. It needs to contain the caller
// args: serialized `Vec<Vec<u8>>`.
//
// This function returns empty vec if the function returns nothing
#[no_mangle]
pub extern "C" fn call_callable_point(
    name: ByteSliceView,
    cache: *mut cache_t,
    checksum: ByteSliceView,
    is_readonly: bool,
    callstack: ByteSliceView,
    env: ByteSliceView,
    args: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    gas_used: Option<&mut u64>,
    events: Option<&mut UnmanagedVector>,
    attributes: Option<&mut UnmanagedVector>,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_call_callable_point(
                name,
                c,
                checksum,
                is_readonly,
                callstack,
                env,
                args,
                db,
                api,
                querier,
                gas_limit,
                print_debug,
                events,
                attributes,
                gas_used,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let option_data = handle_c_error_default(r, error_msg);
    let data = match to_vec(&option_data) {
        Ok(v) => v,
        // Unexpected
        Err(_) => Vec::<u8>::new(),
    };
    UnmanagedVector::new(Some(data))
}

fn do_call_callable_point(
    name: ByteSliceView,
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
    is_readonly: bool,
    callstack: ByteSliceView,
    env: ByteSliceView,
    args: ByteSliceView,
    db: Db,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    print_debug: bool,
    events: Option<&mut UnmanagedVector>,
    attributes: Option<&mut UnmanagedVector>,
    gas_used: Option<&mut u64>,
) -> Result<Option<Vec<u8>>, Error> {
    let name: String = from_slice(&name.read().ok_or_else(|| Error::unset_arg("name"))?)?;
    let args: Vec<Binary> = from_slice(&args.read().ok_or_else(|| Error::unset_arg("args"))?)?;
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let callstack: Vec<Addr> = from_slice(
        &callstack
            .read()
            .ok_or_else(|| Error::unset_arg("callstack"))?,
    )?;

    let backend = into_backend(db, api, querier);
    let options = InstanceOptions {
        gas_limit,
        print_debug,
    };
    let env_u8 = env.read().ok_or_else(|| Error::unset_arg("env"))?;

    // make instance
    let mut instance = cache.get_instance(&checksum, backend, options)?;
    instance.env.set_serialized_env(env_u8);
    instance.env.set_dynamic_callstack(callstack.clone())?;

    // set permission
    set_callee_permission(&mut instance, &name, is_readonly)?;

    // prepare inputs
    let mut arg_ptrs = Vec::<WasmerVal>::with_capacity(args.len() + 1);
    let env_ptr = write_value_to_env(&instance.env, &env_u8)?;
    arg_ptrs.push(env_ptr);
    for arg in args {
        let ptr = write_value_to_env(&instance.env, arg.as_slice())?;
        arg_ptrs.push(ptr);
    }

    let call_result = match instance.call_function(&name, &arg_ptrs) {
        Ok(results) => {
            let result_datas = read_region_vals_from_env(
                &instance.env,
                &results,
                MAX_REGIONS_LENGTH_OUTPUT,
                true,
            )?;
            match result_datas.len() {
                0 => Ok(None),
                1 => Ok(Some(result_datas[0].clone())),
                _ => Err(Error::dynamic_link_err(
                    "unexpected more than 1 returning values",
                )),
            }
        }
        Err(e) => Err(Error::dynamic_link_err(e.to_string())),
    }?;

    // events
    if !is_readonly {
        let e = events.ok_or_else(|| Error::empty_arg("events"))?;
        let a = attributes.ok_or_else(|| Error::empty_arg("attributes"))?;
        let (events, attributes) = instance.get_events_attributes();
        let events_vec = match to_vec(&events) {
            Ok(v) => v,
            Err(e) => return Err(Error::invalid_events(e.to_string())),
        };
        let attributes_vec = match to_vec(&attributes) {
            Ok(v) => v,
            Err(e) => return Err(Error::invalid_attributes(e.to_string())),
        };
        *e = UnmanagedVector::new(Some(events_vec));
        *a = UnmanagedVector::new(Some(attributes_vec));
    };

    // gas
    *gas_used = instance.create_gas_report().used_internally;

    Ok(call_result)
}
