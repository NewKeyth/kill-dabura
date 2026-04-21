$ErrorActionPreference = "Stop"
$AppName = "dabura"
$RepoUrl = "https://github.com/NewKeyth/kill-dabura"
$InstallDir = Join-Path $HOME ".dabura\bin"
$ExePath = Join-Path $InstallDir "$AppName.exe"

Write-Host "`n>>> Instalando $AppName v1.0.0 <<<" -ForegroundColor Cyan

# 1. Crear carpeta
if (-not (Test-Path $InstallDir)) { New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null }

# 2. Descargar el ejecutable desde los Releases de GitHub
$DownloadUrl = "$RepoUrl/releases/latest/download/dabura_windows.exe"
Write-Host "[*] Descargando binario desde GitHub..." -ForegroundColor Yellow
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath -UseBasicParsing
    Write-Host "[+] Descarga completada: $ExePath" -ForegroundColor Green
} catch {
    Write-Host "[X] Error: No se pudo descargar. Asegurate de que el Release v1.0.0 exista en GitHub." -ForegroundColor Red
    return
}

# 3. Configurar el PATH (Sin reiniciar la terminal para el proceso actual)
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    $env:Path += ";$InstallDir"
    Write-Host "[+] Agregado al PATH del sistema." -ForegroundColor Green
}

Write-Host "`n¡Listo! Escribí '$AppName' para empezar." -ForegroundColor Magenta
