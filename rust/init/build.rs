// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

use std::{env, fs, path::Path, str};

use anyhow::{anyhow, Context};

fn main() -> anyhow::Result<()> {
    let target_os = env::var("CARGO_CFG_TARGET_OS");
    let target_env = env::var("CARGO_CFG_TARGET_ENV");
    if Ok("windows") != target_os.as_deref() && Ok("msvc") == target_env.as_deref() {
        anyhow::bail!("invalid target OS {target_os:?} or environment {target_env:?}");
    }

    const MANIFEST_FILE: &str = "manifest.xml";
    println!("cargo:rerun-if-changed={MANIFEST_FILE}");
    // Turn linker warnings into errors.
    println!("cargo:rustc-link-arg-bins=/WX");

    //
    // Embed the Windows application manifest file.
    //
    // Copied from: https://github.com/rust-lang/rust/pull/96737/
    let mut manifest = Path::new(env!("CARGO_MANIFEST_DIR")).to_owned();
    manifest.push(MANIFEST_FILE);

    fs::metadata(&manifest)
        .with_context(|| {
            anyhow!(
                "Could not get manifest file metadata: {}",
                manifest.display()
            )
        })
        .and_then(|m| {
            if m.is_file() {
                Ok(())
            } else {
                Err(anyhow!("Manifest is not not file: {}", manifest.display()))
            }
        })?;

    println!("cargo:rustc-link-arg-bins=/MANIFEST:EMBED");
    println!(
        "cargo:rustc-link-arg-bins=/MANIFESTINPUT:{}",
        manifest.to_str().unwrap()
    );

    Ok(())
}
