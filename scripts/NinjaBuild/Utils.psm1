
function Resolve-Command {
    [CmdletBinding()]
    [OutputType([string])]
    param (
        [Parameter(Mandatory)]
        [string]
        $Name,

        [string]
        $Path = ''
    )

    if ( -not $Path ) {
        $Path = (Get-Command $Name).Source 2>$null
    }

    if ( -not $Path ) {
        throw "Could not find executable `"$Name`" on the system."
    }

    if ( -not (Test-Path -Path $Path) ) {
        Write-Warning "Invalid path `"$Path`" to executable `"$Name`"."
        # try again, but search for the command instead
        # hopefully this isnt a stack overflow...
        return Resolve-Command $Name
    }

    return $Path
}

function Resolve-PathError {
    [CmdletBinding()]
    [OutputType([string])]
    param (
        [Parameter(Mandatory)]
        [string]
        $Path,

        [string]
        $Name
    )

    if ( -not $Name ) {
        $Name = 'Path'
    }

    if ( -not $Path ) {
        throw "$Name cannot be an empty path."
    }

    $p = Resolve-Path $Path 2>$null
    if ( -not $p ) {
        throw "Could not resolve $Name (`"$Path`") on the system."
    }

    return $p
}

<#
.SYNOPSIS
Trims strings and returns the non-empty and non-null results.
#>
filter Get-NonEmpty {
    $s = ( $_ -is [string] ) ? $_.Trim() : $_
    if ( $s ) { $s }
}
New-Alias -Name gne -Value Get-NonEmpty

<#
.SYNOPSIS
Escape spaces(' ') and colons (':') within a string (but not '$')
#>
function Format-Path {
    [CmdletBinding()]
    param (
        [Parameter(Mandatory,
            ValueFromPipeline,
            ValueFromPipelineByPropertyName)]
        $Path
    )

    process {
        if ( $Path -and $Path -is [string]) {
            $Path = $Path.Trim().Replace(' ', '$ ').Replace(':', '$:')
        }
        $Path
    }
}
New-Alias -Name fp -Value Format-Path

<#
.SYNOPSIS
Prepends the value with a `$` and wraps it with the specified quotes.
If specified, $Left and $Right take precedence over $quote.
#>
function Format-Variable {
    [CmdletBinding(DefaultParameterSetName = 'Quote')]
    [OutputType([string])]
    param (
        [Parameter(Mandatory, Position = 0,
            ValueFromPipeline,
            ValueFromPipelineByPropertyName)]
        [Alias('v')]
        [string]
        $Value,

        [Alias('b')]
        [switch]
        $Bracket,

        [Parameter(ParameterSetName = 'Quote', Position = 1)]
        [Alias('q')]
        [string]
        $Quote = '',

        [Parameter(ParameterSetName = 'LeftRight', Position = 1)]
        [Alias('l')]
        [string]
        $Left = '',

        [Parameter(ParameterSetName = 'LeftRight', Position = 2)]
        [Alias('r')]
        [string]
        $Right = ''
    )

    process {
        if ( $Bracket ) {
            $Value = "{$Value}"
        }

        switch ($PSCmdlet.ParameterSetName) {
            'Quote' {
                $Left = $Quote
                $Right = $Quote
            }
        }
        "$Left`$$Value$Right"
    }
}
New-Alias -Name fv -Value Format-Variable
