// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// `cwcow-util` binary for Confidential WCOW related utilities.
// Currently, only supports manipulating offline registries.

use std::collections::HashMap;
use std::env;
use std::ffi::OsString;
use std::fs;
use std::path::Path;
use std::path::PathBuf;

use anyhow::anyhow;
use anyhow::Context;
use clap::Parser;
use clap::Subcommand;
use clap::ValueHint;

use cwcow::*;
use windows::Wdk::System::OfflineRegistry::ORHKEY;

// TODO: make `String`s OsString
// TODO: always use anyhow::anyhow!

#[derive(Parser, Debug)]
#[command(name = env!("CARGO_BIN_NAME"), about, long_about = None, author, version, propagate_version = true,)]
struct Cli {
    /// Set logging verbosity level
    #[arg(short, long, action = clap::ArgAction::Count)]
    verbose: u8,

    /// Suppress non error logs.
    /// Takes precedence over [`verbose`].
    #[arg(short, long)]
    quiet: bool,

    #[command(subcommand)]
    command: Commands,
}

#[non_exhaustive]
#[derive(Subcommand, Debug)]
enum Commands {
    /// Manipulate an offline registry hives
    #[command(alias = "offreg")]
    OfflineRegistry {
        #[arg(short = 'f', long, alias = "file", value_name = "FILE", value_hint=ValueHint::FilePath)]
        hive: PathBuf,

        #[command(subcommand)]
        operation: OfflineRegistryOperations,
    },
}

impl Commands {
    #[tracing::instrument(level = "debug", skip(self))]
    fn run(&self) -> anyhow::Result<()> {
        require_elevated()?;

        let Commands::OfflineRegistry {
            hive, operation, ..
        } = self;
        tracing::info!("offline registry operation");
        let hhive = open_hive(hive)?;

        operation.run(hive, hhive)
    }
}

#[non_exhaustive]
#[derive(Subcommand, Debug)]
enum OfflineRegistryOperations {
    /// Add keys to an offline hive
    Add {
        #[arg(short, long, value_name = "FILE")]
        delta: PathBuf,
    },

    /// View key values in an offline hive
    Get {
        // Mostly for debug/validation
        /// The registry key to enumerate
        #[arg(short, long, value_name = "KEY")]
        key: OsString,

        /// The registry value to retrieve.
        /// All values will be retrieved if unspecified.
        #[arg(short = 'v', long = "value", value_name = "NAME")]
        value_name: Option<String>,

        /// Output result as JSON.
        #[arg(short, long)]
        json: bool,
    },
}

impl OfflineRegistryOperations {
    #[tracing::instrument(level = "debug",  skip(hive), fields(hive=tracing_display_path(&hive)))]
    fn run<P: AsRef<Path>>(&self, hive: P, hhive: ORHKEY) -> anyhow::Result<()> {
        match self {
            OfflineRegistryOperations::Add { delta } => {
                tracing::info!("adding registry values from delta json file to offline hive");

                let delta_regs = parse_delta_file(delta)?;
                let errs: Vec<anyhow::Error> = delta_regs
                    .into_iter()
                    .filter_map(|r| set_value(hhive, r).err())
                    .collect();

                if !errs.is_empty() {
                    // not really a multi-error type that I am aware of there
                    // based off of https://docs.rs/beau_collector/0.2.1/beau_collector/index.html
                    anyhow::bail!(
                        "{}",
                        errs.iter()
                            .map(|e| format!("{:#}", e))
                            .collect::<Vec<String>>()
                            .join("\n"),
                    );
                }

                // only save hive if all value updates were successful
                // Win32 ORSaveHive cannot overwrite an existing hive file, so write to a new one and replace
                let mut new_path: PathBuf = hive.as_ref().into();
                if !new_path.set_extension("new.hiv") {
                    anyhow::bail!(
                        "Setting new hive extention failed: {}",
                        Path::display(hive.as_ref())
                    );
                }

                save_hive(hhive, &new_path).context("Failed to save updated hive")?;

                tracing::info!(
                    source = tracing::field::display(new_path.display()),
                    dest = tracing_display_path(&hive),
                    "rename saved hive to original offline hive file"
                );
                fs::rename(new_path, hive).context("Renaming new hive failed")
            }
            OfflineRegistryOperations::Get {
                key,
                value_name,
                json,
            } => {
                // TODO: get sub-key names as well?
                let hkey = open_key(hhive, pcwstr(key))?;

                let values = if value_name.is_none() {
                    enumerate_values(hkey)
                } else {
                    let name_str: OsString = value_name.as_deref().unwrap().into();
                    get_value(hkey, pcwstr(&name_str)).map(|rv| HashMap::from([(name_str, rv)]))
                }?;

                match json {
                    true => {
                        let key_str = key.to_string_lossy().into_owned();
                        // ignore/surpress unicode conversion errors here since its just for printing

                        let kvs: Vec<_> = values
                            .into_iter()
                            .map(|(name, value)| KeyValue {
                                key: key_str.clone(),
                                name: name.to_string_lossy().into_owned(),
                                value,
                            })
                            .collect();
                        serde_json::to_string_pretty(&kvs)
                            .context("json encoding failed")
                            .map(|s| println!("{s}"))?;
                    }
                    false => {
                        println!("{key:?}:");
                        for (name, val) in values {
                            println!("  {name:?}:\n\t{val:?}")
                        }
                    }
                }

                Ok(())
            }
        }
    }
}

fn main() -> anyhow::Result<()> {
    #[cfg(not(windows))]
    anyhow::bail!("how did you build this?");

    // will call `std::process::exit`, but we get pretty printing for help and errors and co., so thats fine
    let cli = Cli::parse();

    configure_tracing(&cli)?;

    // start a span after tracing is configured, so its not empty/disabled
    let span = tracing::info_span!("main");
    let _guard = span.enter();

    tracing::debug!(?cli, "parsed CLI configuration");

    cli.command.run()
}

// based on
// https://microsoft.visualstudio.com/HyperVCloud/_git/openvmm?path=/openvmm/openvmm_entry/src/tracing_init.rs
fn configure_tracing(cli: &Cli) -> anyhow::Result<()> {
    use std::io;
    use std::io::IsTerminal;
    use tracing_subscriber::filter::LevelFilter;
    use tracing_subscriber::fmt::format::*;
    use tracing_subscriber::fmt::time::UtcTime;

    let lvl = if cli.quiet {
        LevelFilter::WARN
    } else {
        match cli.verbose {
            0 => LevelFilter::INFO,
            1 => LevelFilter::DEBUG,
            // should warn that values greater than 2 are being ignored, but logging isn't enabled, so ...
            2.. => LevelFilter::TRACE,
        }
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
