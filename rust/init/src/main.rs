// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

#![allow(unused_imports)]

use anyhow::Context;
use anyhow::anyhow;
use clap::Parser;
use clap::ValueHint;
use std::env;
use std::ffi::OsString;

use init::*;

// #define DEFAULT_PATH_ENV "PATH=/sbin:/usr/sbin:/bin:/usr/bin"
// #define OPEN_FDS 15

// const char* const default_envp[] = {
//     DEFAULT_PATH_ENV,
//     NULL,
// };

// #ifdef MODULES
// // global kmod k_ctx so we can access it in the file tree traversal
// struct kmod_ctx* k_ctx;

// // possible extensions for the kernel modules files
// const char* kmod_ext = ".ko";
// const char* kmod_xz_ext = ".ko.xz";
// #endif

// // When nothing is passed, default to the LCOWv1 behavior.
// const char* const default_argv[] = {"/bin/gcs", "-loglevel", "debug", "-logfile=/run/gcs/gcs.log"};
// const char* const default_shell = "/bin/sh";
// const char* const lib_modules = "/lib/modules";


#[derive(Parser, Debug)]
#[command(name = env!("CARGO_BIN_NAME"), about, long_about = None, author, version, propagate_version = true,)]
struct Cli {
    /// Set logging verbosity level
    #[arg(short, long, action = clap::ArgAction::Count)]
    verbose: u8,

    /// Launch the debug shell after the specified command.
    #[arg(short, long, value_name = "PATH")]
    debug_shell: Option<OsString>,

    /// Vsock port to inject boot-time entropy from.
    #[arg(short, long, value_name = "PORT")]
    entropy_port: u64,

    /// Create a writable overlay mount over the specified directory.
    /// Can be repeated to specify multiple directories.
    #[arg(short, long,  value_name = "PATH", value_hint = clap::ValueHint::DirPath)]
    writable_overlay: Vec<OsString>,

    /// Command line to exec after initializing.
    #[arg(value_name = "CMD", trailing_var_arg = true, num_args(2..), value_hint = clap::ValueHint::CommandWithArguments)]
    cmd: Vec<OsString>,
}

#[cfg(not(target_os = "linux"))]
fn main() {
    unimplemented!("init is Linux only");
}

#[cfg(target_os = "linux")]
fn main() -> anyhow::Result<()> {
    // will call `std::process::exit`, but we get pretty printing for help and errors and co., so thats fine
    let cli = Cli::parse();

    configure_tracing(&cli)?;

    // start a span after tracing is configured, so its not empty/disabled
    let span = tracing::info_span!("main");
    let _guard = span.enter();

    tracing::debug!(?cli, "parsed CLI configuration");

    // cli.command.run()
    Ok(())
}

// based on
// https://microsoft.visualstudio.com/HyperVCloud/_git/openvmm?path=/openvmm/openvmm_entry/src/tracing_init.rs
fn configure_tracing(cli: &Cli) -> anyhow::Result<()> {
    use std::io;
    use std::io::IsTerminal;
    use tracing_subscriber::filter::LevelFilter;
    use tracing_subscriber::fmt::format::*;
    use tracing_subscriber::fmt::time::UtcTime;

    let lvl = match cli.verbose {
        0 => LevelFilter::WARN,
        1 => LevelFilter::INFO,
        2 => LevelFilter::DEBUG,
        // should warn that values greater than 3 are being ignored, but logging isn't enabled, so ...
        3.. => LevelFilter::TRACE,
    };

    let format = Format::default()
        .with_ansi(std::io::stderr().is_terminal())
        .with_source_location(false)
        .with_timer(UtcTime::rfc_3339());

    // hacky, but check if we are in a non-release build and enable extra logging formatting
    // https://doc.rust-lang.org/reference/conditional-compilation.html#debug_assertions
    #[cfg(debug_assertions)]
    let format = format.pretty();

    tracing_subscriber::fmt()
        .event_format(format)
        .with_max_level(lvl)
        .log_internal_errors(true)
        .with_writer(io::stderr)
        .try_init()
        .map_err(|e| anyhow!(e).context("Failed to enable tracing"))
}
