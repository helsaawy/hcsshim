// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// UNSAFETY: FFI to Win32 API calls.
#![expect(unsafe_code)]

use std::path::Path;
use std::ptr;

use anyhow::anyhow;
use anyhow::Context;

use windows::Win32::Foundation::HANDLE;
use windows::Win32::Security::GetTokenInformation;
use windows::Win32::Security::TokenElevation;
use windows::Win32::Security::TOKEN_ELEVATION;

mod pcwstr;
pub use pcwstr::*;

mod reg;
pub use reg::*;

mod offreg;
pub use offreg::*;

// pseudo token with TOKEN_QUERY and TOKEN_QUERY_SOURCE access rights
// its defined as (HANDLE)(LONG_PTR) -4;
pub const CURRENT_PROCCESS_TOKEN: HANDLE = HANDLE(-4i64 as *mut _);

#[tracing::instrument(level = "debug",  fields(token = ?CURRENT_PROCCESS_TOKEN))]
pub fn require_elevated() -> anyhow::Result<()> {
    tracing::debug!("checking elevated privileges");

    let mut elev = TOKEN_ELEVATION::default();
    let info_size: u32 = size_of_val(&elev.TokenIsElevated) as _;
    let mut len = 0;

    // SAFETY: calling Windows APIs as documented; elev's lifetime is until the end of this function, so safe to dereference pointer.
    unsafe {
        GetTokenInformation(
            CURRENT_PROCCESS_TOKEN,
            TokenElevation,
            Some(ptr::from_mut(&mut elev.TokenIsElevated) as *mut _),
            info_size,
            &mut len,
        )
    }
    .ok()
    .context("Get current process token information failed")?;

    if len != info_size {
        Err(anyhow!(
            "Returned information size ({len}) does not match ({info_size})"
        ))
    } else if elev.TokenIsElevated == 0 {
        Err(anyhow!("Process is not elevated"))
    } else {
        Ok(())
    }
}

// helper function for using Path's in tracing macros, since it does not natively implement
// [std::fmt::Display]
pub fn tracing_display_path<P: AsRef<Path>>(
    p: &P,
) -> tracing::field::DisplayValue<std::path::Display<'_>> {
    tracing::field::display(Path::display(p.as_ref()))
}
