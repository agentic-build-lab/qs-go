$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$binaryDirectory = Join-Path $projectRoot 'bin'
$binaryPath = Join-Path $binaryDirectory 'qsgo.exe'
$goCommand = Get-Command go -ErrorAction SilentlyContinue
$goExecutable = if ($null -ne $goCommand) {
    $goCommand.Source
} else {
    Join-Path $projectRoot '..\toolchain_complete\go\bin\go.exe'
}

if (-not (Test-Path -LiteralPath $goExecutable -PathType Leaf)) {
    throw 'Go was not found on PATH and the competition-local fallback is absent.'
}

New-Item -ItemType Directory -Force -Path $binaryDirectory | Out-Null
Push-Location $projectRoot
try {
    & $goExecutable build -trimpath -o $binaryPath ./cmd/qsgo
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    Write-Output $binaryPath
} finally {
    Pop-Location
}
