// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// UNSAFETY: FFI to Win32 API calls.
#![expect(unsafe_code)]

use std::alloc;
use std::fs::File;
use std::io::BufReader;
use std::ops::Deref;
use std::path::PathBuf;
use std::ptr;

use anyhow::anyhow;
use anyhow::Context;
use anyhow::Ok;
use serde::Deserialize;
use serde::Serialize;
use windows::Win32::System::Registry;

use crate::multi_pcwstr;
use crate::pcwstr;
use crate::strip_null;
use crate::OwnedPcwstr;

// based off of `windows-registry`:
//  https://github.com/microsoft/windows-rs/blob/103d38b7ec98d8fb3156befd7c9d509fb083cf81/crates/libs/registry/src
//
// (loosely) re-implemented here because:
//  - crate API are built around opening handles to the registry;
//  - `Value`'s fields (`Type` and `Data`) are `pub (crate)` and not `pub`; and
//  - `Data` and `OwnedPcwstr` (and their associated modules) aren't `pub`

/// Simplified [Windows registry] data structure.
///
/// [Windows registry]: https://learn.microsoft.com/en-us/windows/win32/sysinfo/about-the-registry
#[derive(Serialize, Deserialize, Debug)]
pub struct KeyValue {
    /// Registry key path
    pub key: String,
    /// Registry value name
    pub name: String,
    #[serde(flatten)]
    /// Registry value data
    pub value: Value,
}

/// The supporeted Windows [registry value types].
///
/// [registry value types]:  https://learn.microsoft.com/en-us/windows/win32/sysinfo/registry-value-types
#[derive(Serialize, Deserialize, Clone, PartialEq, Eq, Debug)]
#[serde(tag = "type", content = "data")]
pub enum Value {
    /// A 32-bit unsigned integer value.
    DWord(u32),

    /// A 64-bit unsigned integer value.
    QWord(u64),

    /// A string value.
    String(String),

    /// A string value that may contain unexpanded environment variables.
    ExpandString(String),

    /// An array of string values.
    MultiString(Vec<String>),

    /// An array u8 bytes.
    Bytes(Vec<u8>),
    // TODO: support other types:
    // e.g. Other(u32, Vec<u8>), little endian D/QWords, Link,
}

impl Value {
    pub fn value_type(&self) -> Registry::REG_VALUE_TYPE {
        match self {
            Self::DWord(_) => Registry::REG_DWORD,
            Self::QWord(_) => Registry::REG_QWORD,
            Self::String(_) => Registry::REG_SZ,
            Self::ExpandString(_) => Registry::REG_EXPAND_SZ,
            Self::MultiString(_) => Registry::REG_MULTI_SZ,
            Self::Bytes(_) => Registry::REG_BINARY,
        }
    }

    pub fn data(&self) -> anyhow::Result<RawData> {
        match self {
            Self::DWord(v) => v.to_le_bytes().try_into(),
            Self::QWord(v) => v.to_le_bytes().try_into(),
            Self::String(v) => RawData::from_slice(pcwstr(v).as_bytes()),
            Self::ExpandString(v) => RawData::from_slice(pcwstr(v).as_bytes()),
            Self::MultiString(v) => RawData::from_slice(multi_pcwstr(v).as_bytes()),
            Self::Bytes(v) => RawData::from_slice(v),
        }
    }

    pub fn from_raw(value_type: Registry::REG_VALUE_TYPE, buff: &[u8]) -> anyhow::Result<Self> {
        use std::ffi::OsString;
        use std::os::windows::ffi::OsStringExt;

        match value_type {
            Registry::REG_DWORD => buff
                .try_into()
                .map(|v| Self::DWord(u32::from_le_bytes(v)))
                .with_context(|| format!("Improper buffer for {value_type:?}: {buff:?}")),
            Registry::REG_QWORD => buff
                .try_into()
                .map(|v| Self::QWord(u64::from_le_bytes(v)))
                .with_context(|| format!("Improper buffer for {value_type:?}: {buff:?}")),

            Registry::REG_EXPAND_SZ | Registry::REG_SZ => RawData::from_slice(buff)
                .and_then(|data| {
                    OsString::from_wide(strip_null(data.as_wide()))
                        .into_string()
                        .map_err(|s| anyhow!("Invalid string data: {s:?}"))
                })
                .map(|s| {
                    if value_type == Registry::REG_EXPAND_SZ {
                        Self::ExpandString(s)
                    } else {
                        Self::String(s)
                    }
                }),
            Registry::REG_MULTI_SZ => RawData::from_slice(buff).and_then(|data| {
                strip_null(data.as_wide())
                    .split(|c| *c == 0)
                    .map(|b| {
                        OsString::from_wide(strip_null(b))
                            .into_string()
                            .map_err(|s| anyhow!("Invalid string data: {s:?}"))
                    })
                    .collect::<anyhow::Result<Vec<_>>>() // will stop at the first error
                    .map(Self::MultiString)
            }),
            // MultiString(Vec<String>),
            Registry::REG_BINARY => Ok(Self::Bytes(buff.to_vec())),
            _ => Err(anyhow::anyhow!(
                "Unsupported registry value type {value_type:?}"
            )),
        }
    }
}

impl core::convert::TryInto<RawKeyValue> for KeyValue {
    type Error = anyhow::Error;

    fn try_into(self) -> anyhow::Result<RawKeyValue> {
        if self.key.is_empty() {
            anyhow::bail!("Registry key name cannot be empty");
        }

        let d = self
            .value
            .data()
            .context("Encoding value to bytes failed")?;

        Ok(RawKeyValue {
            path: pcwstr(self.key),
            value_name: pcwstr(self.name),
            value_type: self.value.value_type(),
            value: d,
        })
    }
}

#[tracing::instrument(level = "debug")]
pub fn parse_delta_file(delta: &PathBuf) -> anyhow::Result<Vec<KeyValue>> {
    tracing::info!("parse registry delta JSON file");

    let file = File::open(delta)
        .with_context(|| format!("Open delta JSON file {} failed", delta.display()))?;
    let reader = BufReader::new(file);

    serde_json::from_reader(reader)
        .with_context(|| format!("Parsing delta JSON file {} failed", delta.display()))
}

pub struct RawKeyValue {
    pub path: OwnedPcwstr,
    pub value_name: OwnedPcwstr,
    pub value_type: Registry::REG_VALUE_TYPE,
    pub value: RawData,
}

// Minimal `Vec<u8>` replacement providing at least `u16` alignment so that it can be used for wide strings.
//
// Unsound or invalid alternatives:
//
// 1. Increasing the the alignment forces the struct size to increase.
//
//  ```rust
//  #[repr(align(2))]
//  struct Aligned(u8);
//
//  let v: Vec<Aligned8> = (1..7).map(|x| Aligned8(x as u8)).collect();
//  assert_eq!(std::mem::size_of_val(&v[0]), 2);
//  ```
//
//  Additionally, `#[repr(packed)]` only works for intra-struct fields, and can't be applied
//  to a `Vec` to consolidate the elements.
//
// 2. [`std::mem::transmute`] is a bit-wise copy that moves data from source into destination,
//  so the resulting slice will have the same length as the original (and not double).
//  I.e., it breaks each `u16` as two `u8`s, but only for half the array.
//
//  ```rust
//  let v: Vec<u16> = (1..7).collect();
//  let a = unsafe { std::mem::transmute::<&[u16], &[u8]>(&v[..]) };
//  assert_eq!(v.len(), a.len()); // ideally want `a.len() == 2*v.len()`
//  assert_eq!(a[0] as u16, v[0]);
//  assert_eq!(a[1], 0);
//  ```
//
//  Additionally, since they perform bit-wise moves of the values (and not the pointed-to values),
//  [`std::mem::transmute`] (or, equivalently, [`std::slice::from_raw_parts`]) does not
//  consume ownership of the original buffer, which may subsequently be dropped.
//  For example, `t` is dropped at the end of the block, so `v`s memory may be arbitrarily overwritten.
//
//  ```rust
//  let size = 7;
//  let v: &[u8] = {
//       let n: usize = (size + 1) / 2; // round up to the nearest u16 in length
//       let t: Vec<_> = std::iter::repeat_n(0u16, n).collect();
//       unsafe { std::slice::from_raw_parts(t.as_ptr() as *const u8, size) }
//  };
//  ```
//
// 3. Keeping the original buffer along side a reference to it can result in potentially unsound
//  memory access patterns, if data is mutated through either `buff` or `s`:
//
//  ```rust
//  struct AlignedU8s{
//    buff: Vec<u16>,
//    s: &[8]
//  }
//  ```
//
//  Foregoing `s` and retaining only the `Vec<u16>` data buffer would require unsafe
//  operations for all slice access, and care to make sure that the Vector is not
//  reallocated while returned slices are still active.
//
// 4. Finally, `Vec<_>` may reallocate it underlying buffer, since it owns it, so re-casting
//  via [`std::Vec::from_raw_parts`] may result in an un-aligned memory buffer if not careful.
//  Additionally, [std::Vec] cares about the original alignment the memory was allocated
//  with, so casting from `u16` to `u8` is unsafe and will cause issues when deallocating.

/// A u16-aligned byte array.
///
/// NOTE: [`Self::new()`] allocates exactly `len` bytes, regardless of `len % 2`.
/// If `len` is odd, [`Self::as_wide()`] will be one `u16` short.
/// I.e.: `self.as_wide().len() * 2 <= len`.
pub struct RawData {
    ptr: *mut u8,
    // SAFETY: `Some(_)` iff `!self.ptr.is_null()`
    layout: Option<alloc::Layout>,
}

impl RawData {
    // Creates a buffer with the specified length of zero bytes.
    pub fn new(len: usize) -> anyhow::Result<Self> {
        let bytes = Self::alloc(len)?;

        if len > 0 {
            // SAFETY: writes to pre-allocated and u16-aligned buffer.
            unsafe {
                core::ptr::write_bytes(bytes.ptr, 0, len);
            }
        }

        Ok(bytes)
    }

    // Creates a buffer by copying the bytes from the slice.
    pub fn from_slice(slice: &[u8]) -> anyhow::Result<Self> {
        let bytes = Self::alloc(slice.len())?;

        if !slice.is_empty() {
            // SAFETY: writes to pre-allocated, u16-aligned, and non-overlapping buffer.
            unsafe {
                core::ptr::copy_nonoverlapping(slice.as_ptr(), bytes.ptr, slice.len());
            }
        }

        Ok(bytes)
    }

    // Allocates a zero-initialized uninitialized, u16-aligned buffer.
    fn alloc(len: usize) -> anyhow::Result<Self> {
        if len == 0 {
            Ok(Self {
                ptr: ptr::null_mut(),
                layout: None,
            })
        } else {
            debug_assert!(len % 2 == 0, "`RawData` length isn't a multiple of 2");

            let layout = alloc::Layout::from_size_align(len, align_of::<u16>())
                .with_context(|| format!("Failed to create layout for {} bytes", len))
                .and_then(|layout| {
                    if layout.size() == 0 {
                        // really, really shouldn't happen, but double check just in case
                        anyhow::bail!("Zero-size layout");
                    }
                    Ok(layout)
                })?;
            // SAFETY: layout is non-zero in size; pointer will have at least 8 byte alignment.
            let ptr = unsafe { alloc::alloc_zeroed(layout) };

            if ptr.is_null() {
                anyhow::bail!("Allocation failed");
            }
            Ok(Self {
                ptr,
                layout: Some(layout),
            })
        }
    }

    // Returns the buffer as a slice of u16 for reading wide characters.
    pub fn as_wide(&self) -> &[u16] {
        if self.is_null() {
            &[]
        } else {
            // SAFETY: allocated to u16 alignment boundary
            unsafe { core::slice::from_raw_parts(self.ptr as *const u16, self.length() / 2) }
        }
    }

    fn length(&self) -> usize {
        if self.is_null() {
            0
        } else {
            // layout should not be None, but provide a default just in case
            self.layout.map(|l| l.size()).unwrap_or(0)
        }
    }

    fn is_null(&self) -> bool {
        // Ideally layout should never be None if ptr is non-null, but double check regardless
        self.ptr.is_null() || self.layout.is_none()
    }
}

impl core::ops::Drop for RawData {
    fn drop(&mut self) {
        if !self.ptr.is_null() {
            // SAFETY: non-null ptr was was allocated via alloc:: using self.layout.
            unsafe {
                // drop cannot fail, so acceptable to panic if self.layout is None and !self.ptr.is_null since that
                // implies our invariant is broken
                alloc::dealloc(self.ptr, self.layout.unwrap());
            }
        }
    }
}

impl core::ops::Deref for RawData {
    type Target = [u8];

    fn deref(&self) -> &[u8] {
        if self.is_null() {
            &[]
        } else {
            // SAFETY: non-null ptr was was allocated to be self.length() bytes long.
            unsafe { core::slice::from_raw_parts(self.ptr, self.length()) }
        }
    }
}

impl core::ops::DerefMut for RawData {
    fn deref_mut(&mut self) -> &mut [u8] {
        if self.ptr.is_null() {
            &mut []
        } else {
            // SAFETY: non-null ptr was was allocated to be self.length() bytes long.
            unsafe { core::slice::from_raw_parts_mut(self.ptr, self.length()) }
        }
    }
}

impl core::cmp::PartialEq for RawData {
    fn eq(&self, other: &Self) -> bool {
        self.deref() == other.deref()
    }
}

impl Eq for RawData {}

impl core::fmt::Debug for RawData {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        self.deref().fmt(f)
    }
}

impl<const N: usize> core::convert::TryFrom<[u8; N]> for RawData {
    type Error = anyhow::Error;

    fn try_from(from: [u8; N]) -> anyhow::Result<Self> {
        Self::from_slice(&from)
    }
}
