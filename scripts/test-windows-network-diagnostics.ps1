$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'diagnose-windows-network.ps1'
if (-not (Test-Path $scriptPath)) { throw 'Diagnostic script is missing' }
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw ($parseErrors | Out-String) }
$functions = $ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $false)
foreach ($function in $functions) { . ([scriptblock]::Create($function.Extent.Text)) }

$shellName = if ($PSVersionTable.PSEdition -eq 'Core') { 'pwsh' } else { 'powershell.exe' }
$shell = Join-Path $PSHOME $shellName
$encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes("[Console]::Out.WriteLine('output value'); [Console]::Error.WriteLine('failure detail'); exit 7"))
$result = Invoke-DiagnosticProcess $shell @('-NoProfile', '-EncodedCommand', $encoded) 15
if ($result.ExitCode -ne 7 -or $result.TimedOut -or $result.Stdout -notmatch 'output value' -or $result.Stderr -notmatch 'failure detail') {
    throw 'Native failure must preserve exit code, stdout and stderr'
}
$encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes('Start-Sleep -Seconds 30'))
$watch = [Diagnostics.Stopwatch]::StartNew()
$result = Invoke-DiagnosticProcess $shell @('-NoProfile', '-EncodedCommand', $encoded) 1
if (-not $result.TimedOut -or $watch.Elapsed.TotalSeconds -gt 10) { throw 'Native probe did not time out promptly' }

$script:report = New-Object System.Text.StringBuilder
Write-DiagnosticSection 'denied' { throw 'access denied fixture' }
Write-DiagnosticSection 'next' { 'still collected' }
$output = $script:report.ToString()
if ($output -notmatch 'access denied fixture' -or $output -notmatch 'still collected') {
    throw 'One failed collector must not prevent later report sections'
}
Write-Host 'PASS: parsing, process errors, timeout and partial report recovery'
