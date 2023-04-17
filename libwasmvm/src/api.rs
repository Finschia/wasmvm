use cosmwasm_vm::{BackendApi, BackendError, BackendResult, GasInfo};

use crate::error::GoError;
use crate::memory::{U8SliceView, UnmanagedVector};

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
    pub call_callable_point: extern "C" fn(
        *const api_t,
        U8SliceView,          // input: address
        U8SliceView,          // input: name of callable point
        U8SliceView,          // input: args
        bool,                 // input: is readonly
        U8SliceView,          // input: callstack
        u64,                  // input: gas limit
        *mut UnmanagedVector, // output: returned data bytes
        *mut UnmanagedVector, // output: error message
        *mut u64,             // output: gas used
    ) -> i32,
    pub validate_interface: extern "C" fn(
        *const api_t,
        U8SliceView,          // input: address
        U8SliceView,          // input: expected interface
        *mut UnmanagedVector, // output: result serialized Option<String>, None: true, Some(e): false and e is the reason
        *mut UnmanagedVector, // output: error message
        *mut u64,             // output: gas used
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

    fn call_callable_point(
        &self,
        contract_addr: &str,
        name: &str,
        args: &[u8],
        is_readonly: bool,
        callstack: &[u8],
        gas_limit: u64,
    ) -> BackendResult<Vec<u8>> {
        let mut error_msg = UnmanagedVector::default();
        let mut result = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let name_binary = match serde_json::to_vec(name) {
            Ok(v) => v,
            Err(e) => {
                return (
                    Err(BackendError::dynamic_link_err(format!(
                        "Error during serializing callable point's name to call: {}",
                        e
                    ))),
                    GasInfo::with_cost(0),
                )
            }
        };
        let go_result: GoError = (self.vtable.call_callable_point)(
            self.state,
            U8SliceView::new(Some(contract_addr.as_bytes())),
            U8SliceView::new(Some(&name_binary)),
            U8SliceView::new(Some(args)),
            is_readonly,
            U8SliceView::new(Some(callstack)),
            gas_limit,
            &mut result as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        let result = result.consume();
        let gas_info = GasInfo::with_cost(used_gas);
        let default = || {
            format!(
                "Failed to call callable point {} of {}",
                name, contract_addr,
            )
        };
        unsafe {
            if let Err(err) = go_result.into_result(error_msg, default) {
                return (
                    Err(BackendError::dynamic_link_err(format!(
                        r#"Error during calling callable point "{}" of contract "{}": {}"#,
                        name, contract_addr, err
                    ))),
                    gas_info,
                );
            }
        }

        let result = result
            .ok_or_else(|| BackendError::unknown("Unset result"))
            .map(|data| data.to_vec());
        (result, gas_info)
    }

    // returns serialized Option<String>.
    // `None` if the interface is valid, otherwise returns `Some<err>`
    // where `err` is why it is invalid.
    fn validate_dynamic_link_interface(
        &self,
        contract_addr: &str,
        expected_interface: &[u8],
    ) -> BackendResult<Vec<u8>> {
        let mut error_msg = UnmanagedVector::default();
        let mut result = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let go_result: GoError = (self.vtable.validate_interface)(
            self.state,
            U8SliceView::new(Some(contract_addr.as_bytes())),
            U8SliceView::new(Some(expected_interface)),
            &mut result as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        let result = result.consume();
        let gas_info = GasInfo::with_cost(used_gas);
        let default = || {
            format!(
                "Failed to validate dynamic link interface of {}",
                contract_addr,
            )
        };
        unsafe {
            if let Err(err) = go_result.into_result(error_msg, default) {
                return (Err(err), gas_info);
            }
        };

        let result = result
            .ok_or_else(|| BackendError::unknown("Unset result"))
            .map(|data| data.to_vec());
        (result, gas_info)
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
    extern "C" fn mock_call_callable_point(
        _api: *const api_t,
        _addr: U8SliceView,
        _name: U8SliceView,
        _args: U8SliceView,
        _is_readonly: bool,
        _callstack: U8SliceView,
        _gas_limit: u64,
        _result: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _gas_used: *mut u64,
    ) -> i32 {
        // ok
        0
    }

    #[no_mangle]
    extern "C" fn mock_validate_interface(
        _api: *const api_t,
        _addr: U8SliceView,
        _expected_interface: U8SliceView,
        _result: *mut UnmanagedVector,
        _err: *mut UnmanagedVector,
        _gas_used: *mut u64,
    ) -> i32 {
        // ok
        0
    }

    #[test]
    fn test_canonical_address() {
        let mock_go_api_vtable = GoApi_vtable {
            humanize_address: mock_address,
            canonicalize_address: mock_address,
            call_callable_point: mock_call_callable_point,
            validate_interface: mock_validate_interface,
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
            call_callable_point: mock_call_callable_point,
            validate_interface: mock_validate_interface,
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
            call_callable_point: mock_call_callable_point,
            validate_interface: mock_validate_interface,
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
            call_callable_point: mock_call_callable_point,
            validate_interface: mock_validate_interface,
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
            call_callable_point: mock_call_callable_point,
            validate_interface: mock_validate_interface,
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
            call_callable_point: mock_call_callable_point,
            validate_interface: mock_validate_interface,
        };

        let mock_go_api = GoApi {
            state: &C_API_T as *const _,
            vtable: mock_go_api_vtable,
        };

        let (canonical_address, _) = mock_go_api.human_address(b"canonical");
        canonical_address.unwrap();
    }
}
