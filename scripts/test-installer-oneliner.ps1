#Requires -Version 5.1
<#
.SYNOPSIS
    Runs the Windows one-liner that README.md shows, in Windows PowerShell 5.1.
.DESCRIPTION
    Two checks. First, install.ps1 never takes a PowerShell alias for a program (see
    below). Second, the one-liner works end to end: GitHub serves a release asset as application/octet-stream, and Windows
    PowerShell 5.1 and PowerShell 7 do not hand that to `iex` the same way, so the
    one-liner has to be tested in the shell it names, not only in pwsh. This
    serves install.ps1 from a local server that answers the way GitHub does, and
    runs the one-liner against it in uninstall dry-run mode with an empty install
    folder, so it proves the script was fetched as text and ran, and installs and
    changes nothing.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptDirectory = Split-Path -Parent $MyInvocation.MyCommand.Path

# In Windows PowerShell 5.1 'curl' is an alias of Invoke-WebRequest, with no Source,
# so `& (Get-Command curl).Source` stops the install with "The expression after '&'
# ... not valid". PowerShell 7 has no such alias, which is why only 5.1 showed it.
# Every Get-Command of install.ps1 must therefore ask for an application.
$tokens = $null
$parseErrors = $null
$installAst = [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $scriptDirectory 'install.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -gt 0) { throw "install.ps1 does not parse: $($parseErrors[0].Message)" }
$lookups = $installAst.FindAll({
    param($node)
    $node -is [System.Management.Automation.Language.CommandAst] -and $node.GetCommandName() -eq 'Get-Command'
}, $true)
foreach ($lookup in $lookups) {
    if ($lookup.Extent.Text -notmatch '-CommandType\s+Application') {
        throw "install.ps1 line $($lookup.Extent.StartLineNumber): '$($lookup.Extent.Text)' can find an alias; ask for -CommandType Application"
    }
}
Write-Host "PASS: $($lookups.Count) program lookup(s) of install.ps1 ask for an application." -ForegroundColor Green

# The command of README.md ("Windows (PowerShell)"), with the URL of the release
# asset as {URL}. TestReadmeWindowsOneLinerIsTheOneCITests keeps the two equal.
$OneLiner = 'irm {URL} | iex'

$scriptDir = $scriptDirectory
$python = (Get-Command python -ErrorAction Stop).Source
$work = Join-Path ([System.IO.Path]::GetTempPath()) ('skillstation-oneliner.' + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $work | Out-Null

# A server that sends the file with the content type GitHub uses for an asset.
$serverPy = Join-Path $work 'serve.py'
Set-Content -LiteralPath $serverPy -Encoding ASCII -Value @'
import http.server, sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        with open(sys.argv[2], "rb") as handle:
            body = handle.read()
        self.send_response(200)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Disposition", "attachment; filename=install.ps1")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass

http.server.HTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
'@

$probe = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, 0)
$probe.Start()
$port = $probe.LocalEndpoint.Port
$probe.Stop()

$server = Start-Process -FilePath $python -PassThru -WindowStyle Hidden -ArgumentList @(
    "`"$serverPy`"", "$port", "`"$(Join-Path $scriptDir 'install.ps1')`"")
try {
    $url = "http://127.0.0.1:$port/install.ps1"
    $contentType = $null
    for ($attempt = 0; $attempt -lt 50 -and -not $contentType; $attempt++) {
        try {
            $response = [System.Net.WebRequest]::Create($url).GetResponse()
            $contentType = $response.ContentType
            $response.Close()
        } catch {
            Start-Sleep -Milliseconds 200
        }
    }
    if (-not $contentType) { throw "the test server did not answer at $url" }
    if ($contentType -notlike 'application/octet-stream*') {
        throw "the test server must send install.ps1 as application/octet-stream, as GitHub does, but sent $contentType"
    }

    $installDir = Join-Path $work 'install'
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $env:SKILLSTATION_UNINSTALL = '1'
    $env:SKILLSTATION_DRY_RUN = '1'
    $env:SKILLSTATION_NO_MODIFY_PATH = '1'
    $env:SKILLSTATION_INSTALL_DIR = $installDir

    $command = $OneLiner.Replace('{URL}', $url)
    Write-Host "PowerShell $($PSVersionTable.PSVersion): $command"
    $output = (& powershell.exe -NoProfile -ExecutionPolicy ByPass -Command $command 2>&1 | Out-String)
    $exit = $LASTEXITCODE
    Write-Host $output
    if ($exit -ne 0 -or $output -notmatch 'No managed skillstation installation found') {
        throw "the one-liner did not run install.ps1 (exit $exit)"
    }
    Write-Host 'PASS: the one-liner fetched install.ps1 as text and ran it.' -ForegroundColor Green
} finally {
    if ($server -and -not $server.HasExited) { Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}
