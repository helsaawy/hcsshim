<#
    Use -Verbose flag to enable verbose output

    .PARAMETER $GoPath
    Path to the go executable

    .PARAMETER $ExtraGoBuildFlag
    Additional build flags to add to the $GoBuildFlag property

    .PARAMETER $ExtraGoTestFlag
    Additional test flags to add to the $GoTestFlag property

    .PARAMETER $LinterPath
    Path to the golangci-lint executable

    .EXAMPLE
    Invoke-psake ?

    List all tasks. Use '??' or '???' to increase the information shown

    .EXAMPLE
    Invoke-psake -Verbose -properties @{GoPath="C:\go\bin\go.exe"} List

    Show all internal variables, as used by tasks.

    .EXAMPLE
    Invoke-psake -Verbose Lint

    Lint the entire repo, with verbose output.

    .LINK
    https://psake.readthedocs.io/en/latest/structure-of-a-psake-build-script/
#>

Properties {
    $go = Confirm-CommandPath ($GoPath ?? 'go') 'go'
    # use parameter $ExtraGoBuildFlag to add more flags, or property $GoBuildFlag to override
    $GoBuildFlag = [string[]]'-ldflags=-s -w' + ($ExtraGoBuildFlag ?? @())
    # use parameter $ExtraGoTestFlag to add more flags, or property $GoTestFlag to override
    $GoTestFlag = [string[]]'-gcflags=all=-d=checkptr' + ($ExtraGoTestFlag ?? @())
    $FuzzTime = '20s'

    $linter = Confirm-CommandPath ($LinterPath ?? 'golangci-lint') 'golangci-lint'
    $LintTimeout = '5m'
    $LintConfig = Confirm-Path './.golangci.yml' 'linter config'

    $BinDir = Join-Path (Get-Location) 'bin/'
    $CmdsBin = Join-Path $BinDir 'cmd/'
    $ToolsBin = Join-Path $BinDir 'tool/'
    $TestsBin = Join-Path $BinDir 'test/'
    $OutDir = './out/'

    $ProtobufDir = './.protobuf/'


    $CPlatDir = Confirm-Path 'C:/ContainerPlat' 'ContainerPlat Directory'
    $CPlatDataDir = Confirm-Path 'C:/ContainerPlatData' 'ContainerPlat Data Directory'
}

#TODO: allow building and (fuzz) testing individual package
#TODO: protobuff
#TODO: move shim

Task default -depends Mod

Task ? -description 'show documentation' -preaction { Disable-TimingReport } `
    -action { WriteDocumentation }
Task ?? -description 'show detailed documentation' -preaction { Disable-TimingReport } `
    -action { WriteDocumentation $True }
Task ??? -description 'show even more detailed documentation' -preaction { Disable-TimingReport } `
    -action {
    $psake.context.Peek().tasks.Keys |
        Where-Object {
            $_ -ne 'default' -and $_ -notmatch '^\?+$'
        } | ForEach-Object {
            $task = $currentContext.tasks.$_
            New-Object PSObject -Property @{
                Name          = $task.Name;
                Alias         = $task.Alias;
                Description   = $task.Description;
                DependsOn     = $task.DependsOn -join ', ' ;
                Precondition  = $task.Precondition;
                Preaction     = $task.Preaction;
                Action        = $task.Action;
                Postaction    = $task.Postaction;
                Postcondition = $task.Postcondition;
            }
        } |
        Sort-Object 'Name' |
        Format-List -Property Name, Alias, Description, `
            Precondition, Preaction, Action, Postaction, Postcondition
}

BuildSetup {
    #
    #   Build go executableS
    #

    $cmdbuildtasks = [string[]]@()
    $toolbuildtasks = [string[]]@()
    $linuxbuildtasks = [string[]]@()
    @(
        @{ Name = 'shim' ; Package = './cmd/containerd-shim-runhcs-v1' }
        @{ Package = './cmd/runhcs'; OutDir = $CmdsBin }
        @{ Package = './cmd/ncproxy'; OutDir = $CmdsBin }

        @{ Package = './cmd/device-util'; OutDir = $ToolsBin }
        @{ Package = './cmd/wclayer'; OutDir = $ToolsBin }
        @{ Package = './cmd/tar2ext4'; OutDir = $ToolsBin }
        @{ Package = './cmd/shimdiag'; OutDir = $ToolsBin }
        @{ Package = './internal/tools/uvmboot'; OutDir = $ToolsBin }
        @{ Package = './internal/tools/zapdir'; OutDir = $ToolsBin }

        @{ Package = './cmd/gcs'; OutDir = $CmdsBin; GoOS = 'linux' }
        @{ Package = './cmd/gcstools'; OutDir = $CmdsBin; GoOS = 'linux' }
        @{ Package = './cmd/hooks/wait-paths'; OutDir = $CmdsBin; GoOS = 'linux' }
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
        @{ Name = 'shimtest'; Package = './containerd-shim-runhcs-v1'; OutDir = $TestsBin }
        @{ Name = 'critest'; Package = './cri-containerd'; OutDir = $TestsBin }
        @{ Name = 'functest'; Alias = 'func' ; Package = './functional'; OutDir = $TestsBin }
        @{ Name = 'runhcstest'; Package = './runhcs'; OutDir = $TestsBin }

        # @{Name = 'gcstest'; Package = './gcs'; OutDir = $TestsBin; GoOS = 'linux' }
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

#TODO: make autogeneragted
#TODO: use env variables instead of `go env -w`

Task LintRepo -alias Lint -description 'Lint the entire repo' -depends LintRoot, LintTest, LintLinux

Task LintRoot -description 'Lint the root go module' -preaction {
    Assert ([bool]$linter) 'Unable to find golangci-linter executable'
    $Env:GOWORK = 'off'
} -action {
    New-LintCmd | MyExec
} -postaction {
    $Env:GOWORK = $null
}

Task LintTest -description 'Lint the ''./test'' go module' -preaction {
    Assert ([bool]$linter) 'Unable to find golangci-linter executable'
    $Env:GOWORK = 'off'
    Set-Location ./test
} -action {
    New-LintCmd | MyExec
} -postaction {
    Set-Location ..
    $Env:GOWORK = $null
}

Task LintLinux -description 'Lint the entire repo for Linux' -depends LintRootLinux, LintTestLinux

Task LintRootLinux -description 'Lint the root go module' -preaction {
    Assert ([bool]$linter) 'Unable to find golangci-linter executable'
    Assert-GoPath
    $Env:GOWORK = 'off'
    New-SetGoOSCmd -GoOS 'linux' | MyExec
} -action {
    New-LintCmd ./cmd/gcs/... ./cmd/gcstools/... ./internal/guest... ./internal/tools/... ./pkg/... | MyExec
} -postaction {
    $Env:GOWORK = $null
    New-ResetGoOSCmd | MyExec
}

Task LintTestLinux -description 'Lint the ''./test'' go module' -preaction {
    Assert ([bool]$linter) 'Unable to find golangci-linter executable'
    Assert-GoPath
    $Env:GOWORK = 'off'
    New-SetGoOSCmd -GoOS 'linux' | MyExec
    Set-Location ./test
} -action {
    New-LintCmd | MyExec
} -postaction {
    Set-Location ..
    $Env:GOWORK = $null
    New-ResetGoOSCmd | MyExec
}

#
#   go mod tidy and vendor
#

Task ModRepo -alias Mod -description 'Tidy and vendor the entire repo' -depends ModRoot, ModTest

Task ModRoot -description 'Tidy and vendor the root go module' `
    -preaction { Assert-GoPath } `
    -action { ( (Join-Cmd $go mod tidy), (Join-Cmd $go mod vendor) ) |
        ConvertTo-Command | Join-Scriptblock -NoCall | MyExec }

Task ModTest -description 'Tidy the ''./test'' go module' `
    -preaction { Assert-GoPath ; Set-Location ./test } `
    -action { Join-Cmd $go mod tidy | ConvertTo-Command | MyExec } `
    -postaction { Set-Location .. }

#
#   go generate
#

Task GoGen -description "Run 'go generate' on the repo" -preaction { Assert-GoPath } `
    -action { Join-Cmd $go generate -x ./... | ConvertTo-Command | MyExec }

#
#   go test
#

Task Test -description 'Run all go unit tests in the repo' -preaction { Assert-GoPath } `
    -action { Join-Cmd $go test @GoTestFlag ($Verbose ? '-v' : '') ./... | ConvertTo-Command | MyExec }

# can only  call `go test -fuzz ..` per package, not on entire repo
Task Fuzz -description 'Run all go fuzzing tests in the repo' -precondition {
    Assert-GoPath # preconditions run before preactions, but checking go version requires go ...
    ((Get-GoVersion) -gt '1.17') -or (Write-Warning 'Fuzzing not supported for go1.17 or less')
} -action {
    Get-GoTestDirs -Package '.' |
        ForEach-Object {
            Join-Cmd $go test @GoTestFlag -v -run='^#' -fuzz=. "-fuzztime=$FuzzTime" $_ |
                ConvertTo-Command | MyExec
            }
}

#
#   Clean & misc
#

Task Clean -description 'Clean binaries and build artifacts' {
    ($CmdsBin, $ToolsBin, $TestsBin, $BinDir, $OutDir, $ProtobufDir) | ForEach-Object {
        if ( $_ -and (Get-Item $_ -ErrorAction SilentlyContinue) ) {
            Write-Verbose "removing $_"
            Remove-Item -Recurse -Force $_
        }
    }
}

Task List -description 'List properties and parameters (for debugging)' `
    -preaction { Disable-TimingReport } `
    -action {
    MyExec { Write-Output 'sdfsd' }
    return
    ('Verbose',
    'VerbosePreference',
    'Go',
    'GoBuildFlag',
    'ExtraGoBuildFlag',
    'GoTestFlag',
    'ExtraGoTestFlag',
    'FuzzTime',
    'linter',
    'LintTimeout',
    'CmdsBin',
    'ToolsBin',
    'TestsBin',
    'OutDir',
    'ProtobufDir') | ForEach-Object {
        $scope, $name = $_ -split ':'
        $a = ( -not $name ) ? @{Name = $scope } : @{Name = $name; Scope = $scope }
        Get-Variable @a -ErrorAction SilentlyContinue
    }
}

Task Noop -description 'do nothing' -action {}

################################################################################
#   Helper Functions
################################################################################

function New-LintCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [Parameter(Position = 0, ValueFromRemainingArguments)]
        [AllowEmptyCollection()]
        [string[]]
        $ExtraArgs,

        [string]
        $linter = $linter,

        [string]
        $Timeout = $LintTimeout,

        [switch]
        $NoCall
    )
    Join-Cmd $linter run ('--timeout=' + $Timeout) `
        --max-issues-per-linter=0 `
        --max-same-issues=0 `
        --modules-download-mode=readonly `
        --config=$LintConfig ( $Verbose ? '--verbose' : '') `
        @ExtraArgs | ConvertTo-Command -NoCall:$NoCall
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
    $pre = ( { Assert-GoPath }, { $env:GOWORK = 'off' },
        ($IsTest ? { Set-Location .\test } : {}),
        ('New-SetGoOSCmd' + ( $GoOS ? " -GoOS $GoOS" : '') + ' | MyExec' |
            ConvertTo-Command -NoCall ) ) |
            Join-Scriptblock -NoCall -NoNewline
    $post = ( { New-ResetGoOSCmd | MyExec }, { $env:GOWORK = $null },
        ($IsTest ? { Set-Location .. } : {})) |
        Join-Scriptblock -NoCall -NoNewline
    # we want the command to be created in the scope of the task action, but before
    # it is passed into (My)Exec
    $cmd = $IsTest ? 'New-GoBuildTestCmd' : 'New-GoBuildCmd'
    $cmd = ( "$cmd -Package `"$Package`" -OutDir `"$OutDir`" | MyExec" ) | ConvertTo-Command -NoCall
    $desc = 'Build go ' + ($IsTest ? 'test executable for' : 'package') + " '$Package'"

    Task $Name -alias $Alias -description $desc `
        -preaction $pre `
        -action $cmd `
        -postaction $post

    Write-Output sdf ssss
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
        $go = $go,

        [switch]
        $NoCall
    )
    $tagparam = ''
    if ( $tags ) {
        $tagparam = '-tags=' + ($Tags -join ',')
    }
    Join-Cmd $go build @Flags $tagparam ('-o=' + $OutDir) $Package | ConvertTo-Command -NoCall:$NoCall
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
        $go = $go,

        [switch]
        $NoCall
    )
    $tagparam = ''
    if ( $tags ) {
        $tagparam = '-tags=' + ($Tags -join ',')
    }
    # `go test -c` parses `-o` as the output file, not the directory (different from `go build`)
    $out = (Join-Path $OutDir (Split-Path -Leaf $Package)) + ".test`$(& `"$go`" env GOEXE)"
    Join-Cmd $go test @Flags $tagparam ('-o=' + $out) '-c' $Package | ConvertTo-Command -NoCall:$NoCall
}

function New-SetGoOSCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [ValidateSet('windows', 'linux')]
        [string]
        $GoOS = 'windows',

        [string]
        $go = $go,

        [switch]
        $NoCall
    )
    Join-Cmd $go env -w ('GOOS=' + ($GoOS.ToLower())) | ConvertTo-Command -NoCall:$NoCall
}

function New-ResetGoOSCmd {
    [OutputType([scriptblock])]
    [CmdletBinding()]
    param (
        [string]
        $go = $go,

        [switch]
        $NoCall
    )
    Join-Cmd $go env -u GOOS | ConvertTo-Command -NoCall:$NoCall
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

function Assert-GoPath {
    param (
        [string]
        $go = $go
    )
    Assert ([bool]$go) 'Unable to find go executable'
}

function Disable-TimingReport {
    # disable timing report at the bottom
    # set-variable doesnt find the variable first...
    (Get-Variable notr).Value = $True
}

function MyExec {
    # todo: pass all parameters to Exec
    [CmdletBinding()]
    param(
        [Parameter(Position = 0, ValueFromPipeline, Mandatory)]
        [scriptblock]$cmd
    )
    # creating a new scriptblock and invoking $cmd from in it causes scoping and recursion concerns ....
    Write-Verbose "execing in $(Get-Location):`n$cmd"
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
        $NoCall,

        [switch]
        $NoNewline
    )
    Begin {
        $b = $NoCall ? '' : ('{' + ($NoNewline ? '' : "`n") )
        $first = $True
    }
    Process {
        # foreach-item has weirdness with the `$_` variable
        foreach ( $s in $script ) {
            if ( $s ) {
                if ( $first ) {
                    $first = $false
                } else {
                    $b += $NoNewline ? '; ' : "`n"
                }
                $b += "$s"
            }
        }
    }
    End {
        $b + ( $NoCall ? '' : (($NoNewline ? '' : "`n") + '}') ) |
            ConvertTo-Command -NoCall:$NoCall
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

function Confirm-CommandPath {
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

    if ( -not $p ) {
        'Could not find executable' + (($Name) ? " for $Name" : "`"$Path`"" ) | Write-Warning
        return
    }
    if ( $p -is [System.Array] -and $p.Count -gt 1 ) {
        'Multiple executables found' + (( $Name) ? " for $Name" : '' ) + ': ' + ($p -join ', ') | Write-Warning
        $p = $p[0]
    }


    "Using `"$p`"" + (( $Name) ? " for $Name" : '' ) | Write-Verbose
    $p
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

        [Microsoft.PowerShell.Commands.TestPathType]
        $PathType = 'Any'
    )

    if (Test-Path $Path -PathType $PathType) {
        $p = (Get-Item $Path).FullName
        "Using `"$p`"" + (( $Name) ? " for $Name" : '' ) | Write-Verbose
        return $p
    }

    "Could not find $PathType " + ($Name ? "for $Name " : "at `"$Path`"" ) | Write-Warning
}