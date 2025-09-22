// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

#![allow(unused_imports)]
#![allow(dead_code)]

use anyhow::{Context, Result, anyhow};
use clap::Parser;
use clap::ValueHint;
use std::env;
use std::ffi::OsString;
use std::os::unix::process::CommandExt;
use std::process::Child;
use std::process::Command;

use init::*;

const DEFAULT_PATH_ENV: &str = "/sbin:/usr/sbin:/bin:/usr/bin";
// #define OPEN_FDS 15

#[cfg(feature = "modules")]
// possible extensions for the kernel modules files
const KMOD_EXT: &str = ".ko";
#[cfg(feature = "modules")]
const KMOD_XZ_EXT: &str = ".ko.xz";

// When nothing is passed, default to the LCOWv1 behavior.
// const char* const default_argv[] = {"/bin/gcs", "-loglevel", "debug", "-logfile=/run/gcs/gcs.log"};
// const char* const default_shell = "/bin/sh";
// const char* const lib_modules = "/lib/modules";

#[derive(Parser, Debug)]
#[command(name = env!("CARGO_BIN_NAME"), about, long_about = None, author, version, propagate_version = true,)]
struct Cli {
    /// Set logging verbosity level
    #[arg(short, long, action = clap::ArgAction::Count)]
    verbose: u8,

    #[cfg(feature = "debug")]
    /// Port to use for stdio.
    #[arg(short, long, value_name = "PORT", default_value_t = 2056)]
    debug_port: u64,

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
    #[arg( trailing_var_arg = true, num_args(1..), required = true, value_name = "CMD", value_hint = clap::ValueHint::CommandWithArguments)]
    cmd: Vec<OsString>,
}

#[cfg(not(target_os = "linux"))]
fn main() {
    unimplemented!("init is Linux only");
}

#[cfg(target_os = "linux")]
fn main() -> Result<()> {
    let cli = Cli::parse();

    configure_tracing(&cli)?;

    // start a span after tracing is configured, so its not empty/disabled
    let span = tracing::info_span!("main");
    let _guard = span.enter();

    tracing::debug!(?cli, "parsed CLI configuration");
    // TODO: block all signals (unblock in spawns)
    // https://docs.rs/libc/latest/libc/fn.sigprocmask.html

    // cli.command.run()
    let child = launch(cli.cmd)?;
    wait(child)
}

#[tracing::instrument(level = "trace")]
fn launch(cmd_args: Vec<OsString>) -> Result<Child> {
    use std::process::Stdio;

    let (p, args) = cmd_args.split_first().context("Empty command args")?;

    // TODO:
    // https://doc.rust-lang.org/std/os/unix/process/trait.CommandExt.html#tymethod.pre_exec
    // https://docs.rs/libc/latest/libc/fn.sigprocmask.html
    //
    // preexec -> setsid, and unblock signals
    //
    // std::sys::pal::unix::cvt ->
    //     if t.is_minus_one() { Err(crate::io::Error::last_os_error()) } else { Ok(t) }

    // Unblock signals before execing.
    // sigset_t set;
    // sigfillset(&set);
    // sigprocmask(SIG_UNBLOCK, &set, 0);

    //  Create a session and process group.
    // setsid();

    // TODO (process_setsid): use `.setsid(true)` when stabalized (https://github.com/rust-lang/rust/issues/105376)
    Command::new(p)
        .args(args)
        .process_group(0)
        .stdin(Stdio::null())
        .env_clear()
        .env("PATH", DEFAULT_PATH_ENV)
        .spawn()
        .with_context(|| anyhow!("Could not spawn command: {:?}", cmd_args.join(" ".as_ref())))
}

#[tracing::instrument(level = "trace", skip(child))]
fn wait(mut child: Child) -> Result<()> {
    let status = child.wait().context("Could not wait on child")?;

    if status.success() {
        return Ok(());
    }
    match status.code() {
        Some(code) => anyhow::bail!("Child exited with code: {:?}", code),
        None => anyhow::bail!("Child terminated by signal"),
    }
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
        .with_ansi(false) // don't use ANSI colors since init TTY may not support it
        .with_source_location(false)
        .with_timer(UtcTime::rfc_3339());

    // hacky, but check if we are in a non-release build and enable extra logging formatting
    // https://doc.rust-lang.org/reference/conditional-compilation.html#debug_assertions
    #[cfg(feature = "debug")]
    let format = format.pretty();

    tracing_subscriber::fmt()
        .event_format(format)
        .with_max_level(lvl)
        .log_internal_errors(true)
        .with_writer(io::stderr)
        .try_init()
        .map_err(|e| anyhow!(e).context("Failed to enable tracing"))
}
