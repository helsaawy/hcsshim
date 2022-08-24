# https://psake.readthedocs.io/en/latest/structure-of-a-psake-build-script/
#
# overide with properties flag:
#  Invoke-psake ? -properties @{"Verbose"=$true}

Properties {
    $Go = Confirm-Path ($GoPath ?? 'go.exe') 'go'
    # use parameter $ExtraGoBuildFlag to add more flags, or property $GoBuildFlag to override
    $GoBuildFlag = [string[]]'-ldflags=-s -w' + $ExtraGoBuildFlag
    # use parameter $ExtraGoTestFlag to add more flags, or property $GoTestFlag to override
    $GoTestFlag = [string[]]'-gcflags=all=-d=checkptr' + $ExtraGoTestFlag
    $FuzzTime = '20s'

    $linter = Confirm-Path ($LinterPath ?? 'golangci-lint.exe') 'golangci-lint'
    $LintTimeout = '5m'

    $Verbose = $true

    $CmdsBin = './bin/cmd/'
    $ToolsBin = './bin/tool/'
    $TestsBin = './bin/test/'
    $OutDir = './out/'
    $ProtobufDir = './.protobuf/'
}

# todo: allow building and (fuzz) testing individual package

Task default -depends Mod

Task noop -action {}

Task ? -description 'show documentation' { WriteDocumentation }
Task ?? -description 'show detailed documentation' { WriteDocumentation($true) }

#reset state between invocations
$Script:VerbosePreference = $Global:VerbosePreference

BuildSetup {
    if ( $Verbose ) {
        $Script:VerbosePreference = 'Continue'
    }

    $buildnamescmds = [string[]]@()
    $buildnamestools = [string[]]@()
    $buildnameslinux = [string[]]@()
    @(
        @{Package = './cmd/containerd-shim-runhcs-v1'; Name = 'shim' }
        @{Package = './cmd/runhcs'; OutDir = $CmdsBin }
        @{Package = './cmd/ncproxy'; OutDir = $CmdsBin }

        @{Package = './cmd/device-util'; OutDir = $ToolsBin }
        @{Package = './cmd/wclayer'; OutDir = $ToolsBin }
        @{Package = './cmd/tar2ext4'; OutDir = $ToolsBin }
        @{Package = './cmd/shimdiag'; OutDir = $ToolsBin }
        @{Package = './internal/tools/uvmboot'; OutDir = $ToolsBin }
        @{Package = './internal/tools/zapdir'; OutDir = $ToolsBin }

        @{Package = './cmd/gcs'; OutDir = $CmdsBin; GoOS = 'linux' }
        @{Package = './cmd/gcstools'; OutDir = $CmdsBin; GoOS = 'linux' }
        @{Package = './cmd/hooks/wait-paths'; OutDir = $CmdsBin; GoOS = 'linux' }
    ) | ForEach-Object {
        $build = $_

        $name = $build['Name']
        $build.Remove('Name')
        if ( -not $name ) {
            $name = Split-Path -Leaf $build['Package']
        }

        # use hashtable keep default GoOS if not specified
        $preargs = @{}
        if ( $build['GoOS'] ) {
            $preargs['GoOS'] = $build['GoOS']
        }
        $build.Remove('GoOS')
        if ( $preargs['GoOS'] -eq 'linux' ) {
            $buildnameslinux += $name
        } elseif ( $build['OutDir'] -eq $ToolsBin  ) {
            $buildnamestools += $name
        } else {
            $buildnamescmds += $name
        }

        $alias = $build['Alias']
        $build.Remove('Alias')
        $buildcmd = "MyExec { $(Get-GoBuildCmd @build) }" | ConvertTo-Command -NoCall
        Task $name -alias $alias -description "Build go package '$($build['Package'])'" `
            # -preaction ((Get-FindGoCmd -NoCall), (Get-SetGoOSCmd @preargs) | Join-Scriptblock -NoCall) `
            -preaction (Get-SetGoOSCmd @preargs) `
            -action $buildcmd `
            -postaction (Get-ResetGoOSCmd)
    }
    Task 'BuildCmds' -description 'Build all command go packages' -depends $buildnamescmds
    Task 'BuildLinux' -description 'Build all linux packages' -depends $buildnameslinux
    Task 'BuildTools' -description 'Build all tool packages' -depends $buildnamestools

    $testnames = [string[]]@()
    @(
        @{Name = 'shimtest'; Package = './test/containerd-shim-runhcs-v1'; OutDir = $TestsBin }
        @{Name = 'critest'; Package = './test/cri-containerd'; OutDir = $TestsBin }
        @{Name = 'functest'; Package = './test/functional'; OutDir = $TestsBin ; Alias = 'func' }
        @{Name = 'runhcstest'; Package = './test/runhcs'; OutDir = $TestsBin }

        # @{Name = 'gcstest'; Package = './test/gcs'; OutDir = $TestsBin; GoOS = 'linux' }
    ) | ForEach-Object {
        $build = $_

        $name = $build['Name']
        $build.Remove('Name')
        if ( -not $name ) {
            $name = Split-Path -Leaf $build['Package']
        }
        $testnames += $name

        # use hashtable keep default GoOS if not specified
        $preargs = @{}
        if ( $build['GoOS'] ) {
            $preargs['GoOS'] = $build['GoOS']
        }
        $build.Remove('GoOS')

        $alias = $build['Alias']
        $build.Remove('Alias')
        $buildcmd = "MyExec { $(Get-GoBuildTestCmd @build) } " | ConvertTo-Command -NoCall
        Task $name -alias $alias -description "Build go test executable for '$($build['Package'])'" `
            # -preaction ((Get-FindGoCmd -NoCall), (Get-SetGoOSCmd @preargs) | Join-Scriptblock -NoCall) `
            -preaction (Get-SetGoOSCmd @preargs) `
            -action $buildcmd `
            -postaction (Get-ResetGoOSCmd)
    }
    Task 'BuildTests' -description 'Build all test executables' -depends $testnames
    Task 'BuildAll' -description 'Build all go packages' -depends BuildCmds, BuildLinux, BuildTools, BuildTests
}

Task LintRepo -alias Lint -description 'Lint the entire repo' -depends LintRoot, LintTest

Task LintRoot -description 'Lint the root go module' -preaction {
    if ( -not $Script:linter ) {
        $Script:linter = Confirm-Path $LinterPath 'golangci-lint.exe'
    }
} -action { Get-LintCmd | MyExec }

Task LintTest -description 'Lint the ''./test'' go module' -preaction {
    if ( -not $Script:linter ) {
        $Script:linter = Confirm-Path $LinterPath 'golangci-lint.exe'
    }
} -action { Get-LintCmd './test' | MyExec }

Task ModRepo -alias Mod -description 'Tidy and vendor the entire repo' -depends ModRoot, ModTest

Task ModRoot -description 'Tidy and vendor the root go module' -preaction {
    & (Get-FindGoCmd -NoCall)
} -action {
    $gomod = "`"$Script:go`" mod "
    (('tidy', 'vendor') | ForEach-Object { $gomod + $_ } | ConvertTo-Command) |
        Join-Scriptblock -NoCall |
        MyExec
}

Task ModTest -description 'Tidy the ''./test'' go module' -preaction {
    & (Get-FindGoCmd -NoCall)
    Set-Location ./test
} -action {
    Join-Cmd $Script:go mod tidy | ConvertTo-Command | MyExec
}

Task GoGen -description "Run 'go generate' on the repo" {
    Join-Cmd $Script:go generate -x ./... | ConvertTo-Command | MyExec
}

Task Test -description 'Run all go unit tests in the repo' {
    Get-FindGoCmd -NoCall | MyExec
    Join-Cmd $Script:go test @GoTestFlag -v ./... | ConvertTo-Command | MyExec
}

# can only  call `go test -fuzz ..` per package, not on entire repo
Task Fuzz -description 'Run all go fuzzing tests in the repo' {
    if ( (Get-GoVersion) -lt '1.18' ) {
        Write-Warning 'Fuzzing not supported for go1.17 or less'
        return
    }
    Get-GoTestDirs -Package '.' |
        ForEach-Object {
            Join-Cmd $Script:go test @GoTestFlag -v -run='^#' -fuzz=. "-fuzztime=$FuzzTime" $_ |
                ConvertTo-Command | MyExec
            }
}

Task Clean -description 'Clean binaries and build artifacts' {
    ($CmdsBin, $ToolsBin, $TestBin) | ForEach-Object {
        if ( Get-Item $_ -ErrorAction SilentlyContinue ) {
            Write-Verbose "removing $_"
            Remove-Item -Recurse -Force $_
        }
    }
}

function Get-GoBuildCmd {
    # todo: support setting variables with '-X path.var=value'
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [Parameter(Position = 0, Mandatory)]
        [string]
        $Package,

        [Parameter(Position = 1)]
        [string]
        $OutDir = $CmdsBin,

        [AllowEmptyCollection()]
        [string[]]
        $Tags,

        [AllowEmptyCollection()]
        [string[]]
        $Flags = $GoBuildFlag,

        [string]
        $GoVar = '$Script:go',

        [switch]
        $NoCall
    )
    $tagparam = ''
    if ( $tags ) {
        $tagparam = '-tags=' + ($Tags -join ',')
    }
    Join-Cmd $GoVar build @Flags $tagparam ('-o=' + $OutDir) $Package | ConvertTo-Command -NoCall:$NoCall
}

function Get-GoBuildTestCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [Parameter(Position = 0, Mandatory)]
        [string]
        $Package,

        [Parameter(Position = 1)]
        [string]
        $OutDir = $TestsBin,

        [AllowEmptyCollection()]
        [string[]]
        $Tags = (, 'functional'),

        [AllowEmptyCollection()]
        [string[]]
        $Flags = $GoTestFlag,

        [string]
        $GoVar = '$Script:go',

        [switch]
        $NoCall
    )
    $tagparam = ''
    if ( $tags ) {
        $tagparam = '-tags=' + ($Tags -join ',')
    }
    # `go test -c` parses `-o` as the output file, not the directory (different from `go build`)
    $out = (Join-Path $OutDir (Split-Path -Leaf $Package)) + ".test`$(& '$Go' env GOEXE)"
    Join-Cmd $GoVar test @Flags $tagparam ('-o=' + $out) '-c' $Package | ConvertTo-Command -NoCall:$NoCall
}

function Get-LintCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [Parameter(Position = 0)]
        [string]
        $Dir = '.',

        [string]
        $linter = $Script:linter,

        [string]
        $Timeout = $LintTimeout,

        [switch]
        $NoCall
    )
    Join-Cmd $Script:linter run ('--timeout=' + $Timeout) --config=.golangci.yml `
        --max-issues-per-linter=0 --max-same-issues=0 --modules-download-mode=readonly `
    (( $Verbose )  ? '--verbose' : '') "$Dir/..." |
        ConvertTo-Command -NoCall:$NoCall
}

function Get-GoTestDirs {
    [CmdletBinding()]
    [OutputType([string[]])]
    param (
        [Parameter(Position = 0)]
        [string]
        $Package = '.',

        [string[]]
        $Tags,

        [string]
        $go = $Script:go
    )
    $Package = Resolve-Path $Package
    $listcmd = @('list', "-tags=`'$($tags -join ',')`'", '-f' )

    $ModulePath = & $go @listcmd '{{ .Root }}' "$Package"
    & $go @listcmd  `
        '{{ if .TestGoFiles }}{{ .Dir }}{{ \"\n\" }}{{ end }}' `
        "$ModulePath/..."
}

function Get-SetGoOSCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [ValidateSet('windows', 'linux')]
        [string]
        $GoOS = 'windows',

        [string]
        $GoVar = '$Script:go',

        [switch]
        $NoCall
    )
    Join-Cmd $GoVar env -w ('GOOS=' + ($GoOS.ToLower())) | ConvertTo-Command -NoCall:$NoCall
}

function Get-ResetGoOSCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [string]
        $GoVar = '$Script:go',

        [switch]
        $NoCall
    )
    Join-Cmd $GoVar env -u GOOS | ConvertTo-Command -NoCall:$NoCall
}

function Get-FindGoCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [string]
        $GoPath = $GoPath,

        [string]
        $GoVar = '$Script:go',

        [switch]
        $NoCall
    )
    "if ( -not $GoVar ) { $GoVar = Confirm-Path `"$GoPath`" '$GoVar' }" |
        ConvertTo-Command -NoCall:$NoCall
}

function Get-GoVersion {
    [CmdletBinding()]
    [OutputType([version])]
    param (
        [string]
        $go = $Script:go
    )
    [version]((& $go env GOVERSION) -replace 'go', '')
}

function MyExec {
    # todo: pass all parameters to Exec
    [CmdletBinding()]
    param(
        [Parameter(Position = 0, ValueFromPipeline, Mandatory)]
        [scriptblock]$cmd
    )
    # creating a new scriptblock and invoking $cmd from in it causes scoping and recursion concerns ....
    Write-Verbose "Executing:`n$cmd"
    Exec -cmd $cmd
}

function Confirm-Path {
    [OutputType([string])]
    [CmdletBinding()]
    param(
        [Parameter(Position = 0, Mandatory)]
        [string]
        $Path,

        [Parameter(Position = 1)]
        [string]
        $Name,

        [System.Management.Automation.Commandtypes]
        $CommandType = 'Application'
    )

    $p = (Get-Command $Path -CommandType $CommandType -ErrorAction SilentlyContinue).Source

    $s = 'Invalid path' + (( $Name ) ? " to `"$Name`"" : '' )
    Assert ([bool]$p) "${s}: $Path"

    "Using `"$p`"" + (( $Name) ? " for `"$Name`"" : '' ) | Write-Verbose
    $p
}

function Join-Scriptblock {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param(
        [Parameter(Position = 0, ValueFromPipeline, ValueFromRemainingArguments, Mandatory)]
        [AllowNull()]
        [scriptblock[]]
        $Script,

        [switch]
        $NoCall
    )
    Begin {
        $b = ( $NoCall ? '' : "{`n" )
        $first = $true
    }
    Process {
        # foreach-item has weirdness with the `$_` variable
        foreach ( $s in $script ) {
            if ( $s ) {
                if ( $first ) {
                    $first = $false
                } else {
                    $b += "`n"
                }
                $b += "$s"
            }
        }
    }
    End {
        $b + ( $NoCall ? '' : "`n}" ) | ConvertTo-Command -NoCall:$NoCall
    }
}

function ConvertTo-Command {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param(
        [Parameter(Position = 0, ValueFromPipeline, Mandatory)]
        [string]
        $Cmd,

        [switch]
        $NoCall
    )
    Process {
        if ( -not $NoCall ) {
            $Cmd = '& ' + $Cmd
        }
        [scriptblock]::Create($Cmd)
    }
}

function Join-Cmd {
    ($args | ForEach-Object {
        if ( $_ ) {
            "`"$_`""
        }
    }) -join ' '
}