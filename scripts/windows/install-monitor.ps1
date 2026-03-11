# LLM Router Windows 前端服务安装脚本
# 需要以管理员权限运行

param(
    [string]$ProjectPath = $PSScriptRoot + "\..\.."
)

# 检查管理员权限
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "错误: 请以管理员权限运行此脚本" -ForegroundColor Red
    Write-Host "右键点击 PowerShell，选择'以管理员身份运行'" -ForegroundColor Yellow
    exit 1
}

Write-Host "LLM Router Windows 前端服务安装脚本" -ForegroundColor Green
Write-Host ""

# 解析项目路径
$ProjectPath = Resolve-Path $ProjectPath -ErrorAction Stop
$FrontendPath = Join-Path $ProjectPath "frontend"

Write-Host "安装信息:" -ForegroundColor Yellow
Write-Host "  项目目录: $ProjectPath"
Write-Host "  前端目录: $FrontendPath"
Write-Host "  用户: $env:USERNAME"
Write-Host ""

# 检查前端目录
if (-not (Test-Path $FrontendPath)) {
    Write-Host "错误: 前端目录不存在: $FrontendPath" -ForegroundColor Red
    exit 1
}

# 检查 npm 是否在 PATH 中
$npmPath = Get-Command npm -ErrorAction SilentlyContinue
if (-not $npmPath) {
    Write-Host "错误: 未找到 npm，请先安装 Node.js" -ForegroundColor Red
    exit 1
}

$npmExe = $npmPath.Source

# 创建服务脚本
$serviceScript = @"
@echo off
cd /d "$FrontendPath"
"$npmExe" run dev
"@

$scriptPath = "$ProjectPath\scripts\windows\start-frontend.bat"
New-Item -ItemType Directory -Force -Path (Split-Path $scriptPath) | Out-Null
$serviceScript | Out-File -FilePath $scriptPath -Encoding ASCII

Write-Host "服务启动脚本已创建: $scriptPath" -ForegroundColor Green

# 创建任务计划（开机启动）
$taskName = "LLMRouter-Frontend"
$taskDescription = "LLM Router Frontend Service"

# 删除已存在的任务
Unregister-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue

# 创建新任务
$action = New-ScheduledTaskAction -Execute "cmd.exe" -Argument "/c `"$scriptPath`""
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)

Register-ScheduledTask -TaskName $taskName -Description $taskDescription -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null

Write-Host "任务计划已创建: $taskName" -ForegroundColor Green
Write-Host ""
Write-Host "安装完成！" -ForegroundColor Green
Write-Host ""
Write-Host "服务管理命令:" -ForegroundColor Yellow
Write-Host "  启动: Start-ScheduledTask -TaskName `"$taskName`""
Write-Host "  停止: Stop-ScheduledTask -TaskName `"$taskName`""
Write-Host "  状态: Get-ScheduledTask -TaskName `"$taskName`""
Write-Host ""
Write-Host "现在可以启动服务:"
Write-Host "  Start-ScheduledTask -TaskName `"$taskName`"" -ForegroundColor Cyan

