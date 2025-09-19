# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Check for (and add) copywrite header to files.
#
# Based on:
#  https://microsoft.visualstudio.com/HyperVCloud/_git/openvmm?path=%2Fxtask%2Fsrc%2Ftasks%2Ffmt%2Fhouse_rules%2Fcopyright.rs&_a=contents&version=GBmain

# TODO: expand this to .go, .ps1, .py, etc., and run over entire repo

[CmdletBinding(SupportsShouldProcess)]
param (
    [switch]
    $Fix
)

$crateDir = Split-Path -Parent -Resolve $PSScriptRoot
$root = Split-Path -Parent -Resolve $crateDir

Write-Verbose "Root path: $root"
Write-Verbose "Crate: $crateDir"

Import-Module -Verbose:$False (Join-Path $root 'tools\ado')
Import-Module -Verbose:$False (Join-Path $root 'tools\misc')

$headerLines = @(
    'Copyright (c) Microsoft Corporation.',
    'Licensed under the MIT License.'
)

$reg = '(?m)' + (( $headerLines | ForEach-Object { '^[/#\s]*' + [regex]::Escape($_) + '$\n?' }) -join '')
Write-Output $reg

# Difficult to exclude a directory from Get-ChildItem via `-Exclude` flag
# -Filter only takes one pattern, so use -Include
Get-ChildItem -Include '*.rs', '*.toml', '*.ps1' -Recurse -Name -Path $crateDir |
    Where-Object { ($_ -notlike 'target*') } |
    ForEach-Object {
        $file = Join-Path -Resolve $crateDir $_
        Write-ADODebug "Checking file: $file"

        # get the first couple lines (and hope the copyright header is part of it)
        $lines = (Get-Content $file -Head 5 -ErrorAction 'Stop' | ForEach-Object {
                $l = $_.Trim()
                if ( -not [string]::IsNullOrWhiteSpace($l) ) {
                    $l
                }
            }) -join "`n"

        if ( $lines -notmatch $reg ) {
            Write-ADOTaskError -ErrorAction 'Continue' -SourcePath $file "Missing copywrite header in $_"
            $exitcode = 1

            if ( $Fix ) {
                # Split-Path -Extension doesn't work in powershell 5.1
                $ext = [System.IO.Path]::GetExtension($file)
                $comment = switch -Regex ( $ext ) {
                    '^\.rs$' { '//' }
                    '^\.(?:TOML|ps1)$' { '#' }
                    Default { write-ADOError -ErrorAction 'Stop' "Unknown file extension ${_}: $File" }
                }
                $ff = "${file}.fix"

                if ( $PSCmdlet.ShouldProcess($file) ) {
                    $headerLines | ForEach-Object {
                        "${comment} ${_}`n" | Add-Content -Encoding 'UTF8' -NoNewline $ff -ErrorAction 'Stop'
                    }
                    "`n" | Add-Content -Encoding 'UTF8' -NoNewline $ff -ErrorAction 'Stop'

                    Get-Content -Raw $file -ErrorAction 'Stop' | Add-Content -NoNewline $ff -ErrorAction 'Stop'
                    Move-Item -Path $ff -Destination $file -Force -ErrorAction 'Stop' -Verbose
                }
            }
        }
    }

if ( $null -ne $exitcode -and $exitcode -ne 0 ) {
    Set-ADOTaskComplete -Result 'Failed'
}

# skip the extra "PowerShell exited with code '*'." task error in ADO
if ( -not (WithinADO) ) {
    exit $exitcode
}
