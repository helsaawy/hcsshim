$ErrorActionPreference = 'Stop'
$VerbosePreference = 'Continue'

try {
    $root = Split-Path -Path $PSScriptRoot -Parent
    Push-Location $root

    @('go', 'protoc') | ForEach-Object {
        if ( $null -eq (Get-Command $_ -ErrorAction Ignore) ) {
            throw "executable $_ not in path"
        }
    }

    # Install protobuild and co; rely on tools.go to vendor correct dependencies
    @(
        'github.com/containerd/protobuild',
        'github.com/containerd/protobuild/cmd/go-fix-acronym',
        'github.com/containerd/ttrpc/cmd/protoc-gen-go-ttrpc',
        'google.golang.org/grpc/cmd/protoc-gen-go-grpc',
        'google.golang.org/protobuf/cmd/protoc-gen-go'
    ) | ForEach-Object { go install $_ }

    go list ./... |
        Where-Object { $_ -notlike '*vendor*' } |
        ForEach-Object {
            Write-Verbose "protobuild $_"
            protobuild $_
        }

    # don't have [gogoproto.customname] customization, so update acronyms manually
    # skip vendored files, and protofiles added to ./protobuf/ directory
    Get-ChildItem -Filter *.pb.go -Recurse -Name |
        Where-Object { $_ -notlike 'vendor*' -and $_ -notlike 'protobuf*' } |
        ForEach-Object {
            $p = "$(Join-Path $root $_)".Trim()
            $cmd = "go-fix-acronym -w -a '(Id|Io|Guid|Uuid|Os)$' $p"
            Write-Verbose $cmd
            Invoke-Expression $cmd

            if ( $p -like "$(Join-Path $root 'cmd\containerd-shim-runhcs-v1\stats')*" ) {
                $cmd = "go-fix-acronym -w -a '(Vm|Ns)$' $p"
                Write-Verbose $cmd
                Invoke-Expression $cmd
            }
        }
} catch {
    Pop-Location
    throw $_
}