# Installs the latest ugh release for the current user. No admin rights needed.
#   irm https://raw.githubusercontent.com/themrsami/ugh/main/install.ps1 | iex
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$file = "ugh_windows_$arch.zip"
$base = 'https://github.com/themrsami/ugh/releases/latest/download'
$dir = Join-Path $env:LOCALAPPDATA 'ugh'
$tmp = Join-Path $env:TEMP ("ugh-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force -Path $tmp, $dir | Out-Null

try {
  Write-Host "downloading $file"
  Invoke-WebRequest "$base/$file" -OutFile (Join-Path $tmp $file) -UseBasicParsing
  Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

  $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match " $([regex]::Escape($file))$" } | Select-Object -First 1
  $want = if ($line) { ($line -split '\s+')[0] } else { '' }
  $got = (Get-FileHash (Join-Path $tmp $file) -Algorithm SHA256).Hash.ToLower()
  if (-not $want -or $want -ne $got) { throw 'checksum mismatch, not installing' }

  Expand-Archive (Join-Path $tmp $file) -DestinationPath $tmp -Force
  Copy-Item (Join-Path $tmp 'ugh.exe') (Join-Path $dir 'ugh.exe') -Force
} finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $dir) {
  [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $dir), 'User')
  $env:Path += ";$dir"
}

Write-Host "installed $(& (Join-Path $dir 'ugh.exe') --version) to $dir"
Write-Host "next, so a bare 'ugh' re-runs your last command:"
Write-Host "  Add-Content `$PROFILE 'ugh.exe --init powershell | Out-String | Invoke-Expression'"
