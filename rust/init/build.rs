// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

use std::env;

fn main() -> anyhow::Result<()> {
    let target_os = env::var("CARGO_CFG_TARGET_OS")?;
    if target_os != "linux" {
        anyhow::bail!("invalid target OS {target_os:?}; only linux is supported");
    }

    let target_env = env::var("CARGO_CFG_TARGET_ENV")?;
    match &*target_env {
        "musl" | "gnu" => (),
        _ => {
            anyhow::bail!("invalid target environment {target_env:?}; only must is supported");
        }
    }

    Ok(())
}
