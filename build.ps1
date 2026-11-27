# Baut Agent und Server unter Windows. Ergebnis im Ordner dist\
# Voraussetzung: Go 1.24 oder neuer (https://go.dev/dl/)
# Versionsnummer: $env:VERSION, sonst Datei VERSION.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$Version = if ($env:VERSION) { $env:VERSION } else { (Get-Content VERSION -Raw).Trim() }
New-Item -ItemType Directory -Force dist | Out-Null
$env:CGO_ENABLED = '0'

Write-Host "-> Agent $Version (Windows) ..."
$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
go run .\tools\winres -version $Version -o agent\rsrc_windows_amd64.syso
go build -trimpath -ldflags "-H windowsgui -X main.agentVersion=$Version -X main.buildMarker=NRMM-AGENT-VERSION:${Version}:END -X main.defaultServer=$env:DEFAULT_SERVER" -o dist\nicolaj-agent.exe .\agent
Copy-Item dist\nicolaj-agent.exe server\assets\nicolaj-agent.exe -Force

Write-Host "-> Server $Version (Windows, Linux) ..."
$ldf = "-s -w -X main.serverVersion=$Version"
go build -trimpath -ldflags $ldf -o dist\nicolaj-rmm-server.exe .\server
$env:GOOS = 'linux'; $env:GOARCH = 'amd64'
go build -trimpath -ldflags $ldf -o dist\nicolaj-rmm-server-linux-amd64 .\server
$env:GOARCH = 'arm64'
go build -trimpath -ldflags $ldf -o dist\nicolaj-rmm-server-linux-arm64 .\server
Remove-Item Env:GOOS, Env:GOARCH

Write-Host '-> Prüfsummen ...'
$lines = Get-ChildItem dist -File | Where-Object { $_.Name -like 'nicolaj-*' } | ForEach-Object {
  '{0}  {1}' -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower(), $_.Name
}
[IO.File]::WriteAllLines("$PSScriptRoot\dist\SHA256SUMS.txt", $lines)

Write-Host "Fertig ($Version):"
Get-ChildItem dist | Format-Table Name, @{n='MB';e={[math]::Round($_.Length/1MB,1)}}
