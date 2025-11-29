# LLM Router Windows 后端服务安装脚本
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

Write-Host "LLM Router Windows 后端服务安装脚本" -ForegroundColor Green
Write-Host ""

# 解析项目路径
$ProjectPath = Resolve-Path $ProjectPath -ErrorAction Stop

Write-Host "安装信息:" -ForegroundColor Yellow
Write-Host "  项目目录: $ProjectPath"
Write-Host "  用户: $env:USERNAME"
Write-Host ""

# 检查项目目录
if (-not (Test-Path $ProjectPath)) {
    Write-Host "错误: 项目目录不存在: $ProjectPath" -ForegroundColor Red
    exit 1
}

# 检查 uv 是否在 PATH 中
$uvPath = Get-Command uv -ErrorAction SilentlyContinue
if (-not $uvPath) {
    Write-Host "警告: 未找到 uv，请确保已安装并添加到 PATH" -ForegroundColor Yellow
}

# 获取 uv 完整路径
$uvExe = if ($uvPath) { $uvPath.Source } else { "$env:USERPROFILE\.local\bin\uv.exe" }

# 创建服务脚本
$serviceScript = @"
@echo off
cd /d "$ProjectPath"
"$uvExe" run llm-router
"@

$scriptPath = "$ProjectPath\scripts\windows\start-backend.bat"
New-Item -ItemType Directory -Force -Path (Split-Path $scriptPath) | Out-Null
$serviceScript | Out-File -FilePath $scriptPath -Encoding ASCII

Write-Host "服务启动脚本已创建: $scriptPath" -ForegroundColor Green

# 创建任务计划（开机启动）
$taskName = "LLMRouter-Backend"
$taskDescription = "LLM Router Backend Service"

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

