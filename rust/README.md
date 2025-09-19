# Rust Source Code

Workspace for Rust code

## Windows Dev Setup

1. Install [Visual Studio C++ build environment][msvc-install].

2. Install Microsoft Rust Toolchain
   a. Follow [instructions][msrust-instructions] to install `AzureAuth` and `MSRustup`.

   b. [Configure Cargo][cargo-cred-provider] to be able to authenticate into our [ADO artifact feed][publicpackages-feed].

    > [!NOTE]
    > `msrustup` may handle this automatically.

   c. Run `msrustup toolchain install` to install the toolchain specified in `rust-toolchain.toml`.

3. Install some helpful cargo features:

   ```shell
   > cargo install cargo-make --no-default-features --features tls-native
   > cargo install cargo-expand cargo-edit
   ```

## Directory Tree

Directory layout:

```text
rust/                                        Root Rust workspace
|   scripts/                                 Helper scripts
|   |    Assert-CopyrightHeaders.ps1         Add Microsoft copyright header to source files
|   init/                                    Linux uVM init binary
```

[msvc-install]: https://eng.ms/docs/more/languages-at-microsoft/cpp/articles/gettingstarted/install/installdev
[msrust-instructions]: https://eng.ms/docs/more/languages-at-microsoft/rust/articles/gettingstarted/install/msrustup
[cargo-cred-provider]: https://eng.ms/docs/more/languages-at-microsoft/rust/articles/gettingstarted/install/msrustup#cargo-credential-provider
[publicpackages-feed]: https://msazure.visualstudio.com/ContainerPlatform/_artifacts/feed/ContainerPlatform_PublicPackages
