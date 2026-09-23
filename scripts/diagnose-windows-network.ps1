[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('off', 'tun-disabled', 'tun-prefer-v4', 'sysproxy')]
    [string]$Label,
    [ValidateRange(1, 65535)]
    [int]$SocksPort = 1080,
    [string]$ClientVersion = 'not supplied',
    [string]$OutputDirectory = (Join-Path ([Environment]::GetFolderPath('Desktop')) 'ITGRay-diagnostics')
)

function Invoke-DiagnosticProcess {
    param([string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds = 20)
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = $FilePath
    $info.Arguments = ($Arguments | ForEach-Object { '"' + $_.Replace('"', '\"') + '"' }) -join ' '
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $info
    try {
        [void]$process.Start()
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000)
        if ($timedOut) { $process.Kill(); $process.WaitForExit() }
        [pscustomobject]@{
            ExitCode = $process.ExitCode
            TimedOut = $timedOut
            Stdout = $stdout.GetAwaiter().GetResult().Trim()
            Stderr = $stderr.GetAwaiter().GetResult().Trim()
        }
    } finally {
        $process.Dispose()
    }
}

function Write-DiagnosticSection {
    param([string]$Title, [scriptblock]$Collect)
    Write-Host "Collecting: $Title"
    [void]$script:report.AppendLine("`r`n=== $Title ($(Get-Date -Format o)) ===")
    try {
        $ErrorActionPreference = 'Stop'
        $content = & $Collect | Out-String -Width 240
        if ([string]::IsNullOrWhiteSpace($content)) { $content = '(no entries)' }
        [void]$script:report.AppendLine($content.TrimEnd())
    } catch {
        [void]$script:report.AppendLine("UNAVAILABLE/ERROR: $($_.Exception.Message)")
    }
}

$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'Run this script on the affected Windows computer.'
}
[void][IO.Directory]::CreateDirectory($OutputDirectory)
$reportPath = Join-Path $OutputDirectory ("{0}-{1}-{2}.txt" -f (Get-Date -Format 'yyyyMMdd-HHmmss-fff'), $Label, $PID)
$script:report = New-Object System.Text.StringBuilder
[void]$script:report.AppendLine("ITG Ray network diagnostics | label=$Label | client=$ClientVersion")
[void]$script:report.AppendLine("PowerShell=$($PSVersionTable.PSVersion) | SOCKS=127.0.0.1:$SocksPort")
[void]$script:report.AppendLine('Label is user supplied; the script does not verify or change VPN state/settings.')
[void]$script:report.AppendLine('Contains network addresses, routes and adapter names. Review before sharing. No automatic upload.')
[void]$script:report.AppendLine('OS-route probes ignore explicit proxies, but still traverse an active TUN.')
[void]$script:report.AppendLine('SOCKS resolves names through the proxy; it does not force the remote destination address family.')
[void]$script:report.AppendLine('IPv6 failures in disabled mode may be expected. This report does not diagnose Windows corruption.')
Write-Host "Report: $reportPath"
Write-Host 'Keep the selected VPN state unchanged until collection finishes (usually 1-3 minutes).'

Write-DiagnosticSection 'Windows and privilege level' {
    Get-CimInstance Win32_OperatingSystem -OperationTimeoutSec 10 | Select-Object Caption, Version, BuildNumber
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    "Administrator=$($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator))"
}
Write-DiagnosticSection 'Adapters' {
    Get-NetAdapter -IncludeHidden | Select-Object ifIndex, Name, InterfaceDescription, Status, LinkSpeed | Format-Table -AutoSize
}
Write-DiagnosticSection 'IPv6 adapter bindings' {
    Get-NetAdapterBinding -Name '*' -ComponentID ms_tcpip6 | Select-Object Name, ComponentID, Enabled | Format-Table -AutoSize
}
Write-DiagnosticSection 'IP addresses' {
    Get-NetIPAddress | Select-Object InterfaceIndex, AddressFamily, IPAddress, PrefixLength, AddressState | Format-Table -AutoSize
}
Write-DiagnosticSection 'Interface metrics and MTU' {
    Get-NetIPInterface | Select-Object InterfaceIndex, InterfaceAlias, AddressFamily, ConnectionState, NlMtu, InterfaceMetric | Format-Table -AutoSize
}
Write-DiagnosticSection 'Active routes' {
    Get-NetRoute -PolicyStore ActiveStore | Sort-Object AddressFamily, DestinationPrefix, RouteMetric |
        Select-Object AddressFamily, DestinationPrefix, NextHop, InterfaceIndex, RouteMetric, Protocol | Format-Table -AutoSize
}
Write-DiagnosticSection 'Persistent routes' {
    Get-NetRoute -PolicyStore PersistentStore | Select-Object DestinationPrefix, NextHop, InterfaceIndex, RouteMetric | Format-Table -AutoSize
}
Write-DiagnosticSection 'DNS servers' {
    Get-DnsClientServerAddress | Select-Object InterfaceIndex, InterfaceAlias, AddressFamily, ServerAddresses | Format-List
}
Write-DiagnosticSection 'Effective NRPT' {
    Get-DnsClientNrptPolicy -Effective | Select-Object Namespace, NameServers, DirectAccessEnabled | Format-List
}
Write-DiagnosticSection 'IPv6 registry policy' {
    $settings = Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters'
    if ($null -eq $settings.DisabledComponents) { 'DisabledComponents: absent (Windows default)' }
    else { 'DisabledComponents: 0x{0:X8}' -f [uint32]$settings.DisabledComponents }
}
Write-DiagnosticSection 'IPv6 prefix policy' {
    Invoke-DiagnosticProcess 'netsh.exe' @('interface', 'ipv6', 'show', 'prefixpolicies') | Format-List
}
Write-DiagnosticSection 'Winsock providers' {
    Invoke-DiagnosticProcess 'netsh.exe' @('winsock', 'show', 'catalog') | Format-List
}
Write-DiagnosticSection 'User proxy presence' {
    $settings = Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    [pscustomobject]@{
        ProxyEnabled = $settings.ProxyEnable
        ProxyServerConfigured = -not [string]::IsNullOrWhiteSpace($settings.ProxyServer)
        PACConfigured = -not [string]::IsNullOrWhiteSpace($settings.AutoConfigURL)
    } | Format-List
}
Write-DiagnosticSection 'Firewall profiles' {
    Get-NetFirewallProfile | Select-Object Name, Enabled, DefaultInboundAction, DefaultOutboundAction | Format-Table -AutoSize
}
Write-DiagnosticSection 'Relevant processes (names only)' {
    Get-Process | Where-Object { $_.ProcessName -match 'itgray|itg.?ray|sing.?box|xray|zapret|winws|windivert|goodbyedpi' } |
        Select-Object ProcessName, Id | Format-Table -AutoSize
}
Write-DiagnosticSection 'Relevant drivers' {
    Get-CimInstance Win32_SystemDriver -OperationTimeoutSec 10 |
        Where-Object { $_.Name -match 'windivert|winws|zapret|goodbyedpi|wintun' -or $_.DisplayName -match 'windivert|zapret|goodbyedpi|wintun' } |
        Select-Object Name, DisplayName, State, StartMode | Format-Table -AutoSize
}
Write-DiagnosticSection 'Relevant services' {
    Get-Service | Where-Object { $_.Name -match 'itgray|zapret|winws|windivert|goodbyedpi' -or $_.DisplayName -match 'ITG Ray|zapret|goodbyedpi' } |
        Select-Object Name, DisplayName, Status | Format-Table -AutoSize
}
Write-DiagnosticSection 'Local SOCKS listener' {
    Get-NetTCPConnection -State Listen -LocalPort $SocksPort | Select-Object LocalAddress, LocalPort, OwningProcess | Format-Table -AutoSize
}

$targets = @('www.youtube.com', 'www.cloudflare.com')
foreach ($target in $targets) {
    foreach ($recordType in @('A', 'AAAA')) {
        Write-DiagnosticSection "DNS $target $recordType" {
            Resolve-DnsName -Name $target -Type $recordType -DnsOnly -NoHostsFile -QuickTimeout |
                Select-Object Name, Type, IPAddress, NameHost, TTL | Format-Table -AutoSize
        }
    }
}
$curl = Get-Command curl.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $curl) {
    [void]$script:report.AppendLine('SKIPPED HTTPS probes: curl.exe is not installed. DNS/route results remain available.')
} else {
    Write-DiagnosticSection 'curl version and supported features' {
        Invoke-DiagnosticProcess $curl.Source @('--disable', '--version') | Format-List
    }
    foreach ($target in $targets) {
        $common = @('--disable', '--silent', '--show-error', '--connect-timeout', '5', '--max-time', '12',
            '--http1.1', '--output', 'NUL', '--write-out', 'http=%{http_code} remote=%{remote_ip} connect=%{time_connect} tls=%{time_appconnect} total=%{time_total}', "https://$target/")
        foreach ($family in @('4', '6')) {
            Write-DiagnosticSection "HTTPS OS-route IPv$family $target" {
                Invoke-DiagnosticProcess $curl.Source ($common + @("-$family", '--noproxy', '*')) | Format-List
            }
        }
        if ($Label -ne 'off') {
            Write-DiagnosticSection "HTTPS SOCKS remote DNS $target" {
                Invoke-DiagnosticProcess $curl.Source ($common + @('--noproxy', 'unused.invalid', '--socks5-hostname', "127.0.0.1:$SocksPort")) | Format-List
            }
        }
    }
}
[void]$script:report.AppendLine("`r`nFinished: $(Get-Date -Format o)")
[IO.File]::WriteAllText($reportPath, $script:report.ToString(), (New-Object Text.UTF8Encoding($true)))
Write-Host "Saved: $reportPath"
