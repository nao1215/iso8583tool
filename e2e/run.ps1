<#
.SYNOPSIS
  Windows bootstrap for the atago end-to-end suite (e2e/atago/*.atago.yaml).

.DESCRIPTION
  The PowerShell twin of e2e/run.sh. It builds iso8583tool and the single-shot
  TCP mock server (e2e/mock) as native .exe binaries, exports the environment
  the specs expect, and runs `atago run` against the real binary.

  A native bootstrap is used on Windows on purpose: running run.sh under Git
  Bash would hand the native binaries MSYS-style paths (/d/a/...) that they
  cannot resolve. Here every path is a native Windows path, so ${env:ISO_EXAMPLES}
  and the built binaries resolve for iso8583tool.exe and atago.exe alike.

  Scenarios that need a POSIX shell are gated with skip: { os: windows } in the
  specs, so they are skipped here rather than failing.

  Environment contract (mirrors run.sh):
    PATH            iso8583tool and iso-mock resolve here
    ISO_EXAMPLES    absolute path to the bundled examples/ fixtures
    REPLY_HEX       hex of the 0810 network-echo response the mock replies with

.PARAMETER Args
  Extra arguments forwarded to `atago run` before the spec directory
  (e.g. --filter send).
#>
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$AtagoArgs
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = (Resolve-Path (Join-Path $scriptDir '..')).Path

if (-not (Get-Command atago -ErrorAction SilentlyContinue)) {
    Write-Error "e2e: atago is not installed. Install it from https://github.com/nao1215/atago (CI uses nao1215/setup-atago)."
    exit 127
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("iso8583tool-e2e-" + [System.Guid]::NewGuid().ToString('N'))
$binDir = Join-Path $tmp 'bin'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
try {
    Write-Host 'e2e: building iso8583tool and the mock server...'
    $toolExe = Join-Path $binDir 'iso8583tool.exe'
    $mockExe = Join-Path $binDir 'iso-mock.exe'
    $env:CGO_ENABLED = '0'
    & go build -o $toolExe (Join-Path $repoRoot 'main.go')
    if ($LASTEXITCODE -ne 0) { throw 'go build iso8583tool failed' }
    & go build -o $mockExe (Join-Path $repoRoot 'e2e/mock')
    if ($LASTEXITCODE -ne 0) { throw 'go build iso-mock failed' }

    # Put the freshly built binaries first on PATH so the specs exercise them.
    $env:PATH = "$binDir;$env:PATH"
    $env:ISO_EXAMPLES = Join-Path $repoRoot 'examples'
    $replyPath = Join-Path $env:ISO_EXAMPLES 'basei/0810-network-echo-response.hex'
    $env:REPLY_HEX = ((Get-Content -Raw $replyPath) -replace '\s', '')

    Write-Host ("e2e: iso8583tool " + (& $toolExe version | Select-Object -First 1))
    # Extra args (e.g. --filter X) go before the path so the flag parser sees them.
    $specDir = Join-Path $scriptDir 'atago'
    & atago run @AtagoArgs $specDir
    exit $LASTEXITCODE
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
