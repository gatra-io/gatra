# GATRA End-to-End Automated Integration Test Suite
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "🧪 Running GATRA E2E Integration Test Suite" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

# 0. Kill any orphan gatra processes
Get-Process -Name "gatra" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300

# 1. Build Binary
Write-Host "`n[1/6] Building binary..." -ForegroundColor Yellow
go build -o bin\gatra.exe .\cmd\gatra
if ($LASTEXITCODE -ne 0) { throw "Build failed" }
Write-Host "✓ Binary built successfully" -ForegroundColor Green

# 2. Generate Key Pair & Mint Token (Using Structured JSON Output)
Write-Host "`n[2/6] Generating keypair and minting capability token..." -ForegroundColor Yellow
$keyJsonRaw = .\bin\gatra.exe gen-keys -t e2e_traj_001 -p "demo/tool" --json | Out-String
$keyObj = $keyJsonRaw | ConvertFrom-Json

$pubKey = $keyObj.public_key
$token = $keyObj.token

Write-Host "✓ Keypair generated and token minted successfully" -ForegroundColor Green

# 3. Launch Standard Blocking Server Process
Write-Host "`n[3/6] Launching GATRA server in standard blocking mode..." -ForegroundColor Yellow
$logPath = "e2e_server.log"
$errPath = "e2e_server_err.log"

if (Test-Path $logPath) { Remove-Item $logPath -Force -ErrorAction SilentlyContinue }
if (Test-Path $errPath) { Remove-Item $errPath -Force -ErrorAction SilentlyContinue }

$serverProcess = Start-Process -FilePath ".\bin\gatra.exe" -ArgumentList "start -c policy.json -k `"$pubKey`" -d e2e_temp.db" -RedirectStandardOutput $logPath -RedirectStandardError $errPath -PassThru -NoNewWindow
Start-Sleep -Seconds 2

if ($serverProcess.HasExited) {
    Write-Host "`n❌ Server failed to start! (Exit Code: $($serverProcess.ExitCode))" -ForegroundColor Red
    if (Test-Path $logPath) { Get-Content $logPath }
    if (Test-Path $errPath) { Get-Content $errPath }
    throw "GATRA server crashed on startup."
}

Write-Host "✓ Server running with zero-trust key configuration" -ForegroundColor Green

try {
    # 4. Execute Standard Blocking Assertions
    Write-Host "`n[4/6] Executing standard policy assertions..." -ForegroundColor Yellow
    $passed = 0
    $failed = 0

    function Assert-Test {
        param([string]$name, [bool]$condition)
        if ($condition) {
            Write-Host "  [PASS] $name" -ForegroundColor Green
            $script:passed++
        } else {
            Write-Host "  [FAIL] $name" -ForegroundColor Red
            $script:failed++
        }
    }

    # Test 1: Health Endpoint
    $health = Invoke-RestMethod -Uri "http://localhost:8080/healthz" -ErrorAction SilentlyContinue
    Assert-Test "Healthz status == healthy" ($health.status -eq "healthy")

    # Test 2: Unauthenticated Request (Missing Token -> 401)
    $authReqFailed = $false
    try {
        Invoke-RestMethod -Uri "http://localhost:8080/v1/action" -Method Post -ContentType "application/json" -Body '{"amount": 10.00}'
    } catch {
        if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::Unauthorized) { $authReqFailed = $true }
    }
    Assert-Test "Missing capability token returns 401 Unauthorized" $authReqFailed

    # Test 3: CEL Condition Policy Rejection (EUR currency -> 403)
    $celFailed = $false
    try {
        Invoke-RestMethod -Uri "http://localhost:8080/v1/action" -Method Post -Headers @{"X-Capability-Token"=$token} -ContentType "application/json" -Body '{"amount": 10.00, "currency": "EUR"}'
    } catch {
        if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::Forbidden) { $celFailed = $true }
    }
    Assert-Test "CEL policy violation (currency EUR) returns 403 Forbidden" $celFailed

    # Test 4: Single-Call Stateless Limit Breach ($60 > $50 limit -> 403)
    $singleLimitFailed = $false
    try {
        Invoke-RestMethod -Uri "http://localhost:8080/v1/action" -Method Post -Headers @{"X-Capability-Token"=$token} -ContentType "application/json" -Body '{"amount": 60.00, "currency": "USD"}'
    } catch {
        if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::Forbidden) { $singleLimitFailed = $true }
    }
    Assert-Test "Single call limit breach ($60 > $50) returns 403 Forbidden" $singleLimitFailed

    # Test 5: Valid USD Request ($40.00 -> 200 OK)
    $res1 = Invoke-RestMethod -Uri "http://localhost:8080/v1/action" -Method Post -Headers @{"X-Capability-Token"=$token} -ContentType "application/json" -Body '{"amount": 40.00, "currency": "USD"}'
    Assert-Test "Valid USD payload returns 200 OK" ($res1.status -eq "success")

    # Test 6: Admin API Policies Endpoint
    $policies = Invoke-RestMethod -Uri "http://localhost:8080/admin/api/policies"
    Assert-Test "Admin API returns active policies" ($policies.rules.Count -ge 1)

    # Test 7: Prometheus Metrics Endpoint
    $metrics = Invoke-RestMethod -Uri "http://localhost:8080/metrics"
    Assert-Test "Metrics endpoint exports gatra_requests_total counter" ($metrics -like "*gatra_requests_total*")

    # Stop blocking server process
    Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500

    # 5. Launch Dry-Run Server Process and Assert
    Write-Host "`n[5/6] Testing Dry-Run / Audit Mode (--dry-run)..." -ForegroundColor Yellow
    $dryRunProcess = Start-Process -FilePath ".\bin\gatra.exe" -ArgumentList "start -c policy.json -k `"$pubKey`" -d e2e_temp_dry.db --dry-run" -RedirectStandardOutput $logPath -RedirectStandardError $errPath -PassThru -NoNewWindow
    Start-Sleep -Seconds 2

    # Test 8: Violating payload ($100 > $50 limit) permitted in Dry-Run mode -> HTTP 200 OK
    $dryRunRes = Invoke-RestMethod -Uri "http://localhost:8080/v1/action" -Method Post -Headers @{"X-Capability-Token"=$token} -ContentType "application/json" -Body '{"amount": 100.00, "currency": "USD"}'
    Assert-Test "Dry-Run mode permits policy violation ($100 > $50) with HTTP 200 OK" ($dryRunRes.status -eq "success")

    # Test 9: Prometheus Metrics exports dry_run_flagged status
    $dryMetrics = Invoke-RestMethod -Uri "http://localhost:8080/metrics"
    Assert-Test "Metrics endpoint exports status=`"dry_run_flagged`"" ($dryMetrics -like "*dry_run_flagged*")

    Stop-Process -Id $dryRunProcess.Id -Force -ErrorAction SilentlyContinue

    # 6. Output Summary
    Write-Host "`n[6/6] Test Results Summary" -ForegroundColor Yellow
    Write-Host "==================================================" -ForegroundColor Cyan
    Write-Host " Passed: $passed" -ForegroundColor Green

    $failedColor = "Gray"
    if ($failed -gt 0) { $failedColor = "Red" }
    Write-Host " Failed: $failed" -ForegroundColor $failedColor
    Write-Host "==================================================" -ForegroundColor Cyan

    if ($failed -gt 0) { throw "E2E Test Suite Failure" }

} finally {
    # Cleanup background processes and temporary databases
    Write-Host "`n🧹 Cleaning up test environment..." -ForegroundColor Gray
    Get-Process -Name "gatra" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 200

    if (Test-Path "e2e_temp.db") { Remove-Item "e2e_temp.db" -Force -ErrorAction SilentlyContinue }
    if (Test-Path "e2e_temp_dry.db") { Remove-Item "e2e_temp_dry.db" -Force -ErrorAction SilentlyContinue }
    if (Test-Path $logPath) { Remove-Item $logPath -Force -ErrorAction SilentlyContinue }
    if (Test-Path $errPath) { Remove-Item $errPath -Force -ErrorAction SilentlyContinue }
    Write-Host "✓ Cleanup complete" -ForegroundColor Green
}