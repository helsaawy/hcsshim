# https://psake.readthedocs.io/en/latest/structure-of-a-psake-build-script/
#
# overide with properties flag:
#  Invoke-psake ? -properties @{"Verbose"=$true}

<#
    .PARAMETER: $GoPath
    Path to the go executable

    .PARAMETER: $ExtraGoBuildFlag
    Additional build flags to add to the $GoBuildFlag property

    .PARAMETER: $ExtraGoTestFlag
    Additional test flags to add to the $GoTestFlag property

    .PARAMETER: $LinterPath
    Path to the golangci-lint executable

    .PARAMETER: $Verbose
    Enable verbose output
#>

Properties {
    $Verbose = $true # TODO: remove me
    $Script:VerbosePreference = ( $Verbose ? 'Continue' : $Global:VerbosePreference )

    $Go = Confirm-Path ($GoPath ?? 'go.exe') 'go'
    # use parameter $ExtraGoBuildFlag to add more flags, or property $GoBuildFlag to override
    $GoBuildFlag = [string[]]'-ldflags=-s -w' + $ExtraGoBuildFlag
    # use parameter $ExtraGoTestFlag to add more flags, or property $GoTestFlag to override
    $GoTestFlag = [string[]]'-gcflags=all=-d=checkptr' + $ExtraGoTestFlag
    $FuzzTime = '20s'

    $linter = Confirm-Path ($LinterPath ?? 'golangci-lint.exe') 'golangci-lint'
    $LintTimeout = '5m'

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

BuildSetup {
    #
    #   Build go executableS
    #

    $cmdbuildtasks = [string[]]@()
    $toolbuildtasks = [string[]]@()
    $linuxbuildtasks = [string[]]@()
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
        $name = New-GoBuildTask @build

        if ( $build['GoOS'] -eq 'linux' ) {
            $linuxbuildtasks += $name
        } elseif ( $build['OutDir'] -eq $ToolsBin  ) {
            $toolbuildtasks += $name
        } else {
            $cmdbuildtasks += $name
        }
    }
    Task 'BuildCmds' -description 'Build all command go packages' -depends $cmdbuildtasks
    Task 'BuildLinux' -description 'Build all linux packages' -depends $linuxbuildtasks
    Task 'BuildTools' -description 'Build all tool packages' -depends $toolbuildtasks

    #
    #   Build go test executableS
    #

    $testbuildtasks = [string[]]@()
    @(
        @{Name = 'shimtest'; Package = './test/containerd-shim-runhcs-v1'; OutDir = $TestsBin }
        @{Name = 'critest'; Package = './test/cri-containerd'; OutDir = $TestsBin }
        @{Name = 'functest'; Package = './test/functional'; OutDir = $TestsBin ; Alias = 'func' }
        @{Name = 'runhcstest'; Package = './test/runhcs'; OutDir = $TestsBin }

        # @{Name = 'gcstest'; Package = './test/gcs'; OutDir = $TestsBin; GoOS = 'linux' }
    ) | ForEach-Object {
        $build = $_
        $name = New-GoBuildTask @build -IsTest
        $testbuildtasks += $name
    }
    Task 'BuildTests' -description 'Build all test executables' -depends $testbuildtasks
    Task 'BuildAll' -description 'Build all go packages' -depends BuildCmds, BuildLinux, BuildTools, BuildTests
}

#
#   Linting
#

Task LintRepo -alias Lint -description 'Lint the entire repo' -depends LintRoot, LintTest

Task LintRoot -description 'Lint the root go module' { New-LintCmd | MyExec }

Task LintTest -description 'Lint the ''./test'' go module' { New-LintCmd './test' | MyExec }

#
#   go mod tidy and vendor
#

Task ModRepo -alias Mod -description 'Tidy and vendor the entire repo' -depends ModRoot, ModTest

Task ModRoot -description 'Tidy and vendor the root go module' {
    $gomod = "`"$go`" mod "
    (('tidy', 'vendor') | ForEach-Object { $gomod + $_ } | ConvertTo-Command) |
        Join-Scriptblock -NoCall |
        MyExec
}

Task ModTest -description 'Tidy the ''./test'' go module' -preaction {
    Set-Location ./test
} -action {
    Join-Cmd $go mod tidy | ConvertTo-Command | MyExec
}

#
#   go generate
#

Task GoGen -description "Run 'go generate' on the repo" {
    Join-Cmd $go generate -x ./... | ConvertTo-Command | MyExec
}

#
#   go test
#

Task Test -description 'Run all go unit tests in the repo' {
    Join-Cmd $go test @GoTestFlag ($Verbose ? '-v' : '') ./... | ConvertTo-Command | MyExec
}

# can only  call `go test -fuzz ..` per package, not on entire repo
Task Fuzz -description 'Run all go fuzzing tests in the repo' {
    if ( (Get-GoVersion) -lt '1.18' ) {
        Write-Warning 'Fuzzing not supported for go1.17 or less'
        return
    }
    Get-GoTestDirs -Package '.' |
        ForEach-Object {
            Join-Cmd $go test @GoTestFlag -v -run='^#' -fuzz=. "-fuzztime=$FuzzTime" $_ |
                ConvertTo-Command | MyExec
            }
}

#
#   Clean
#

Task Clean -description 'Clean binaries and build artifacts' {
    ($CmdsBin, $ToolsBin, $TestBin) | ForEach-Object {
        if ( Get-Item $_ -ErrorAction SilentlyContinue ) {
            Write-Verbose "removing $_"
            Remove-Item -Recurse -Force $_
        }
    }
}

################################################################################
#   Helper Functions
################################################################################

function New-LintCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [Parameter(Position = 0)]
        [string]
        $Dir = '.',

        [string]
        $linter = $linter,

        [string]
        $Timeout = $LintTimeout,

        [switch]
        $NoCall
    )
    Join-Cmd $linter run ('--timeout=' + $Timeout) --config=.golangci.yml `
        --max-issues-per-linter=0 --max-same-issues=0 --modules-download-mode=readonly `
    (( $Verbose )  ? '--verbose' : '') "$Dir/..." |
        ConvertTo-Command -NoCall:$NoCall
}

function New-GoBuildTask {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [Parameter(Position = 0, Mandatory)]
        [string]
        $Package,

        [Parameter(Position = 1)]
        [ValidateNotNullOrEmpty()]
        [string]
        $OutDir = $CmdsBin,

        [string]
        $Name = (Split-Path -Leaf $Package),

        [string]
        $Alias,

        [string]
        $GoOS,

        [switch]
        $IsTest
    )
    # use hashtable keep default GoOS if not specified
    $preargs = @{} + (( $GoOS ) ? @{GoOS = $GoOS } : @{})
    $cmdargs = @{Package = $Package; OutDir = $OutDir}
    $cmd = $IsTest ? (New-GoBuildTestCmd @cmdargs) : (New-GoBuildCmd @cmdargs)
    $desc = "Build go " + ($IsTest ? "test executable for" : "package")+ " '$Package'"

    # -preaction ((New -NoCall), (New-SetGoOSCmd @preargs) | Join-Scriptblock -NoCall)
    Task $Name -alias $Alias -description $desc `
        -preaction (New-SetGoOSCmd @preargs) `
        -action ( "MyExec { $cmd }" | ConvertTo-Command -NoCall ) `
        -postaction (New-ResetGoOSCmd)

    $Name
}

function New-GoBuildCmd {
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
        $GoVar = '$go',

        [switch]
        $NoCall
    )
    $tagparam = ''
    if ( $tags ) {
        $tagparam = '-tags=' + ($Tags -join ',')
    }
    Join-Cmd $GoVar build @Flags $tagparam ('-o=' + $OutDir) $Package | ConvertTo-Command -NoCall:$NoCall
}

function New-GoBuildTestCmd {
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
        $GoVar = '$go',

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

function New-SetGoOSCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [ValidateSet('windows', 'linux')]
        [string]
        $GoOS = 'windows',

        [string]
        $GoVar = '$go',

        [switch]
        $NoCall
    )
    Join-Cmd $GoVar env -w ('GOOS=' + ($GoOS.ToLower())) | ConvertTo-Command -NoCall:$NoCall
}

function New-ResetGoOSCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [string]
        $GoVar = '$go',

        [switch]
        $NoCall
    )
    Join-Cmd $GoVar env -u GOOS | ConvertTo-Command -NoCall:$NoCall
}

# function New-FindGoCmd {
#     [OutputType([scriptblock])]
#     [CmdletBinding()]
#     param (
#         [string]
#         $GoPath = $GoPath,

#         [string]
#         $GoVar = '$Script:go',

#         [switch]
#         $NoCall
#     )
#     "if ( -not $GoVar ) { $GoVar = Confirm-Path `"$GoPath`" '$GoVar' }" |
#         ConvertTo-Command -NoCall:$NoCall
# }

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
        $go = $go
    )
    $Package = Resolve-Path $Package
    $listcmd = @('list', "-tags=`'$($tags -join ',')`'", '-f' )

    $ModulePath = & $go @listcmd '{{ .Root }}' "$Package"
    & $go @listcmd  `
        '{{ if .TestGoFiles }}{{ .Dir }}{{ \"\n\" }}{{ end }}' `
        "$ModulePath/..."
}

function Get-GoVersion {
    [OutputType([version])]
    param (
        [string]
        $go = $go
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
    Write-Verbose "task action:`n$cmd"
    Exec -cmd $cmd
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
