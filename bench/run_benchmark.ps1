<#
.SYNOPSIS
Validates or records a reproducible cross-runtime qs benchmark on Windows.

.DESCRIPTION
Validate mode performs read-only prerequisite, provenance, percentile, and
Working Set polling checks. Record mode requires clean implementation and
upstream worktrees, runs correctness gates before timing, and writes a new
self-contained evidence directory under work/. It never updates the frozen
bench/results.json historical record.

.EXAMPLE
powershell -NoProfile -ExecutionPolicy Bypass -File .\bench\run_benchmark.ps1 -Mode validate

.EXAMPLE
powershell -NoProfile -ExecutionPolicy Bypass -File .\bench\run_benchmark.ps1 -Mode record
#>
[CmdletBinding()]
param(
    [ValidateSet('record', 'validate')]
    [string]$Mode = 'record',

    [string]$OutputDirectory = '',

    [string]$UpstreamRoot = '',

    [string]$GoExecutable = '',

    [string]$NodeExecutable = '',

    [switch]$RunFrozenOracleIntegration
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:expected_upstream_commit = '3a890d4ecd3deb72a45d90be36f4f8c5970467c7'
$script:latency_sample_count = 40
$script:iterations_per_sample = 500
$script:startup_sample_count = 40
$script:working_set_poll_interval_ms = 10
$script:expected_workloads = @('parse_flat_100', 'parse_nested_20', 'stringify_flat_100', 'stringify_nested_20')
$script:expected_checksums = @{
    'parse_flat_100' = 2000000L
    'parse_nested_20' = 20000L
    'stringify_flat_100' = 23580000L
    'stringify_nested_20' = 12780000L
}
$script:utf8_no_bom = New-Object System.Text.UTF8Encoding($false)

function Get-FullPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        [string]$BasePath
    )

    if ([System.IO.Path]::IsPathRooted($Path)) {
        return [System.IO.Path]::GetFullPath($Path)
    }
    return [System.IO.Path]::GetFullPath((Join-Path -Path $BasePath -ChildPath $Path))
}

function ConvertTo-NativeArgument {
    param([Parameter(Mandatory = $true)][AllowEmptyString()][string]$Value)

    if ($Value.Length -eq 0) {
        return '""'
    }
    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    if ($Value.Contains('"')) {
        throw 'Native arguments containing quotation marks are not supported by this runner.'
    }
    if ($Value.EndsWith('\')) {
        throw 'Native arguments ending in a backslash are not supported by this runner.'
    }
    return '"' + $Value + '"'
}

function Invoke-NativeProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [hashtable]$Environment = @{},
        [switch]$CaptureWorkingSet,
        [int]$PollIntervalMilliseconds = 10,
        [int]$TimeoutSeconds = 900
    )

    if ($TimeoutSeconds -le 0) {
        throw 'TimeoutSeconds must be positive.'
    }
    if ($PollIntervalMilliseconds -le 0) {
        throw 'PollIntervalMilliseconds must be positive.'
    }

    $start_info = New-Object System.Diagnostics.ProcessStartInfo
    $start_info.FileName = $FilePath
    $start_info.Arguments = (($Arguments | ForEach-Object { ConvertTo-NativeArgument -Value ([string]$_) }) -join ' ')
    $start_info.WorkingDirectory = $WorkingDirectory
    $start_info.UseShellExecute = $false
    $start_info.CreateNoWindow = $true
    $start_info.RedirectStandardOutput = $true
    $start_info.RedirectStandardError = $true
    foreach ($key in $Environment.Keys) {
        if ($null -eq $Environment[$key]) {
            [void]$start_info.EnvironmentVariables.Remove([string]$key)
        } else {
            $start_info.EnvironmentVariables[[string]$key] = [string]$Environment[$key]
        }
    }

    $native_process = New-Object System.Diagnostics.Process
    $native_process.StartInfo = $start_info
    $working_set_samples = New-Object 'System.Collections.Generic.List[object]'
    $stopwatch = New-Object System.Diagnostics.Stopwatch
    $started = $false
    $timed_out = $false
    $stdout = ''
    $stderr = ''
    $exit_code = -1
    $os_reported_peak = 0L

    try {
        $stopwatch.Start()
        if (-not $native_process.Start()) {
            throw "Failed to start native process: $FilePath"
        }
        $started = $true
        $stdout_task = $native_process.StandardOutput.ReadToEndAsync()
        $stderr_task = $native_process.StandardError.ReadToEndAsync()

        if ($CaptureWorkingSet) {
            while ($true) {
                try {
                    $native_process.Refresh()
                    if (-not $native_process.HasExited) {
                        $working_set_samples.Add([pscustomobject][ordered]@{
                            elapsed_ms = [math]::Round($stopwatch.Elapsed.TotalMilliseconds, 4)
                            working_set_bytes = [int64]$native_process.WorkingSet64
                        })
                    }
                } catch {
                    if (-not $native_process.HasExited) {
                        throw
                    }
                }

                if ($native_process.WaitForExit($PollIntervalMilliseconds)) {
                    break
                }
                if ($stopwatch.Elapsed.TotalSeconds -ge $TimeoutSeconds) {
                    $timed_out = $true
                    $native_process.Kill()
                    $native_process.WaitForExit()
                    break
                }
            }
        } else {
            if (-not $native_process.WaitForExit($TimeoutSeconds * 1000)) {
                $timed_out = $true
                $native_process.Kill()
                $native_process.WaitForExit()
            }
        }

        $native_process.WaitForExit()
        $stopwatch.Stop()
        $stdout = $stdout_task.Result
        $stderr = $stderr_task.Result
        $exit_code = $native_process.ExitCode
        try {
            $os_reported_peak = [int64]$native_process.PeakWorkingSet64
        } catch {
            $os_reported_peak = 0L
        }
    } finally {
        if ($stopwatch.IsRunning) {
            $stopwatch.Stop()
        }
        if ($started -and -not $native_process.HasExited) {
            try {
                $native_process.Kill()
                $native_process.WaitForExit()
            } catch {
                # Preserve the original failure.
            }
        }
        $native_process.Dispose()
    }

    if ($timed_out) {
        throw "Native process timed out after $TimeoutSeconds seconds: $FilePath"
    }

    return [pscustomobject][ordered]@{
        exit_code = $exit_code
        duration_ms = [math]::Round($stopwatch.Elapsed.TotalMilliseconds, 4)
        stdout = $stdout
        stderr = $stderr
        working_set_samples = @($working_set_samples.ToArray())
        os_reported_peak_working_set_bytes = $os_reported_peak
    }
}

function Invoke-NativeText {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [hashtable]$Environment = @{}
    )

    $native_result = Invoke-NativeProcess -FilePath $FilePath -Arguments $Arguments -WorkingDirectory $WorkingDirectory -Environment $Environment -TimeoutSeconds 60
    if ($native_result.exit_code -ne 0) {
        $failure_text = $native_result.stderr.Trim()
        throw "Command failed with exit code $($native_result.exit_code): $FilePath $failure_text"
    }
    return $native_result.stdout.Trim()
}

function Write-Utf8Text {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Text
    )

    $parent_path = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $parent_path -PathType Container)) {
        [void][System.IO.Directory]::CreateDirectory($parent_path)
    }
    [System.IO.File]::WriteAllText($Path, $Text, $script:utf8_no_bom)
}

function Write-JsonFile {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)]$Value
    )

    $json_text = $Value | ConvertTo-Json -Depth 40
    Write-Utf8Text -Path $Path -Text ($json_text + [Environment]::NewLine)
}

function Get-Sha256 {
    param([Parameter(Mandatory = $true)][string]$Path)

    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-TextSha256 {
    param([Parameter(Mandatory = $true)][AllowEmptyString()][string]$Text)

    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($Text)
        return ([System.BitConverter]::ToString($sha256.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha256.Dispose()
    }
}

function Get-ArtifactRecord {
    param(
        [Parameter(Mandatory = $true)][string]$OutputRoot,
        [Parameter(Mandatory = $true)][string]$RelativePath
    )

    $artifact_path = Join-Path -Path $OutputRoot -ChildPath $RelativePath
    $artifact_item = Get-Item -LiteralPath $artifact_path
    return [pscustomobject][ordered]@{
        path = $RelativePath.Replace('\', '/')
        sha256 = Get-Sha256 -Path $artifact_path
        bytes = [int64]$artifact_item.Length
    }
}

function Assert-PrivacySafeText {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Text,
        [Parameter(Mandatory = $true)][string]$Label,
        [Parameter(Mandatory = $true)][string[]]$SensitiveValues
    )

    foreach ($sensitive_value in $SensitiveValues) {
        if ([string]::IsNullOrWhiteSpace($sensitive_value)) {
            continue
        }
        if ($Text.IndexOf($sensitive_value, [System.StringComparison]::OrdinalIgnoreCase) -ge 0) {
            throw "Privacy guard rejected $Label because it contains a local identity or path."
        }
    }

    $credential_patterns = @(
        '(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b',
        '(?i)\bCookie\s*:\s*\S+',
        '\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b',
        '(?i)\bgh[pousr]_[A-Za-z0-9_]{20,}\b',
        '(?i)\bgithub_pat_[A-Za-z0-9_]{20,}\b',
        '(?i)\bsk-[A-Za-z0-9_-]{20,}\b',
        '\bAKIA[0-9A-Z]{16}\b',
        '\bAIza[0-9A-Za-z_-]{30,}\b',
        '(?i)\bxox[baprs]-[A-Za-z0-9-]{16,}\b',
        '(?i)\bBearer\s+[A-Za-z0-9._~+/-]{16,}=*',
        '-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
        '(?i)\b(?:api[_-]?key|access[_-]?token|secret|password)\b\s*[:=]\s*["'']?[A-Za-z0-9_./+=-]{16,}'
    )
    foreach ($credential_pattern in $credential_patterns) {
        if ([regex]::IsMatch($Text, $credential_pattern)) {
            throw "Privacy guard rejected $Label because it matches a credential pattern."
        }
    }
}

function Assert-PublishableEvidencePrivacy {
    param(
        [Parameter(Mandatory = $true)][string]$OutputRoot,
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [string[]]$AdditionalSensitiveValues = @()
    )

    $sensitive_values = New-Object 'System.Collections.Generic.List[string]'
    $sensitive_values.Add($RepositoryRoot)
    $sensitive_values.Add($RepositoryRoot.Replace('\', '/'))
    $sensitive_values.Add($RepositoryRoot.Replace('\', '\\'))
    if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable('USERPROFILE'))) {
        $profile_path = [Environment]::GetEnvironmentVariable('USERPROFILE')
        $sensitive_values.Add($profile_path)
        $sensitive_values.Add($profile_path.Replace('\', '/'))
        $sensitive_values.Add($profile_path.Replace('\', '\\'))
    }
    if (-not [string]::IsNullOrWhiteSpace([Environment]::UserName)) {
        $sensitive_values.Add([Environment]::UserName)
    }
    foreach ($additional_sensitive_value in $AdditionalSensitiveValues) {
        if (-not [string]::IsNullOrWhiteSpace($additional_sensitive_value)) {
            $sensitive_values.Add($additional_sensitive_value)
        }
    }

    $publishable_files = @(Get-ChildItem -LiteralPath $OutputRoot -Recurse -File | Where-Object {
        $_.Extension -in @('.json', '.txt', '.log')
    })
    foreach ($publishable_file in $publishable_files) {
        $publishable_text = [string](Get-Content -LiteralPath $publishable_file.FullName -Raw)
        Assert-PrivacySafeText -Text $publishable_text -Label $publishable_file.Name -SensitiveValues @($sensitive_values.ToArray())
    }
    return $publishable_files.Count
}

function Get-SourceRecord {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$RelativePath
    )

    $source_path = Join-Path -Path $RepositoryRoot -ChildPath $RelativePath
    return [pscustomobject][ordered]@{
        path = $RelativePath.Replace('\', '/')
        sha256 = Get-Sha256 -Path $source_path
    }
}

function Resolve-Executable {
    param(
        [string]$ConfiguredPath,
        [Parameter(Mandatory = $true)][string]$DefaultCommand,
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [string]$PreferredPath = ''
    )

    if (-not [string]::IsNullOrWhiteSpace($ConfiguredPath)) {
        $configured_full_path = Get-FullPath -Path $ConfiguredPath -BasePath $RepositoryRoot
        if (-not (Test-Path -LiteralPath $configured_full_path -PathType Leaf)) {
            throw "Configured executable does not exist: $configured_full_path"
        }
        return $configured_full_path
    }

    if (-not [string]::IsNullOrWhiteSpace($PreferredPath)) {
        $preferred_full_path = Get-FullPath -Path $PreferredPath -BasePath $RepositoryRoot
        if (Test-Path -LiteralPath $preferred_full_path -PathType Leaf) {
            return $preferred_full_path
        }
    }

    $command_info = Get-Command $DefaultCommand -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $command_info) {
        throw "Required executable was not found: $DefaultCommand"
    }
    return [System.IO.Path]::GetFullPath($command_info.Source)
}

function Get-GitContext {
    param(
        [Parameter(Mandatory = $true)][string]$GitExecutablePath,
        [Parameter(Mandatory = $true)][string]$RepositoryRoot
    )

    $reported_root = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', '--show-toplevel') -WorkingDirectory $RepositoryRoot
    $reported_full_path = [System.IO.Path]::GetFullPath($reported_root)
    if (-not $reported_full_path.Equals($RepositoryRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Repository isolation check failed. Expected $RepositoryRoot but Git reported $reported_full_path."
    }

    $status_text = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('status', '--porcelain', '--untracked-files=normal') -WorkingDirectory $RepositoryRoot
    return [pscustomobject][ordered]@{
        commit = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', 'HEAD') -WorkingDirectory $RepositoryRoot
        tree_sha1 = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', 'HEAD^{tree}') -WorkingDirectory $RepositoryRoot
        branch = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', '--abbrev-ref', 'HEAD') -WorkingDirectory $RepositoryRoot
        clean = [string]::IsNullOrWhiteSpace($status_text)
        status = $status_text
    }
}

function Test-FrozenUpstream {
    param(
        [Parameter(Mandatory = $true)][string]$GitExecutablePath,
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$FrozenUpstreamRoot
    )

    $manifest_path = Join-Path -Path $RepositoryRoot -ChildPath 'testdata\oracle\oracle_manifest.json'
    $manifest = Get-Content -LiteralPath $manifest_path -Raw | ConvertFrom-Json
    if ($manifest.schema -ne 'qs_go_oracle_manifest/v1') {
        throw "Unsupported oracle manifest schema: $($manifest.schema)"
    }
    if ($manifest.upstream.commit -ne $script:expected_upstream_commit) {
        throw 'The oracle manifest does not contain the expected frozen upstream commit.'
    }
    if ([int]$manifest.baseline.assertions_passed -ne 1045 -or [int]$manifest.baseline.assertions_failed -ne 0) {
        throw 'The oracle manifest does not contain the expected 1045/1045 baseline.'
    }

    $upstream_git_root = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', '--show-toplevel') -WorkingDirectory $FrozenUpstreamRoot
    $upstream_git_full_path = [System.IO.Path]::GetFullPath($upstream_git_root)
    if (-not $upstream_git_full_path.Equals($FrozenUpstreamRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'The configured upstream path is not the root of the expected isolated Git checkout.'
    }

    $upstream_commit = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', 'HEAD') -WorkingDirectory $FrozenUpstreamRoot
    if ($upstream_commit -ne $manifest.upstream.commit) {
        throw "Frozen upstream commit mismatch. Expected $($manifest.upstream.commit), got $upstream_commit."
    }
    $upstream_status = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('status', '--porcelain', '--untracked-files=no') -WorkingDirectory $FrozenUpstreamRoot
    if (-not [string]::IsNullOrWhiteSpace($upstream_status)) {
        throw 'The frozen upstream checkout has tracked modifications.'
    }

    $test_tree = Invoke-NativeText -FilePath $GitExecutablePath -Arguments @('rev-parse', 'HEAD:test') -WorkingDirectory $FrozenUpstreamRoot
    if ($test_tree -ne $manifest.upstream.test_tree_sha1) {
        throw "Frozen upstream test-tree mismatch. Expected $($manifest.upstream.test_tree_sha1), got $test_tree."
    }

    $upstream_prefix = $FrozenUpstreamRoot.TrimEnd('\') + '\'
    foreach ($test_entry in @($manifest.tests)) {
        $test_relative_path = ([string]$test_entry.path).Replace('/', '\')
        $test_full_path = [System.IO.Path]::GetFullPath((Join-Path -Path $FrozenUpstreamRoot -ChildPath $test_relative_path))
        if (-not $test_full_path.StartsWith($upstream_prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Oracle manifest path escapes the upstream checkout: $test_relative_path"
        }
        if (-not (Test-Path -LiteralPath $test_full_path -PathType Leaf)) {
            throw "Oracle manifest file is missing: $test_relative_path"
        }
        $actual_hash = Get-Sha256 -Path $test_full_path
        if ($actual_hash -ne ([string]$test_entry.sha256).ToLowerInvariant()) {
            throw "Oracle manifest SHA-256 mismatch: $test_relative_path"
        }
    }

    $tape_path = Join-Path -Path $FrozenUpstreamRoot -ChildPath 'node_modules\tape\bin\tape'
    if (-not (Test-Path -LiteralPath $tape_path -PathType Leaf)) {
        throw 'The frozen upstream Tape executable is missing; install the frozen dependency tree before recording.'
    }

    $upstream_root_identity = $FrozenUpstreamRoot.TrimEnd('\').ToLowerInvariant()
    return [pscustomobject][ordered]@{
        root_path = $FrozenUpstreamRoot
        root_sha256 = Get-TextSha256 -Text $upstream_root_identity
        commit = $upstream_commit
        describe = [string]$manifest.upstream.describe
        test_tree_sha1 = $test_tree
        manifest_sha256 = Get-Sha256 -Path $manifest_path
        tape_relative_path = 'node_modules/tape/bin/tape'
        baseline_assertions_passed = [int]$manifest.baseline.assertions_passed
        baseline_assertions_failed = [int]$manifest.baseline.assertions_failed
    }
}

function Test-FuzzReport {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)]$UpstreamContext
    )

    $fuzz_report_path = Join-Path -Path $RepositoryRoot -ChildPath 'fuzz\report.json'
    if (-not (Test-Path -LiteralPath $fuzz_report_path -PathType Leaf)) {
        throw 'The frozen differential fuzz report is missing: fuzz/report.json'
    }
    $fuzz_report = Get-Content -LiteralPath $fuzz_report_path -Raw | ConvertFrom-Json
    if ([string]$fuzz_report.schema -ne 'qs_go_differential_report/v1') {
        throw "Unexpected differential fuzz report schema: $($fuzz_report.schema)"
    }
    if ([int64]$fuzz_report.duration_ms -lt 60000 -or [int64]$fuzz_report.total_cases -le 0) {
        throw 'The differential fuzz report does not contain the required completed run.'
    }
    if (([int64]$fuzz_report.parse_cases + [int64]$fuzz_report.stringify_cases) -ne [int64]$fuzz_report.total_cases) {
        throw 'The differential fuzz report case totals are inconsistent.'
    }
    if ([int]$fuzz_report.mismatches -ne 0 -or [int]$fuzz_report.oracle_errors -ne 0 -or [int]$fuzz_report.go_errors -ne 0) {
        throw 'The differential fuzz report contains a mismatch or execution error.'
    }
    if ([string]$fuzz_report.upstream_commit -ne [string]$UpstreamContext.commit -or
        [string]$fuzz_report.upstream_test_tree -ne [string]$UpstreamContext.test_tree_sha1) {
        throw 'The differential fuzz report is not bound to the verified upstream identity.'
    }
    if ([string]::IsNullOrWhiteSpace([string]$fuzz_report.seed_hex) -or
        [string]::IsNullOrWhiteSpace([string]$fuzz_report.started_at) -or
        [string]::IsNullOrWhiteSpace([string]$fuzz_report.finished_at)) {
        throw 'The differential fuzz report is missing its seed or timestamps.'
    }

    return [pscustomobject][ordered]@{
        path = 'fuzz/report.json'
        sha256 = Get-Sha256 -Path $fuzz_report_path
        schema = [string]$fuzz_report.schema
        seed_hex = [string]$fuzz_report.seed_hex
        duration_ms = [int64]$fuzz_report.duration_ms
        total_cases = [int64]$fuzz_report.total_cases
        parse_cases = [int64]$fuzz_report.parse_cases
        stringify_cases = [int64]$fuzz_report.stringify_cases
        upstream_commit = [string]$fuzz_report.upstream_commit
        upstream_test_tree = [string]$fuzz_report.upstream_test_tree
    }
}

function Get-HostMetadata {
    $processor_records = @(Get-CimInstance -ClassName Win32_Processor)
    $computer_record = Get-CimInstance -ClassName Win32_ComputerSystem
    $os_record = Get-CimInstance -ClassName Win32_OperatingSystem
    if ($processor_records.Count -eq 0 -or $null -eq $computer_record -or $null -eq $os_record) {
        throw 'Windows host metadata is unavailable through CIM.'
    }

    $physical_cores = 0
    $logical_cores = 0
    $processor_names = @()
    foreach ($processor_record in $processor_records) {
        $physical_cores += [int]$processor_record.NumberOfCores
        $logical_cores += [int]$processor_record.NumberOfLogicalProcessors
        $processor_names += $processor_record.Name.ToString().Trim()
    }

    return [pscustomobject][ordered]@{
        os_caption = [string]$os_record.Caption
        os_version = [string]$os_record.Version
        os_build_number = [string]$os_record.BuildNumber
        os_architecture = [string]$os_record.OSArchitecture
        cpu_models = @($processor_names)
        physical_cores = $physical_cores
        logical_cores = $logical_cores
        total_physical_memory_bytes = [uint64]$computer_record.TotalPhysicalMemory
        total_visible_memory_bytes = ([uint64]$os_record.TotalVisibleMemorySize * 1024)
        available_physical_memory_bytes = ([uint64]$os_record.FreePhysicalMemory * 1024)
        observed_at = [DateTime]::UtcNow.ToString('o')
    }
}

function Get-NodeBenchmarkEnvironment {
    param([Parameter(Mandatory = $true)]$UpstreamContext)

    return @{
        'NODE_OPTIONS' = $null
        'NODE_PATH' = $null
        'NODE_V8_COVERAGE' = $null
        'NODE_COMPILE_CACHE' = $null
        'NODE_DISABLE_COMPILE_CACHE' = $null
        'UV_THREADPOOL_SIZE' = '4'
        'QSGO_BENCH_UPSTREAM_ROOT' = [string]$UpstreamContext.root_path
        'QSGO_BENCH_UPSTREAM_ROOT_SHA256' = [string]$UpstreamContext.root_sha256
    }
}

function Get-GoBenchmarkEnvironment {
    param([Parameter(Mandatory = $true)][int]$LogicalCores)

    if ($LogicalCores -le 0) {
        throw 'Logical core count must be positive before constructing the Go benchmark environment.'
    }
    return @{
        'GOMAXPROCS' = $LogicalCores.ToString([System.Globalization.CultureInfo]::InvariantCulture)
        'GOGC' = '100'
        'GOMEMLIMIT' = 'off'
        'GODEBUG' = $null
        'GOENV' = 'off'
        'GOWORK' = 'off'
        'GOFLAGS' = ''
        'GOOS' = 'windows'
        'GOARCH' = 'amd64'
        'GOAMD64' = 'v1'
        'GOEXPERIMENT' = ''
    }
}

function Merge-Environment {
    param([Parameter(Mandatory = $true)][hashtable[]]$Sources)

    $merged = @{}
    foreach ($source in $Sources) {
        foreach ($key in $source.Keys) {
            $merged[[string]$key] = $source[$key]
        }
    }
    return $merged
}

function Get-PublicEnvironmentPolicy {
    param(
        [Parameter(Mandatory = $true)][int]$LogicalCores,
        [Parameter(Mandatory = $true)]$UpstreamContext
    )

    return [pscustomobject][ordered]@{
        node = [pscustomobject][ordered]@{
            NODE_OPTIONS = $null
            NODE_PATH = $null
            NODE_V8_COVERAGE = $null
            NODE_COMPILE_CACHE = $null
            NODE_DISABLE_COMPILE_CACHE = $null
            UV_THREADPOOL_SIZE = '4'
            QSGO_BENCH_UPSTREAM_ROOT = 'verified_checkout_path_not_recorded'
            QSGO_BENCH_UPSTREAM_ROOT_SHA256 = 'private_path_identity_check_not_recorded'
        }
        go = [pscustomobject][ordered]@{
            GOMAXPROCS = $LogicalCores.ToString([System.Globalization.CultureInfo]::InvariantCulture)
            GOGC = '100'
            GOMEMLIMIT = 'off'
            GODEBUG = $null
            GOENV = 'off'
            GOWORK = 'off'
            GOFLAGS = ''
            GOOS = 'windows'
            GOARCH = 'amd64'
            GOAMD64 = 'v1'
            GOEXPERIMENT = ''
        }
        null_means_removed_from_child_environment = $true
    }
}

function Get-Distribution {
    param([Parameter(Mandatory = $true)][object[]]$Values)

    if ($Values.Count -eq 0) {
        throw 'Cannot summarize an empty sample set.'
    }
    $sorted_values = @($Values | ForEach-Object { [double]$_ } | Sort-Object)
    $median_index = [int][math]::Floor((($sorted_values.Count - 1) * 0.50) + 0.5)
    $p95_index = [int][math]::Floor((($sorted_values.Count - 1) * 0.95) + 0.5)
    $p99_index = [int][math]::Floor((($sorted_values.Count - 1) * 0.99) + 0.5)
    return [pscustomobject][ordered]@{
        sample_count = $sorted_values.Count
        median = [double]$sorted_values[$median_index]
        p95 = [double]$sorted_values[$p95_index]
        p99 = [double]$sorted_values[$p99_index]
        minimum = [double]$sorted_values[0]
        maximum = [double]$sorted_values[$sorted_values.Count - 1]
    }
}

function Assert-NearlyEqual {
    param(
        [Parameter(Mandatory = $true)][double]$Actual,
        [Parameter(Mandatory = $true)][double]$Expected,
        [Parameter(Mandatory = $true)][string]$Label
    )

    if ([math]::Abs($Actual - $Expected) -gt 0.000001) {
        throw "$Label is inconsistent with its raw samples. Actual=$Actual Expected=$Expected"
    }
}

function Assert-FinitePositive {
    param(
        [Parameter(Mandatory = $true)][double]$Value,
        [Parameter(Mandatory = $true)][string]$Label
    )

    if ([double]::IsNaN($Value) -or [double]::IsInfinity($Value) -or $Value -le 0.0) {
        throw "$Label must be a finite positive number."
    }
}

function Get-WorkloadSummaries {
    param(
        [Parameter(Mandatory = $true)]$LatencyReport,
        [Parameter(Mandatory = $true)][string]$ExpectedSchema
    )

    if ([string]$LatencyReport.schema -ne $ExpectedSchema) {
        throw "Latency report schema mismatch. Expected $ExpectedSchema, got $($LatencyReport.schema)."
    }
    if ([string]::IsNullOrWhiteSpace([string]$LatencyReport.runtime)) {
        throw 'Latency report runtime is missing.'
    }
    if ([int]$LatencyReport.samples -ne $script:latency_sample_count) {
        throw "Latency report sample count is not $($script:latency_sample_count)."
    }
    if ([int]$LatencyReport.iterations_per_sample -ne $script:iterations_per_sample) {
        throw "Latency report iteration count is not $($script:iterations_per_sample)."
    }

    $reported_workloads = @($LatencyReport.workloads)
    if ($reported_workloads.Count -ne $script:expected_workloads.Count) {
        throw "Latency report must contain exactly $($script:expected_workloads.Count) workloads."
    }

    $summaries = [ordered]@{}
    for ($workload_index = 0; $workload_index -lt $script:expected_workloads.Count; $workload_index++) {
        $workload = $reported_workloads[$workload_index]
        $workload_name = [string]$workload.name
        $expected_workload_name = $script:expected_workloads[$workload_index]
        if ($workload_name -ne $expected_workload_name) {
            throw "Unexpected workload at index $workload_index. Expected $expected_workload_name, got $workload_name."
        }
        if ($summaries.Contains($workload_name)) {
            throw "Duplicate workload in latency report: $workload_name"
        }
        $raw_samples = @($workload.samples_ns_per_op)
        if ($raw_samples.Count -ne $script:latency_sample_count) {
            throw "Workload $workload_name does not contain $($script:latency_sample_count) raw samples."
        }
        for ($sample_index = 0; $sample_index -lt $raw_samples.Count; $sample_index++) {
            Assert-FinitePositive -Value ([double]$raw_samples[$sample_index]) -Label "$workload_name sample $($sample_index + 1)"
        }
        $distribution = Get-Distribution -Values $raw_samples
        Assert-FinitePositive -Value ([double]$workload.median_ns_per_op) -Label "$workload_name median"
        Assert-FinitePositive -Value ([double]$workload.p95_ns_per_op) -Label "$workload_name p95"
        Assert-FinitePositive -Value ([double]$workload.p99_ns_per_op) -Label "$workload_name p99"
        Assert-FinitePositive -Value ([double]$workload.minimum_ns_per_op) -Label "$workload_name minimum"
        Assert-FinitePositive -Value ([double]$workload.maximum_ns_per_op) -Label "$workload_name maximum"
        Assert-NearlyEqual -Actual ([double]$workload.median_ns_per_op) -Expected $distribution.median -Label "$workload_name median"
        Assert-NearlyEqual -Actual ([double]$workload.p95_ns_per_op) -Expected $distribution.p95 -Label "$workload_name p95"
        Assert-NearlyEqual -Actual ([double]$workload.p99_ns_per_op) -Expected $distribution.p99 -Label "$workload_name p99"
        Assert-NearlyEqual -Actual ([double]$workload.minimum_ns_per_op) -Expected $distribution.minimum -Label "$workload_name minimum"
        Assert-NearlyEqual -Actual ([double]$workload.maximum_ns_per_op) -Expected $distribution.maximum -Label "$workload_name maximum"
        if ([int]$workload.operations -ne ($script:latency_sample_count * $script:iterations_per_sample)) {
            throw "Workload $workload_name reports an unexpected operation count."
        }
        if ([int64]$workload.checksum -ne [int64]$script:expected_checksums[$workload_name]) {
            throw "Workload $workload_name reports an unexpected checksum."
        }

        $summaries[$workload_name] = [pscustomobject][ordered]@{
            sample_count = $distribution.sample_count
            iterations_per_sample = $script:iterations_per_sample
            median_ns_per_op = $distribution.median
            p95_ns_per_op = $distribution.p95
            p99_ns_per_op = $distribution.p99
            minimum_ns_per_op = $distribution.minimum
            maximum_ns_per_op = $distribution.maximum
            median_operations_per_second = [math]::Round((1000000000.0 / $distribution.median), 3)
            checksum = [int64]$workload.checksum
        }
    }
    return $summaries
}

function Get-PolledPeakWorkingSet {
    param([Parameter(Mandatory = $true)][object[]]$Samples)

    if ($Samples.Count -eq 0) {
        throw 'No Working Set samples were captured.'
    }
    return [int64](($Samples | Measure-Object -Property working_set_bytes -Maximum).Maximum)
}

function Invoke-LoggedCommand {
    param(
        [Parameter(Mandatory = $true)][string]$Label,
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][hashtable]$Environment,
        [Parameter(Mandatory = $true)][string]$StdoutPath,
        [Parameter(Mandatory = $true)][string]$StderrPath,
        [int]$TimeoutSeconds = 900
    )

    $command_result = Invoke-NativeProcess -FilePath $FilePath -Arguments $Arguments -WorkingDirectory $WorkingDirectory -Environment $Environment -TimeoutSeconds $TimeoutSeconds
    Write-Utf8Text -Path $StdoutPath -Text $command_result.stdout
    Write-Utf8Text -Path $StderrPath -Text $command_result.stderr
    if ($command_result.exit_code -ne 0) {
        throw "$Label failed with exit code $($command_result.exit_code)."
    }
    return $command_result
}

function Measure-ColdStarts {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeName,
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][hashtable]$Environment,
        [Parameter(Mandatory = $true)][string]$RawOutputPath,
        [string]$ExpectedStdout = ''
    )

    $startup_samples = New-Object 'System.Collections.Generic.List[object]'
    for ($sample_index = 0; $sample_index -lt $script:startup_sample_count; $sample_index++) {
        $sample_started_at = [DateTime]::UtcNow.ToString('o')
        $startup_result = Invoke-NativeProcess -FilePath $FilePath -Arguments $Arguments -WorkingDirectory $WorkingDirectory -Environment $Environment -TimeoutSeconds 30
        $startup_record = [pscustomobject][ordered]@{
            sample = $sample_index + 1
            started_at = $sample_started_at
            duration_ms = [double]$startup_result.duration_ms
            exit_code = [int]$startup_result.exit_code
            stdout = $startup_result.stdout.Trim()
            stderr = $startup_result.stderr.Trim()
        }
        Assert-FinitePositive -Value ([double]$startup_record.duration_ms) -Label "$RuntimeName cold-start sample $($sample_index + 1)"
        $startup_samples.Add($startup_record)
        Write-JsonFile -Path $RawOutputPath -Value @($startup_samples.ToArray())

        if ($startup_result.exit_code -ne 0) {
            throw "$RuntimeName cold-start sample $($sample_index + 1) failed with exit code $($startup_result.exit_code)."
        }
        if (-not [string]::IsNullOrEmpty($ExpectedStdout) -and $startup_record.stdout -ne $ExpectedStdout) {
            throw "$RuntimeName cold-start sample $($sample_index + 1) returned unexpected output."
        }
    }

    $durations = @($startup_samples.ToArray() | ForEach-Object { [double]$_.duration_ms })
    $distribution = Get-Distribution -Values $durations
    return [pscustomobject][ordered]@{
        sample_count = $distribution.sample_count
        median_ms = $distribution.median
        p95_ms = $distribution.p95
        p99_ms = $distribution.p99
        minimum_ms = $distribution.minimum
        maximum_ms = $distribution.maximum
    }
}

function Get-OutputRoot {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [string]$ConfiguredOutputDirectory
    )

    $work_root = [System.IO.Path]::GetFullPath((Join-Path -Path $RepositoryRoot -ChildPath 'work'))
    if ([string]::IsNullOrWhiteSpace($ConfiguredOutputDirectory)) {
        $timestamp = [DateTime]::UtcNow.ToString('yyyyMMdd_HHmmssZ')
        $candidate = Join-Path -Path $work_root -ChildPath ("benchmark_run_$timestamp")
    } else {
        $candidate = Get-FullPath -Path $ConfiguredOutputDirectory -BasePath $RepositoryRoot
    }
    $candidate = [System.IO.Path]::GetFullPath($candidate)
    $work_prefix = $work_root.TrimEnd('\') + '\'
    if (-not $candidate.StartsWith($work_prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'Benchmark output must be a child of this repository work directory.'
    }
    if (Test-Path -LiteralPath $candidate) {
        throw "Benchmark output already exists; refusing to overwrite it: $candidate"
    }
    return $candidate
}

function Invoke-Validation {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)]$RepositoryContext,
        [Parameter(Mandatory = $true)]$UpstreamContext,
        [Parameter(Mandatory = $true)][string]$GoExecutablePath,
        [Parameter(Mandatory = $true)][string]$NodeExecutablePath
    )

    $go_source = Get-Content -LiteralPath (Join-Path $RepositoryRoot 'cmd\benchmark_qsgo\main.go') -Raw
    $node_source = Get-Content -LiteralPath (Join-Path $RepositoryRoot 'bench\original_benchmark.cjs') -Raw
    if ($go_source -notmatch 'samples_ns_per_op' -or $node_source -notmatch 'samples_ns_per_op') {
        throw 'Both latency harnesses must emit samples_ns_per_op.'
    }

    $historical_results_path = Join-Path $RepositoryRoot 'bench\results.json'
    $historical_results = Get-Content -LiteralPath $historical_results_path -Raw | ConvertFrom-Json
    if ($historical_results.schema -ne 'qs_go_comparative_benchmark/v1') {
        throw 'The frozen historical benchmark result has an unexpected schema.'
    }
    if ([int]$historical_results.method.latency_samples -ne $script:latency_sample_count -or
        [int]$historical_results.method.iterations_per_sample -ne $script:iterations_per_sample -or
        [int]$historical_results.method.startup_samples -ne $script:startup_sample_count -or
        [int]$historical_results.method.peak_working_set_poll_interval_ms -ne $script:working_set_poll_interval_ms) {
        throw 'The frozen historical benchmark method does not match the runner constants.'
    }

    $percentile_probe = Get-Distribution -Values @((1..40) | ForEach-Object { [double]$_ })
    if ($percentile_probe.median -ne 21.0 -or $percentile_probe.p95 -ne 38.0 -or $percentile_probe.p99 -ne 40.0) {
        throw 'The percentile implementation failed its deterministic validation probe.'
    }
    $validation_host = Get-HostMetadata
    $node_environment = Get-NodeBenchmarkEnvironment -UpstreamContext $UpstreamContext
    $go_runtime_environment = Get-GoBenchmarkEnvironment -LogicalCores $validation_host.logical_cores
    $environment_policy = Get-PublicEnvironmentPolicy -LogicalCores $validation_host.logical_cores -UpstreamContext $UpstreamContext
    $fuzz_context = Test-FuzzReport -RepositoryRoot $RepositoryRoot -UpstreamContext $UpstreamContext
    $working_set_probe = Invoke-NativeProcess -FilePath $NodeExecutablePath -Arguments @('bench/startup_probe.cjs') -WorkingDirectory $RepositoryRoot -Environment $node_environment -CaptureWorkingSet -PollIntervalMilliseconds $script:working_set_poll_interval_ms -TimeoutSeconds 30
    if ($working_set_probe.exit_code -ne 0 -or @($working_set_probe.working_set_samples).Count -eq 0) {
        throw 'The read-only Working Set polling probe failed.'
    }
    Assert-PrivacySafeText -Text 'public benchmark evidence' -Label 'benign validation probe' -SensitiveValues @($RepositoryRoot)
    $privacy_probe_rejected = $false
    try {
        Assert-PrivacySafeText -Text ('ghp_' + (('A' * 24) -join '')) -Label 'credential validation probe' -SensitiveValues @($RepositoryRoot)
    } catch {
        $privacy_probe_rejected = $true
    }
    if (-not $privacy_probe_rejected) {
        throw 'The privacy guard failed its deterministic credential probe.'
    }
    foreach ($privacy_probe_case in @(
        'judge@example.invalid',
        'Cookie: session=not_a_real_cookie_value',
        'eyJhbGciOiJub25lIn0.eyJzdWIiOiJwcm9iZSJ9.c2lnbmF0dXJlX3Byb2Jl'
    )) {
        $privacy_probe_rejected = $false
        try {
            Assert-PrivacySafeText -Text $privacy_probe_case -Label 'privacy validation probe' -SensitiveValues @($RepositoryRoot)
        } catch {
            $privacy_probe_rejected = $true
        }
        if (-not $privacy_probe_rejected) {
            throw 'The privacy guard failed a deterministic email, Cookie, or JWT probe.'
        }
    }
    $escaped_path_probe_rejected = $false
    try {
        $escaped_path_probe = ([pscustomobject]@{ path = $RepositoryRoot } | ConvertTo-Json)
        Assert-PrivacySafeText -Text $escaped_path_probe -Label 'JSON-escaped path validation probe' -SensitiveValues @(
            $RepositoryRoot,
            $RepositoryRoot.Replace('\', '\\')
        )
    } catch {
        $escaped_path_probe_rejected = $true
    }
    if (-not $escaped_path_probe_rejected) {
        throw 'The privacy guard failed its deterministic JSON-escaped path probe.'
    }

    $validation_record = [pscustomobject][ordered]@{
        schema = 'qs_go_benchmark_runner_validation/v1'
        status = 'valid'
        validation_scope = 'prerequisites_only; correctness gates and benchmark timing are not executed'
        repository_commit = $RepositoryContext.commit
        repository_tree_sha1 = $RepositoryContext.tree_sha1
        repository_branch = $RepositoryContext.branch
        repository_clean = $RepositoryContext.clean
        upstream_commit = $UpstreamContext.commit
        upstream_test_tree_sha1 = $UpstreamContext.test_tree_sha1
        oracle_manifest_sha256 = $UpstreamContext.manifest_sha256
        go_version = Invoke-NativeText -FilePath $GoExecutablePath -Arguments @('version') -WorkingDirectory $RepositoryRoot -Environment $go_runtime_environment
        node_version = Invoke-NativeText -FilePath $NodeExecutablePath -Arguments @('--version') -WorkingDirectory $RepositoryRoot -Environment $node_environment
        historical_results_sha256 = Get-Sha256 -Path $historical_results_path
        host = $validation_host
        normalized_benchmark_environment = $environment_policy
        differential_fuzz_report = $fuzz_context
        frozen_oracle_integration_requested_for_record_mode = [bool]$RunFrozenOracleIntegration
        mode_writes_files = $false
        working_set_probe_samples = @($working_set_probe.working_set_samples).Count
        privacy_guard_probe = 'passed'
    }
    [Console]::Out.WriteLine(($validation_record | ConvertTo-Json -Depth 10))
}

function Invoke-Record {
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$FrozenUpstreamRoot,
        [Parameter(Mandatory = $true)]$RepositoryContext,
        [Parameter(Mandatory = $true)]$UpstreamContext,
        [Parameter(Mandatory = $true)][string]$GitExecutablePath,
        [Parameter(Mandatory = $true)][string]$GoExecutablePath,
        [Parameter(Mandatory = $true)][string]$NodeExecutablePath,
        [Parameter(Mandatory = $true)][string]$BenchmarkOutputRoot
    )

    if (-not $RepositoryContext.clean) {
        throw 'Record mode requires a clean implementation repository. Commit or discard tracked and untracked changes first.'
    }
    if ($RunFrozenOracleIntegration) {
        $default_oracle_upstream = [System.IO.Path]::GetFullPath((Join-Path -Path $RepositoryRoot -ChildPath '..\upstream_qs'))
        if (-not $FrozenUpstreamRoot.Equals($default_oracle_upstream, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw 'The opt-in frozen oracle integration gate requires the verified checkout at ../upstream_qs.'
        }
    }

    [void][System.IO.Directory]::CreateDirectory($BenchmarkOutputRoot)
    foreach ($relative_directory in @('artifacts', 'correctness', 'evidence', 'raw', 'runtime', 'runtime\go_cache', 'runtime\go_tmp')) {
        [void][System.IO.Directory]::CreateDirectory((Join-Path -Path $BenchmarkOutputRoot -ChildPath $relative_directory))
    }

    $record_started_at = [DateTime]::UtcNow.ToString('o')
    $historical_results_path = Join-Path $RepositoryRoot 'bench\results.json'
    $historical_results_hash_before = Get-Sha256 -Path $historical_results_path
    $source_paths = @(
        'bench\run_benchmark.ps1',
        'bench\original_benchmark.cjs',
        'bench\startup_probe.cjs',
        'cmd\benchmark_qsgo\main.go',
        'cmd\qsgo\main.go',
        'fuzz\report.json',
        'go.mod',
        'testdata\oracle\oracle_manifest.json'
    )
    $source_records_before = @($source_paths | ForEach-Object { Get-SourceRecord -RepositoryRoot $RepositoryRoot -RelativePath $_ })
    $host_before_correctness = Get-HostMetadata
    $fuzz_context = Test-FuzzReport -RepositoryRoot $RepositoryRoot -UpstreamContext $UpstreamContext
    $node_environment = Get-NodeBenchmarkEnvironment -UpstreamContext $UpstreamContext
    $go_runtime_environment = Get-GoBenchmarkEnvironment -LogicalCores $host_before_correctness.logical_cores
    $environment_policy = Get-PublicEnvironmentPolicy -LogicalCores $host_before_correctness.logical_cores -UpstreamContext $UpstreamContext
    $go_version = Invoke-NativeText -FilePath $GoExecutablePath -Arguments @('version') -WorkingDirectory $RepositoryRoot -Environment $go_runtime_environment
    $node_version = Invoke-NativeText -FilePath $NodeExecutablePath -Arguments @('--version') -WorkingDirectory $RepositoryRoot -Environment $node_environment
    Write-JsonFile -Path (Join-Path $BenchmarkOutputRoot 'runtime\record_context.json') -Value ([pscustomobject][ordered]@{
        schema = 'qs_go_benchmark_record_context/v1'
        recorded_at = $record_started_at
        repository_commit = $RepositoryContext.commit
        repository_tree_sha1 = $RepositoryContext.tree_sha1
        repository_branch = $RepositoryContext.branch
        repository_clean = $RepositoryContext.clean
        upstream_commit = $UpstreamContext.commit
        upstream_test_tree_sha1 = $UpstreamContext.test_tree_sha1
        oracle_manifest_sha256 = $UpstreamContext.manifest_sha256
        historical_results_sha256 = $historical_results_hash_before
        go_version = $go_version
        node_version = $node_version
        host_before_correctness = $host_before_correctness
        normalized_benchmark_environment = $environment_policy
        differential_fuzz_report = $fuzz_context
        frozen_oracle_integration_requested = [bool]$RunFrozenOracleIntegration
        source_files = $source_records_before
    })

    [System.IO.File]::Copy(
        (Join-Path -Path $RepositoryRoot -ChildPath 'fuzz\report.json'),
        (Join-Path -Path $BenchmarkOutputRoot -ChildPath 'evidence\fuzz_report.json'),
        $false
    )
    if ((Get-Sha256 -Path (Join-Path $BenchmarkOutputRoot 'evidence\fuzz_report.json')) -ne $fuzz_context.sha256) {
        throw 'The copied differential fuzz report failed its SHA-256 check.'
    }

    $go_tool_environment = @{
        'CGO_ENABLED' = '0'
        'GOCACHE' = Join-Path $BenchmarkOutputRoot 'runtime\go_cache'
        'GOTMPDIR' = Join-Path $BenchmarkOutputRoot 'runtime\go_tmp'
        'GOTOOLCHAIN' = 'local'
        'GOPROXY' = 'off'
        'GOSUMDB' = 'off'
        'TMP' = Join-Path $BenchmarkOutputRoot 'runtime\go_tmp'
        'TEMP' = Join-Path $BenchmarkOutputRoot 'runtime\go_tmp'
        'QSGO_RUN_ORACLE_TESTS' = $null
    }
    $go_environment = Merge-Environment -Sources @($go_runtime_environment, $node_environment, $go_tool_environment)

    [void](Invoke-LoggedCommand -Label 'Go correctness tests' -FilePath $GoExecutablePath -Arguments @('test', './...', '-count=1') -WorkingDirectory $RepositoryRoot -Environment $go_environment -StdoutPath (Join-Path $BenchmarkOutputRoot 'correctness\go_test_stdout.txt') -StderrPath (Join-Path $BenchmarkOutputRoot 'correctness\go_test_stderr.txt'))
    [void](Invoke-LoggedCommand -Label 'Go vet' -FilePath $GoExecutablePath -Arguments @('vet', './...') -WorkingDirectory $RepositoryRoot -Environment $go_environment -StdoutPath (Join-Path $BenchmarkOutputRoot 'correctness\go_vet_stdout.txt') -StderrPath (Join-Path $BenchmarkOutputRoot 'correctness\go_vet_stderr.txt'))

    if ($RunFrozenOracleIntegration) {
        $oracle_environment = Merge-Environment -Sources @($go_environment, @{'QSGO_RUN_ORACLE_TESTS' = '1'})
        [void](Invoke-LoggedCommand -Label 'Frozen oracle integration' -FilePath $GoExecutablePath -Arguments @('test', './internal/differential', '-run', '^TestFrozenOracleHandshakeAndBasicCases$', '-count=1') -WorkingDirectory $RepositoryRoot -Environment $oracle_environment -StdoutPath (Join-Path $BenchmarkOutputRoot 'correctness\frozen_oracle_stdout.txt') -StderrPath (Join-Path $BenchmarkOutputRoot 'correctness\frozen_oracle_stderr.txt'))
    }

    $upstream_test_result = Invoke-LoggedCommand -Label 'Frozen upstream tests' -FilePath $NodeExecutablePath -Arguments @($UpstreamContext.tape_relative_path, 'test/**/*.js') -WorkingDirectory $FrozenUpstreamRoot -Environment $node_environment -StdoutPath (Join-Path $BenchmarkOutputRoot 'correctness\upstream_test_stdout.txt') -StderrPath (Join-Path $BenchmarkOutputRoot 'correctness\upstream_test_stderr.txt')
    if ($upstream_test_result.stdout -notmatch '(?m)^# tests 1045\s*$' -or
        $upstream_test_result.stdout -notmatch '(?m)^# pass  1045\s*$' -or
        $upstream_test_result.stdout -match '(?m)^# fail\s+') {
        throw 'Frozen upstream tests exited successfully but did not report the expected 1045/1045 Tape baseline.'
    }

    $benchmark_binary_path = Join-Path $BenchmarkOutputRoot 'artifacts\benchmark_qsgo.exe'
    $qsgo_binary_path = Join-Path $BenchmarkOutputRoot 'artifacts\qsgo.exe'
    [void](Invoke-LoggedCommand -Label 'Go benchmark build' -FilePath $GoExecutablePath -Arguments @('build', '-trimpath', '-o', $benchmark_binary_path, './cmd/benchmark_qsgo') -WorkingDirectory $RepositoryRoot -Environment $go_environment -StdoutPath (Join-Path $BenchmarkOutputRoot 'correctness\benchmark_build_stdout.txt') -StderrPath (Join-Path $BenchmarkOutputRoot 'correctness\benchmark_build_stderr.txt'))
    [void](Invoke-LoggedCommand -Label 'qsgo build' -FilePath $GoExecutablePath -Arguments @('build', '-trimpath', '-o', $qsgo_binary_path, './cmd/qsgo') -WorkingDirectory $RepositoryRoot -Environment $go_environment -StdoutPath (Join-Path $BenchmarkOutputRoot 'correctness\qsgo_build_stdout.txt') -StderrPath (Join-Path $BenchmarkOutputRoot 'correctness\qsgo_build_stderr.txt'))

    $host_before_measurement = Get-HostMetadata
    Write-JsonFile -Path (Join-Path $BenchmarkOutputRoot 'runtime\host_snapshots.json') -Value ([pscustomobject][ordered]@{
        before_correctness = $host_before_correctness
        before_measurement = $host_before_measurement
    })

    $original_latency_process = Invoke-NativeProcess -FilePath $NodeExecutablePath -Arguments @('--expose-gc', 'bench/original_benchmark.cjs') -WorkingDirectory $RepositoryRoot -Environment $node_environment -CaptureWorkingSet -PollIntervalMilliseconds $script:working_set_poll_interval_ms -TimeoutSeconds 900
    Write-Utf8Text -Path (Join-Path $BenchmarkOutputRoot 'raw\original_latency.json') -Text $original_latency_process.stdout
    Write-Utf8Text -Path (Join-Path $BenchmarkOutputRoot 'raw\original_latency_stderr.txt') -Text $original_latency_process.stderr
    Write-JsonFile -Path (Join-Path $BenchmarkOutputRoot 'raw\original_working_set.json') -Value ([pscustomobject][ordered]@{
        runtime = 'original'
        poll_interval_ms = $script:working_set_poll_interval_ms
        samples = @($original_latency_process.working_set_samples)
    })
    if ($original_latency_process.exit_code -ne 0) {
        throw "Original latency benchmark failed with exit code $($original_latency_process.exit_code)."
    }

    $port_latency_process = Invoke-NativeProcess -FilePath $benchmark_binary_path -WorkingDirectory $RepositoryRoot -Environment $go_runtime_environment -CaptureWorkingSet -PollIntervalMilliseconds $script:working_set_poll_interval_ms -TimeoutSeconds 900
    Write-Utf8Text -Path (Join-Path $BenchmarkOutputRoot 'raw\port_latency.json') -Text $port_latency_process.stdout
    Write-Utf8Text -Path (Join-Path $BenchmarkOutputRoot 'raw\port_latency_stderr.txt') -Text $port_latency_process.stderr
    Write-JsonFile -Path (Join-Path $BenchmarkOutputRoot 'raw\port_working_set.json') -Value ([pscustomobject][ordered]@{
        runtime = 'port'
        poll_interval_ms = $script:working_set_poll_interval_ms
        samples = @($port_latency_process.working_set_samples)
    })
    if ($port_latency_process.exit_code -ne 0) {
        throw "Port latency benchmark failed with exit code $($port_latency_process.exit_code)."
    }

    try {
        $original_latency_report = $original_latency_process.stdout | ConvertFrom-Json
        $port_latency_report = $port_latency_process.stdout | ConvertFrom-Json
    } catch {
        throw 'A latency benchmark did not emit valid JSON.'
    }
    if ([bool]$original_latency_report.upstream_identity_verified -ne $true) {
        throw 'The original latency process did not use the verified upstream checkout.'
    }
    $original_workloads = Get-WorkloadSummaries -LatencyReport $original_latency_report -ExpectedSchema 'qs_original_benchmark/v1'
    $port_workloads = Get-WorkloadSummaries -LatencyReport $port_latency_report -ExpectedSchema 'qs_go_benchmark/v1'
    if ($original_workloads.Count -ne $port_workloads.Count) {
        throw 'The two latency reports contain different workload counts.'
    }
    foreach ($workload_name in $original_workloads.Keys) {
        if (-not $port_workloads.Contains($workload_name)) {
            throw "The port latency report is missing workload $workload_name."
        }
        if ([int64]$original_workloads[$workload_name].checksum -ne [int64]$port_workloads[$workload_name].checksum) {
            throw "The latency checksum differs for workload $workload_name."
        }
    }

    $original_startup_path = Join-Path $BenchmarkOutputRoot 'raw\original_cold_start.json'
    $port_startup_path = Join-Path $BenchmarkOutputRoot 'raw\port_cold_start.json'
    $original_startup = Measure-ColdStarts -RuntimeName 'Original' -FilePath $NodeExecutablePath -Arguments @('bench/startup_probe.cjs') -WorkingDirectory $RepositoryRoot -Environment $node_environment -RawOutputPath $original_startup_path
    $port_startup = Measure-ColdStarts -RuntimeName 'Port' -FilePath $qsgo_binary_path -Arguments @('parse', 'a%5Bb%5D=c') -WorkingDirectory $RepositoryRoot -Environment $go_runtime_environment -RawOutputPath $port_startup_path -ExpectedStdout '{"a":{"b":"c"}}'

    $host_after_measurement = Get-HostMetadata
    Write-JsonFile -Path (Join-Path $BenchmarkOutputRoot 'runtime\host_snapshots.json') -Value ([pscustomobject][ordered]@{
        before_correctness = $host_before_correctness
        before_measurement = $host_before_measurement
        after_measurement = $host_after_measurement
    })
    $historical_results_hash_after = Get-Sha256 -Path $historical_results_path
    if ($historical_results_hash_after -ne $historical_results_hash_before) {
        throw 'The frozen historical bench/results.json changed during the run.'
    }
    $repository_context_after = Get-GitContext -GitExecutablePath $GitExecutablePath -RepositoryRoot $RepositoryRoot
    if (-not $repository_context_after.clean -or $repository_context_after.commit -ne $RepositoryContext.commit) {
        throw 'The implementation repository changed during the benchmark run.'
    }
    $upstream_context_after = Test-FrozenUpstream -GitExecutablePath $GitExecutablePath -RepositoryRoot $RepositoryRoot -FrozenUpstreamRoot $FrozenUpstreamRoot
    if ($upstream_context_after.commit -ne $UpstreamContext.commit -or $upstream_context_after.manifest_sha256 -ne $UpstreamContext.manifest_sha256) {
        throw 'The frozen upstream evidence changed during the benchmark run.'
    }
    $source_records_after = @($source_paths | ForEach-Object { Get-SourceRecord -RepositoryRoot $RepositoryRoot -RelativePath $_ })
    for ($source_index = 0; $source_index -lt $source_records_before.Count; $source_index++) {
        if ($source_records_before[$source_index].path -ne $source_records_after[$source_index].path -or
            $source_records_before[$source_index].sha256 -ne $source_records_after[$source_index].sha256) {
            throw "Benchmark source changed during the run: $($source_records_before[$source_index].path)"
        }
    }

    $original_polled_peak = Get-PolledPeakWorkingSet -Samples @($original_latency_process.working_set_samples)
    $port_polled_peak = Get-PolledPeakWorkingSet -Samples @($port_latency_process.working_set_samples)
    $workload_comparison = [ordered]@{}
    foreach ($workload_name in $original_workloads.Keys) {
        $original_workload = $original_workloads[$workload_name]
        $port_workload = $port_workloads[$workload_name]
        $workload_comparison[$workload_name] = [pscustomobject][ordered]@{
            median_change_percent = [math]::Round(((100.0 * [double]$port_workload.median_ns_per_op / [double]$original_workload.median_ns_per_op) - 100.0), 6)
            p99_change_percent = [math]::Round(((100.0 * [double]$port_workload.p99_ns_per_op / [double]$original_workload.p99_ns_per_op) - 100.0), 6)
        }
    }

    $source_records = $source_records_before

    $artifact_paths = @(
        'artifacts\benchmark_qsgo.exe',
        'artifacts\qsgo.exe',
        'correctness\go_test_stdout.txt',
        'correctness\go_test_stderr.txt',
        'correctness\go_vet_stdout.txt',
        'correctness\go_vet_stderr.txt',
        'correctness\upstream_test_stdout.txt',
        'correctness\upstream_test_stderr.txt',
        'correctness\benchmark_build_stdout.txt',
        'correctness\benchmark_build_stderr.txt',
        'correctness\qsgo_build_stdout.txt',
        'correctness\qsgo_build_stderr.txt',
        'evidence\fuzz_report.json',
        'raw\original_latency.json',
        'raw\original_latency_stderr.txt',
        'raw\original_working_set.json',
        'raw\port_latency.json',
        'raw\port_latency_stderr.txt',
        'raw\port_working_set.json',
        'raw\original_cold_start.json',
        'raw\port_cold_start.json',
        'runtime\record_context.json',
        'runtime\host_snapshots.json'
    )
    if ($RunFrozenOracleIntegration) {
        $artifact_paths += 'correctness\frozen_oracle_stdout.txt'
        $artifact_paths += 'correctness\frozen_oracle_stderr.txt'
    }
    $privacy_file_count_before_summary = Assert-PublishableEvidencePrivacy -OutputRoot $BenchmarkOutputRoot -RepositoryRoot $RepositoryRoot -AdditionalSensitiveValues @($UpstreamContext.root_sha256)
    $artifact_records = @($artifact_paths | ForEach-Object { Get-ArtifactRecord -OutputRoot $BenchmarkOutputRoot -RelativePath $_ })

    $summary = [pscustomobject][ordered]@{
        schema = 'qs_go_comparative_benchmark/v2'
        recorded_at = $record_started_at
        completed_at = [DateTime]::UtcNow.ToString('o')
        source = [pscustomobject][ordered]@{
            repository_commit = $RepositoryContext.commit
            repository_tree_sha1 = $RepositoryContext.tree_sha1
            repository_branch = $RepositoryContext.branch
            repository_clean_before_and_after = $true
            upstream_commit = $UpstreamContext.commit
            upstream_describe = $UpstreamContext.describe
            upstream_test_tree_sha1 = $UpstreamContext.test_tree_sha1
            oracle_manifest_sha256 = $UpstreamContext.manifest_sha256
            differential_fuzz_report = $fuzz_context
            historical_results_path = 'bench/results.json'
            historical_results_sha256 = $historical_results_hash_before
            files = $source_records
        }
        environment = [pscustomobject][ordered]@{
            host_before_correctness = $host_before_correctness
            host_before_measurement = $host_before_measurement
            host_after_measurement = $host_after_measurement
            go_version = $go_version
            node_version = $node_version
            normalized_benchmark_environment = $environment_policy
        }
        method = [pscustomobject][ordered]@{
            latency_samples = $script:latency_sample_count
            iterations_per_sample = $script:iterations_per_sample
            startup_samples = $script:startup_sample_count
            peak_working_set_poll_interval_ms = $script:working_set_poll_interval_ms
            processes_measured_sequentially = $true
            measurement_order = @('original_latency', 'port_latency', 'original_cold_start', 'port_cold_start')
            percentile_index = 'floor((sample_count - 1) * quantile + 0.5)'
            p99_equals_maximum_with_40_samples = $true
            generated_files_never_replace_historical_results = $true
        }
        correctness = [pscustomobject][ordered]@{
            go_test = 'passed'
            go_vet = 'passed'
            upstream_tape_1045_of_1045 = 'passed'
            frozen_oracle_integration = $(if ($RunFrozenOracleIntegration) { 'passed' } else { 'not_requested' })
        }
        runtimes = [pscustomobject][ordered]@{
            original = [pscustomobject][ordered]@{
                name = 'ljharb/qs'
                runtime = [string]$original_latency_report.runtime
                platform = [string]$original_latency_report.platform
                architecture = [string]$original_latency_report.architecture
                runtime_memory_counters = $original_latency_report.memory
                workloads = $original_workloads
                startup = $original_startup
                polled_peak_working_set_bytes = $original_polled_peak
                os_reported_peak_working_set_bytes = [int64]$original_latency_process.os_reported_peak_working_set_bytes
            }
            port = [pscustomobject][ordered]@{
                name = 'qs-go'
                runtime = [string]$port_latency_report.runtime
                platform = [string]$port_latency_report.os
                architecture = [string]$port_latency_report.architecture
                runtime_memory_counters = $port_latency_report.memory
                workloads = $port_workloads
                startup = $port_startup
                polled_peak_working_set_bytes = $port_polled_peak
                os_reported_peak_working_set_bytes = [int64]$port_latency_process.os_reported_peak_working_set_bytes
            }
        }
        port_vs_original = [pscustomobject][ordered]@{
            negative_change_means_port_is_faster_or_smaller = $true
            polled_peak_working_set_change_percent = [math]::Round(((100.0 * $port_polled_peak / $original_polled_peak) - 100.0), 6)
            startup_median_ratio = [math]::Round(([double]$port_startup.median_ms / [double]$original_startup.median_ms), 6)
            startup_p99_ratio = [math]::Round(([double]$port_startup.p99_ms / [double]$original_startup.p99_ms), 6)
            workloads = $workload_comparison
        }
        artifacts = $artifact_records
        privacy_guard = [pscustomobject][ordered]@{
            status = 'passed'
            files_scanned_before_summary = $privacy_file_count_before_summary
            final_post_write_scan = 'passed_on_successful_completion'
        }
        limitations = @(
            'Microbenchmark results are host-specific and were collected in one sequential session.',
            'With 40 samples and the recorded percentile rule, p99 selects the maximum sample.',
            'Working Set was sampled externally every 10 ms; short spikes can be missed.',
            'Go allocation counters and Node heap counters describe different garbage collectors and are not directly comparable.',
            'Correctness evidence is recorded before timing and remains authoritative over benchmark speed.'
        )
    }

    $summary_path = Join-Path $BenchmarkOutputRoot 'summary.json'
    $summary_text = ($summary | ConvertTo-Json -Depth 40) + [Environment]::NewLine
    $summary_user_profile = [Environment]::GetEnvironmentVariable('USERPROFILE')
    $summary_user_profile_escaped = $(if ([string]::IsNullOrWhiteSpace($summary_user_profile)) {
        ''
    } else {
        $summary_user_profile.Replace('\', '\\')
    })
    $sensitive_summary_values = @(
        $RepositoryRoot,
        $RepositoryRoot.Replace('\', '/'),
        $RepositoryRoot.Replace('\', '\\'),
        $summary_user_profile,
        $summary_user_profile_escaped,
        [Environment]::UserName,
        $UpstreamContext.root_sha256
    )
    Assert-PrivacySafeText -Text $summary_text -Label 'summary.json' -SensitiveValues $sensitive_summary_values
    Write-Utf8Text -Path $summary_path -Text $summary_text
    [void](Assert-PublishableEvidencePrivacy -OutputRoot $BenchmarkOutputRoot -RepositoryRoot $RepositoryRoot -AdditionalSensitiveValues @($UpstreamContext.root_sha256))
    [Console]::Out.WriteLine("Benchmark record completed: $BenchmarkOutputRoot")
}

function Invoke-Main {
    $repository_root = [System.IO.Path]::GetFullPath((Join-Path -Path $PSScriptRoot -ChildPath '..'))
    $git_command = Get-Command git -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $git_command) {
        throw 'Git is required but was not found.'
    }
    $git_executable_path = [System.IO.Path]::GetFullPath($git_command.Source)

    if ([string]::IsNullOrWhiteSpace($UpstreamRoot)) {
        $frozen_upstream_root = [System.IO.Path]::GetFullPath((Join-Path -Path $repository_root -ChildPath '..\upstream_qs'))
    } else {
        $frozen_upstream_root = Get-FullPath -Path $UpstreamRoot -BasePath $repository_root
    }
    if (-not (Test-Path -LiteralPath $frozen_upstream_root -PathType Container)) {
        throw "Frozen upstream checkout does not exist: $frozen_upstream_root"
    }

    $go_executable_path = Resolve-Executable -ConfiguredPath $GoExecutable -DefaultCommand 'go' -RepositoryRoot $repository_root -PreferredPath '..\toolchain_complete\go\bin\go.exe'
    $node_executable_path = Resolve-Executable -ConfiguredPath $NodeExecutable -DefaultCommand 'node' -RepositoryRoot $repository_root
    $repository_context = Get-GitContext -GitExecutablePath $git_executable_path -RepositoryRoot $repository_root
    $upstream_context = Test-FrozenUpstream -GitExecutablePath $git_executable_path -RepositoryRoot $repository_root -FrozenUpstreamRoot $frozen_upstream_root

    if ($Mode -eq 'validate') {
        Invoke-Validation -RepositoryRoot $repository_root -RepositoryContext $repository_context -UpstreamContext $upstream_context -GoExecutablePath $go_executable_path -NodeExecutablePath $node_executable_path
        return
    }

    $benchmark_output_root = Get-OutputRoot -RepositoryRoot $repository_root -ConfiguredOutputDirectory $OutputDirectory
    Invoke-Record -RepositoryRoot $repository_root -FrozenUpstreamRoot $frozen_upstream_root -RepositoryContext $repository_context -UpstreamContext $upstream_context -GitExecutablePath $git_executable_path -GoExecutablePath $go_executable_path -NodeExecutablePath $node_executable_path -BenchmarkOutputRoot $benchmark_output_root
}

try {
    Invoke-Main
} catch {
    [Console]::Error.WriteLine("Benchmark runner failed: $($_.Exception.Message)")
    exit 1
}
