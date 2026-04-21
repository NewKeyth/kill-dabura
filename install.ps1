$ErrorActionPreference = "Stop"

$AppName = "dabura"
# Obtenemos la ruta de la carpeta donde se está ejecutando este script
$CurrentDir = $PSScriptRoot
if (-not $CurrentDir) { $CurrentDir = Get-Location }

$ExecutablePath = Join-Path $CurrentDir "$AppName.exe"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "   Instalador de $AppName (Portable)  " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# Verificar que el ejecutable existe en esta carpeta
if (-not (Test-Path $ExecutablePath)) {
    Write-Host "[!] No se encontró $AppName.exe en $CurrentDir." -ForegroundColor Red
    Write-Host "[*] Intentando compilar desde el código fuente..." -ForegroundColor Yellow
    
    if (Get-Command go -ErrorAction SilentlyContinue) {
        go build -o $ExecutablePath ./cmd/dabura
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[+] Compilación exitosa: $ExecutablePath" -ForegroundColor Green
        } else {
            Write-Host "[X] Error al compilar. Abortando." -ForegroundColor Red
            return
        }
    } else {
        Write-Host "[X] Go no está instalado y no hay un .exe listo. Abortando." -ForegroundColor Red
        return
    }
} else {
    Write-Host "[+] Ejecutable encontrado en: $ExecutablePath" -ForegroundColor Green
}

# Modificar el PATH del usuario
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")

# Limpiar posibles rutas viejas de .dabura para evitar conflictos
$CleanPath = ($UserPath -split ';' | Where-Object { $_ -notlike '*\.dabura\bin*' }) -join ';'

# Verificar si la carpeta actual ya está en el PATH
if ($CleanPath -notlike "*$CurrentDir*") {
    Write-Host "[*] Agregando $CurrentDir al PATH del usuario..." -ForegroundColor Yellow
    $NewPath = "$CleanPath;$CurrentDir"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    Write-Host "[+] PATH actualizado exitosamente." -ForegroundColor Green
} else {
    Write-Host "[+] La ubicación actual ya está en tu PATH." -ForegroundColor Green
    if ($CleanPath.Length -lt $UserPath.Length) {
        [Environment]::SetEnvironmentVariable("PATH", $CleanPath, "User")
        Write-Host "[+] Se limpiaron rutas obsoletas del PATH." -ForegroundColor Gray
    }
}

Write-Host "`n¡Instalación completada!" -ForegroundColor Magenta
Write-Host ">>> IMPORTANTE: Reiniciá la terminal para usar el comando '$AppName'." -ForegroundColor Yellow
