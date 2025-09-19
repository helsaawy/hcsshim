// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// UNSAFETY: raw pointer/byte manipulation to convert wide chars to bytes.
#![expect(unsafe_code)]

use std::ffi::OsStr;
use std::os::windows::ffi::OsStrExt;
use std::os::windows::ffi::OsStringExt;

use windows::core::PCWSTR;

// copied from below (since it not `pub`):
//  https://github.com/microsoft/windows-rs/blob/103d38b7ec98d8fb3156befd7c9d509fb083cf81/crates/libs/registry/src/pcwstr.rs

// don't implement [`std::Convert::Into`] since it consumes and (then drops) the underlying Vector,
// which can lead to a dangling pointer for any existing `PCWSTR`s.

// An owned, constant, null-terminted, wide string
pub struct OwnedPcwstr(Vec<u16>);

impl OwnedPcwstr {
    pub fn as_ptr(&self) -> *const u16 {
        debug_assert!(
            self.0.last() == Some(&0),
            "`OwnedPcwstr` isn't null-terminated"
        );
        self.0.as_ptr()
    }

    // Get the string as 8-bit bytes including the two terminating null bytes.
    pub fn as_bytes(&self) -> &[u8] {
        // SAFETY: data refers to aligned and contiguous Vec<_> buffer.
        // OwnedPcwstr is used for non-mutable strings, so underlying memory will not be modified
        // and therefore won't be reallocated.
        unsafe { core::slice::from_raw_parts(self.as_ptr() as *const _, self.0.len() * 2) }
    }

    pub fn as_raw(&self) -> PCWSTR {
        PCWSTR::from_raw(self.as_ptr())
    }

    pub fn slice(&self) -> &[u16] {
        &self.0
    }
}

impl core::fmt::Debug for OwnedPcwstr {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        use std::ffi::OsString;
        OsString::from_wide(self.slice()).fmt(f)
    }
}

pub fn pcwstr<T: AsRef<OsStr>>(value: T) -> OwnedPcwstr {
    OwnedPcwstr(
        value
            .as_ref()
            .encode_wide()
            .chain(core::iter::once(0))
            .collect::<Vec<_>>(),
    )
}

pub fn multi_pcwstr<T: AsRef<OsStr>>(value: &[T]) -> OwnedPcwstr {
    OwnedPcwstr(
        value
            .iter()
            .flat_map(|value| value.as_ref().encode_wide().chain(core::iter::once(0)))
            .chain(core::iter::once(0))
            .collect(),
    )
}

// remove the trailing `\0` byte, if one exists
pub fn strip_null(s: &[u16]) -> &[u16] {
    s.strip_suffix(&[0]).unwrap_or(s)
}
