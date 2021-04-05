use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::str::from_utf8;

use cosmwasm_vm::{features_from_csv, Cache, CacheOptions, Checksum, Size};

use crate::api::GoApi;
use crate::args::{CACHE_ARG, CHECKSUM_ARG, DATA_DIR_ARG, FEATURES_ARG, WASM_ARG};
use crate::error::{clear_error, handle_c_error_binary, handle_c_error_default, set_error, Error};
use crate::memory::{ByteSliceView, UnmanagedVector};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

#[repr(C)]
pub struct cache_t {}

pub fn to_cache(ptr: *mut cache_t) -> Option<&'static mut Cache<GoApi, GoStorage, GoQuerier>> {
    if ptr.is_null() {
        None
    } else {
        let c = unsafe { &mut *(ptr as *mut Cache<GoApi, GoStorage, GoQuerier>) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn init_cache(
    data_dir: ByteSliceView,
    supported_features: ByteSliceView,
    cache_size: u32,
    instance_memory_limit: u32,
    error_msg: Option<&mut UnmanagedVector>,
) -> *mut cache_t {
    let r = catch_unwind(|| {
        do_init_cache(
            data_dir,
            supported_features,
            cache_size,
            instance_memory_limit,
        )
    })
    .unwrap_or_else(|_| Err(Error::panic()));
    match r {
        Ok(t) => {
            clear_error();
            t as *mut cache_t
        }
        Err(e) => {
            set_error(e, error_msg);
            std::ptr::null_mut()
        }
    }
}

fn do_init_cache(
    data_dir: ByteSliceView,
    supported_features: ByteSliceView,
    cache_size: u32,
    instance_memory_limit: u32, // in MiB
) -> Result<*mut Cache<GoApi, GoStorage, GoQuerier>, Error> {
    let dir = data_dir
        .read()
        .ok_or_else(|| Error::unset_arg(DATA_DIR_ARG))?;
    let dir_str = String::from_utf8(dir.to_vec())?;
    // parse the supported features
    let features_bin = supported_features
        .read()
        .ok_or_else(|| Error::unset_arg(FEATURES_ARG))?;
    let features_str = from_utf8(features_bin)?;
    let features = features_from_csv(features_str);
    let memory_cache_size = Size::mebi(
        cache_size
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let instance_memory_limit = Size::mebi(
        instance_memory_limit
            .try_into()
            .expect("Cannot convert u32 to usize. What kind of system is this?"),
    );
    let options = CacheOptions {
        base_dir: dir_str.into(),
        supported_features: features,
        memory_cache_size,
        instance_memory_limit,
    };
    let cache = unsafe { Cache::new(options) }?;
    let out = Box::new(cache);
    Ok(Box::into_raw(out))
}

#[no_mangle]
pub extern "C" fn save_wasm(
    cache: *mut cache_t,
    wasm: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_save_wasm(c, wasm)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let checksum = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(checksum))
}

fn do_save_wasm(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    wasm: ByteSliceView,
) -> Result<Checksum, Error> {
    let wasm = wasm.read().ok_or_else(|| Error::unset_arg(WASM_ARG))?;
    let checksum = cache.save_wasm(wasm)?;
    Ok(checksum)
}

#[no_mangle]
pub extern "C" fn load_wasm(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_load_wasm(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(data))
}

fn do_load_wasm(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<Vec<u8>, Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let wasm = cache.load_wasm(&checksum)?;
    Ok(wasm)
}

#[no_mangle]
pub extern "C" fn pin(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_pin(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg);
}

fn do_pin(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    cache.pin(&checksum)?;
    Ok(())
}

#[no_mangle]
pub extern "C" fn unpin(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_unpin(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg);
}

fn do_unpin(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    cache.unpin(&checksum)?;
    Ok(())
}

#[repr(C)]
#[derive(Copy, Clone, Default, Debug, PartialEq)]
pub struct AnalysisReport {
    pub has_ibc_entry_points: bool,
}

impl From<cosmwasm_vm::AnalysisReport> for AnalysisReport {
    fn from(report: cosmwasm_vm::AnalysisReport) -> Self {
        AnalysisReport {
            has_ibc_entry_points: report.has_ibc_entry_points,
        }
    }
}

#[no_mangle]
pub extern "C" fn analyze_code(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> AnalysisReport {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_analyze_code(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    match r {
        Ok(value) => {
            clear_error();
            value
        }
        Err(error) => {
            set_error(error, error_msg);
            AnalysisReport::default()
        }
    }
}

fn do_analyze_code(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<AnalysisReport, Error> {
    let checksum: Checksum = checksum
        .read()
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?
        .try_into()?;
    let report = cache.analyze(&checksum)?;
    Ok(report.into())
}

/// frees a cache reference
///
/// # Safety
///
/// This must be called exactly once for any `*cache_t` returned by `init_cache`
/// and cannot be called on any other pointer.
#[no_mangle]
pub extern "C" fn release_cache(cache: *mut cache_t) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = unsafe { Box::from_raw(cache as *mut Cache<GoApi, GoStorage, GoQuerier>) };
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    static HACKATOM: &[u8] = include_bytes!("../api/testdata/hackatom.wasm");
    static IBC_REFLECT: &[u8] = include_bytes!("../api/testdata/ibc_reflect.wasm");

    #[test]
    fn init_cache_and_release_cache_work() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features: &[u8] = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn init_cache_writes_error() {
        let dir: String = String::from("borken\0dir"); // null bytes are valid UTF8 but not allowed in FS paths
        let features: &[u8] = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert!(cache_ptr.is_null());
        assert_eq!(error_msg.is_some(), true);
        let msg = String::from_utf8(error_msg.consume().unwrap()).unwrap();
        assert_eq!(msg, "Error calling the VM: Cache error: Error creating Wasm dir for cache: data provided contains a nul byte");
    }

    #[test]
    fn save_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features: &[u8] = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn load_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features: &[u8] = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let wasm = load_wasm(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let wasm = wasm.consume().unwrap_or_default();
        assert_eq!(wasm, HACKATOM);

        release_cache(cache_ptr);
    }

    #[test]
    fn pin_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features: &[u8] = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // pinning again has no effect
        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn unpin_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features: &[u8] = b"staking";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        // Unpinning again has no effect
        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn analyze_code_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let features: &[u8] = b"stargate";

        let mut error_msg = UnmanagedVector::default();
        let cache_ptr = init_cache(
            ByteSliceView::new(dir.as_bytes()),
            ByteSliceView::new(features),
            512,
            32,
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum_hackatom = save_wasm(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum_hackatom = checksum_hackatom.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let checksum_ibc_reflect = save_wasm(
            cache_ptr,
            ByteSliceView::new(IBC_REFLECT),
            Some(&mut error_msg),
        );
        assert_eq!(error_msg.is_none(), true);
        let _ = error_msg.consume();
        let checksum_ibc_reflect = checksum_ibc_reflect.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let hackatom_report = analyze_code(
            cache_ptr,
            ByteSliceView::new(&checksum_hackatom),
            Some(&mut error_msg),
        );
        let _ = error_msg.consume();
        assert_eq!(
            hackatom_report,
            AnalysisReport {
                has_ibc_entry_points: false
            }
        );

        let mut error_msg = UnmanagedVector::default();
        let ibc_reflect_report = analyze_code(
            cache_ptr,
            ByteSliceView::new(&checksum_ibc_reflect),
            Some(&mut error_msg),
        );
        let _ = error_msg.consume();
        assert_eq!(
            ibc_reflect_report,
            AnalysisReport {
                has_ibc_entry_points: true
            }
        );

        release_cache(cache_ptr);
    }
}
