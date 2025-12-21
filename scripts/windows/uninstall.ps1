# LLM Router Windows 服务卸载脚本
# 需要以管理员权限运行

# 检查管理员权限
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "错误: 请以管理员权限运行此脚本" -ForegroundColor Red
    Write-Host "右键点击 PowerShell，选择'以管理员身份运行'" -ForegroundColor Yellow
    exit 1
}

Write-Host "LLM Router Windows 服务卸载脚本" -ForegroundColor Green
Write-Host ""

# 询问卸载哪些服务
Write-Host "请选择要卸载的服务:" -ForegroundColor Yellow
Write-Host "1) 仅后端服务"
Write-Host "2) 仅前端服务"
Write-Host "3) 后端 + 前端服务"
$choice = Read-Host "请选择 (1-3)"

$backendUninstall = $false
$frontendUninstall = $false

switch ($choice) {
    "1" { $backendUninstall = $true }
    "2" { $frontendUninstall = $true }
    "3" { 
        $backendUninstall = $true
        $frontendUninstall = $true
    }
    default {
        Write-Host "无效选择" -ForegroundColor Red
        exit 1
    }
}

# 卸载后端服务
if ($backendUninstall) {
    Write-Host ""
    Write-Host "卸载后端服务..." -ForegroundColor Green
    
    $taskName = "LLMRouter-Backend"
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    
    if ($task) {
        if ($task.State -eq "Running") {
            Stop-ScheduledTask -TaskName $taskName
            Write-Host "后端服务已停止"
        }
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
        Write-Host "后端任务计划已删除"
    } else {
        Write-Host "后端服务未安装"
    }
}

# 卸载前端服务
if ($frontendUninstall) {
    Write-Host ""
    Write-Host "卸载前端服务..." -ForegroundColor Green
    
    $taskName = "LLMRouter-Frontend"
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    
    if ($task) {
        if ($task.State -eq "Running") {
            Stop-ScheduledTask -TaskName $taskName
            Write-Host "前端服务已停止"
        }
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
        Write-Host "前端任务计划已删除"
    } else {
        Write-Host "前端服务未安装"
    }
}

Write-Host ""
Write-Host "卸载完成！" -ForegroundColor Green

