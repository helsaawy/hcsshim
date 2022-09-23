#Requires -RunAsAdministrator

$ErrorActionPreference = 'Stop'

$global:basew = 'E:/release/lsg/0.0.46/outputs/build/images_lcow/lcow/core-image-minimal-lcow.tar'

Remove-Item -Recurse -Force out/rootfs -ErrorAction Ignore
Remove-Item -Recurse -Force deps -ErrorAction Ignore
Remove-Item -Recurse -Force bin/cmd -ErrorAction Ignore

wsl -- make bin/init bin/vsockexec
# $global:base = wsl wslpath -u $basew
# wsl -- make BASE=$base out/delta.tar.gz

$env:GOOS = 'linux'
('cmd/gcs', 'cmd/gcstools', 'cmd/hooks/wait-paths') | ForEach-Object {
    go build -ldflags '-s -w' -o "bin/$_" "./$_"
    Write-Output $_
}
$env:GOOS = $null

New-Item -ItemType Directory -Force out/rootfs/bin/ > $null
New-Item -ItemType Directory -Force out/rootfs/info/ > $null

Copy-Item bin/init out/rootfs/
Copy-Item bin/vsockexec out/rootfs/bin/
Copy-Item bin/cmd/gcs out/rootfs/bin/
Copy-Item bin/cmd/gcstools out/rootfs/bin/
Copy-Item bin/cmd/hooks/wait-paths out/rootfs/bin/
('generichook', 'install-drivers') | ForEach-Object {
    New-Item -ItemType SymbolicLink -Path out/rootfs/bin/$_ -Target 'gcstools' > $null
}
git -C . rev-parse HEAD | Out-File -NoNewline out/rootfs/info/gcs.commit
git -C . rev-parse --abbrev-ref HEAD | Out-File -NoNewline out/rootfs/info/gcs.branch

# manually add newlines to avoid CRLF issues
"#mtree`n/set mode=777 uid=0 gid=0`n" | Out-File -FilePath out/input.mtree -NoNewline
tar.exe -cf - -C out/rootfs --format mtree --options 'mtree:!mode,!uid,!gid' . | ForEach-Object {
    $l = $_
    if ( -not $l -or $l -eq '#mtree' ) {
        return
    }
    if ( $l -match 'type=file' ) {
        $l += ' contents=rootfs/' + (($l -split '\s')[0] -replace '^\.\/', '')
    }
    "$l`n" | Out-File -NoNewline -Append -FilePath out/input.mtree
}
# double chroot to deal with weirdness with paths
tar.exe -zvcf out/delta.win.tar.gz -l --totals -C out '@input.mtree' -C rootfs

tar.exe -czf out/initrd.img --totals --format newc ('@' + $basew) '@out/delta.win.tar.gz'
# tar -tvf out/initrd.img bin/g*

tar.exe -cf out/rootfs.tar --uid 0 --gid 0 --uname root --gname root ('@' + $basew) '@out/delta.win.tar.gz' 2>$null

# go build -o .\bin\cmd .\cmd\tar2ext4
./bin/cmd/tar2ext4.exe -vhd -i ./out/rootfs.tar -o out/rootfs.vhd

Move-Item -Force out/initrd.img 'C:\ContainerPlat\LinuxBootFiles\initrd.img'

# go build -o .\bin\tool .\internal\tools\uvmboot
.\bin\cmd\uvmboot.exe -gcs lcow -fwd-stdout -fwd-stderr -output-handling stdout `
    -boot-files-path 'C:\ContainerPlat\LinuxBootFiles' `
    -root-fs-type vhd -exec 'echo hello from linux'