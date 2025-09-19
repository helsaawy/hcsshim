// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// UNSAFETY: FFI to Win32 API calls.
#![expect(unsafe_code)]

use std::collections::HashMap;
use std::ffi::OsString;
use std::fs;
use std::os::windows::ffi::OsStringExt;
use std::path::{Path, PathBuf};

use anyhow::{anyhow, Context};

use windows::core::{PCWSTR, PWSTR};
use windows::Wdk::System::OfflineRegistry::*;
use windows::Win32::System::Registry;

use super::tracing_display_path;
use crate::pcwstr::*;
use crate::reg::*;

#[tracing::instrument(level = "trace", skip(hive), fields(hive=tracing_display_path(&hive)))]
pub fn open_hive<P: AsRef<Path>>(hive: P) -> anyhow::Result<ORHKEY> {
    tracing::debug!(path = tracing_display_path(&hive), "open offline registry");

    let _ = match fs::exists(&hive) {
        Ok(false) => Err(anyhow!(
            "registry hive does not exist: {}",
            Path::display(hive.as_ref())
        )),
        r => r.context("Check if registry hive exists failed"),
    }?;

    let mut h = ORHKEY::default();
    let p = pcwstr::<&Path>(hive.as_ref());
    // SAFETY: calling Windows APIs as documented; p's lifetime is until the end of this function, so safe to dereference pointer.
    unsafe { OROpenHive(p.as_raw(), &mut h) }
        .ok()
        .map(|_| h) // consume the handle from above and replace the unit ()
        .with_context(|| format!("Open hive failed: {}", Path::display(hive.as_ref())))
}

#[tracing::instrument(level = "trace", skip(dest), fields(dest=tracing_display_path(&dest)))]
pub fn save_hive<P: AsRef<Path>>(hive: ORHKEY, dest: P) -> anyhow::Result<()> {
    tracing::debug!(path = tracing_display_path(&dest), "save offline registry");

    // ignore errors, since they will likely be re-surfaced in the ORSaveHive call below
    if let Ok(true) = fs::exists(&dest) {
        anyhow::bail!(
            "registry hive already exist: {}",
            Path::display(dest.as_ref())
        )
    };

    let p = pcwstr::<&Path>(dest.as_ref());
    // Don't support anything pre-RS5, so set version to 6.1 (Windows 8+)
    // See:
    //  https://learn.microsoft.com/en-us/windows/win32/devnotes/orsavehive
    //
    // SAFETY: calling Windows APIs as documented; p's lifetime is until the end of this function, so safe to dereference pointer.
    unsafe { ORSaveHive(hive, p.as_raw(), 6, 1) }
        .ok()
        .with_context(|| format!("Save hive failed: {}", Path::display(dest.as_ref())))
}

#[tracing::instrument(level = "trace")]
pub fn set_value(hive: ORHKEY, reg_kv: KeyValue) -> anyhow::Result<()> {
    tracing::debug!(
        key = reg_kv.key,
        name = reg_kv.name,
        "set offline registry value"
    );

    if hive.is_invalid() {
        anyhow::bail!("Invalid hive handle: {hive:?}");
    }

    let reg: RawKeyValue = reg_kv.try_into()?;
    let key = create_key_all(hive, reg.path)?;

    // SAFETY: calling Windows APIs as documented;
    // reg's lifetime is until the end of this function, so safe to dereference PCWSTR pointers.
    unsafe {
        ORSetValue(
            key,
            reg.value_name.as_raw(),
            reg.value_type.0,
            Some(&*reg.value),
        )
    }
    .ok()
    .context("Set registry value failed")
}

// TODO: use an std::iter::Iterator? (https://github.com/microsoft/windows-rs/blob/master/crates/libs/registry/src/value_iterator.rs)
#[tracing::instrument(level = "trace")]
pub fn enumerate_values(key: ORHKEY) -> anyhow::Result<HashMap<OsString, Value>> {
    tracing::trace!("enumerate offline registry values");

    let mut count = 0u32;
    let mut max_name_len = 0u32;
    let mut max_value_len = 0u32;

    // SAFETY: calling Windows APIs as documented.
    unsafe {
        ORQueryInfoKey(
            key,
            PWSTR::null(),
            None,
            None,
            None,
            None,
            Some(&mut count),
            Some(&mut max_name_len),
            Some(&mut max_value_len),
            None,
            None,
        )
    }
    .ok()
    .context("Get offline registry key information failed")?;

    tracing::debug!(count, max_name_len, max_value_len, "found key information");

    // reuse buffers across iterations
    let mut value_type = Registry::REG_VALUE_TYPE::default();
    let mut name_buff = vec![0u16; (max_name_len + 1) as usize]; // account for null terminator
    let mut value_buff = vec![0u8; max_value_len as usize];

    (0..count)
        .map(|i| {
            let mut name_len = name_buff.len() as u32;
            let mut value_len = value_buff.len() as u32;

            // SAFETY: calling Windows APIs as documented; name_ & value_buff are not modified, so Vec's won't be reallocated.
            unsafe {
                OREnumValue(
                    key,
                    i,
                    PWSTR::from_raw(name_buff.as_mut_ptr()),
                    &mut name_len,
                    Some(&mut value_type.0),
                    Some(value_buff.as_mut_ptr()),
                    Some(&mut value_len),
                )
            }
            .ok()
            .context("Get value name and data failed")?;

            let name = OsString::from_wide(&name_buff[0..name_len as usize]);
            tracing::debug!(
                value_type = tracing::field::debug(value_type),
                name = tracing::field::debug(&name),
                "found value"
            );

            let value = Value::from_raw(value_type, &value_buff[0..value_len as usize])?;
            Ok((name, value))
        })
        .collect()
}

#[tracing::instrument(level = "trace")]
pub fn get_value(key: ORHKEY, value: OwnedPcwstr) -> anyhow::Result<Value> {
    tracing::trace!("get offline registry value");

    let mut value_type = Registry::REG_VALUE_TYPE::default();
    let mut buff_len = 0u32;

    // SAFETY: calling Windows APIs as documented; value's lifetime is until the end of this function, so safe to dereference pointer.
    unsafe {
        ORGetValue(
            key,
            PCWSTR::null(),
            value.as_raw(),
            Some(&mut value_type.0),
            None,
            Some(&mut buff_len),
        )
    }
    .ok()
    .with_context(|| format!("Get value size and type failed: {value:?}",))?;

    tracing::debug!(
        length = buff_len,
        value_type = tracing::field::debug(value_type),
        name = tracing::field::debug(&value),
        "found value type and length"
    );

    let mut buff = vec![0u8; buff_len as usize];

    // SAFETY: calling Windows APIs as documented;
    // value and buff lifetimes are until the end of this function, so safe to dereference pointer.
    unsafe {
        ORGetValue(
            key,
            PCWSTR::null(),
            value.as_raw(),
            None,
            Some(buff.as_mut_ptr() as *mut _),
            Some(&mut buff_len),
        )
    }
    .ok()
    .with_context(|| format!("Get value data: {value:?}",))?;

    Value::from_raw(value_type, &buff)
}

// Iterate over path components to create them individually
// ORCreateKey ultimately calls ORParseSubKey to find the desired key or its parent.
// This only succeeds if every element in the path but the very last already exits.
// I.e., it can only create one new element at a time.
//
//https://microsoft.visualstudio.com/OS/_git/os.2020?path=/minkernel/ntos/config/offreg/ORKeyAPI.c&version=GBofficial/main&line=2375&lineEnd=2390&lineStartColumn=1&lineEndColumn=1&lineStyle=plain&_a=contents
#[tracing::instrument(level = "trace")]
pub fn create_key_all(hive: ORHKEY, key: OwnedPcwstr) -> anyhow::Result<ORHKEY> {
    use std::ffi::OsString;
    use std::path::Component;

    let p: PathBuf = OsString::from_wide(strip_null(key.slice())).into();
    let mut h = hive;

    for c in p.components() {
        match c {
            Component::RootDir => {} // should be fine if the path starts with a `\`
            Component::Normal(k) => {
                h = create_key(h, pcwstr(k))?;
            }
            c => anyhow::bail!("Unexpected component in key path: {c:?}"),
        }
    }
    if h.is_invalid() {
        // just in case
        Err(anyhow!("No valid key handles created"))
    } else {
        Ok(h)
    }
}

#[tracing::instrument(level = "trace")]
pub fn create_key(hive: ORHKEY, key: OwnedPcwstr) -> anyhow::Result<ORHKEY> {
    use windows::Win32::Security::PSECURITY_DESCRIPTOR;
    use windows::Win32::System::Registry::{
        REG_CREATED_NEW_KEY, REG_CREATE_KEY_DISPOSITION, REG_OPENED_EXISTING_KEY,
        REG_OPTION_NON_VOLATILE,
    };

    let mut h = ORHKEY::default();
    let mut created = 0u32;

    // SAFETY: calling Windows APIs as documented;
    // key's lifetime is until the end of this function, so safe to dereference pointer.
    unsafe {
        ORCreateKey(
            hive,
            key.as_raw(),
            PCWSTR::null(),
            REG_OPTION_NON_VOLATILE.0,
            PSECURITY_DESCRIPTOR(std::ptr::null_mut()),
            &mut h,
            Some(&mut created),
        )
    }
    .ok()
    .map(|_| {
        // if successful, check the dispostion
        match REG_CREATE_KEY_DISPOSITION(created) {
            REG_CREATED_NEW_KEY => {
                tracing::debug!(key = tracing::field::debug(&key), "created new key")
            }
            REG_OPENED_EXISTING_KEY => tracing::trace!("opened existing key"),
            x => tracing::warn!(
                key = tracing::field::debug(&key),
                "unknown disposition {x:?}"
            ),
        };
        h // consume the handle from above and replace the unit ()
    })
    .with_context(|| format!("Create key failed: {key:?}",))
}

#[tracing::instrument(level = "trace")]
pub fn open_key(hive: ORHKEY, key: OwnedPcwstr) -> anyhow::Result<ORHKEY> {
    tracing::trace!("open offline registry key");

    let mut h = ORHKEY::default();
    // SAFETY: calling Windows APIs as documented;
    // key's lifetime is until the end of this function, so safe to dereference pointer
    unsafe { OROpenKey(hive, key.as_raw(), &mut h) }
        .ok()
        .map(|_| h) // consume the handle from above and replace the unit ()
        .with_context(|| format!("Open key failed: {key:?}",))
}
