# Install the latest checksummed release to the user's local application directory.
$ErrorActionPreference = 'Stop'
$arch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw 'Unsupported architecture. Install from source with Go 1.27.3+.' }
}
$asset = "pzk_windows_$arch.exe"
$base = 'https://github.com/Hell077/pzk/releases/latest/download'
$dir = if ($env:PZK_INSTALL_DIR) { $env:PZK_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'PortZapKill' }
$temp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $binary = Join-Path $temp $asset
    Invoke-WebRequest "$base/$asset" -OutFile $binary -UseBasicParsing
    $checksumFile = Join-Path $temp 'checksums.txt'
    Invoke-WebRequest "$base/checksums.txt" -OutFile $checksumFile -UseBasicParsing
    $checksums = Get-Content $checksumFile -Raw
    $line = $checksums -split "`n" | Where-Object { $_ -match "\s+$([regex]::Escape($asset))\s*$" } | Select-Object -First 1
    if (-not $line) { throw 'No release checksum found. Nothing installed.' }
    $expected = ($line.Trim() -split '\s+')[0]
    if ((Get-FileHash $binary -Algorithm SHA256).Hash -ne $expected) { throw 'Checksum mismatch. Nothing installed.' }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Copy-Item $binary (Join-Path $dir 'pzk.exe') -Force
    Write-Output "Installed PortZapKill to $dir\pzk.exe"
    if (($env:Path -split ';') -notcontains $dir) { Write-Output "Add this directory to your user PATH: $dir" }
} finally {
    Remove-Item $temp -Recurse -Force
}
