#!/bin/bash
APP_NAME="dabura"
REPO_URL="https://github.com/NewKeyth/kill-dabura"
INSTALL_DIR="$HOME/.local/bin"
BIN_PATH="$INSTALL_DIR/$APP_NAME"

echo -e "\n\033[0;36m>>> Instalando $APP_NAME v1.0.0 <<<\033[0m"

# 1. Crear carpeta
mkdir -p "$INSTALL_DIR"

# 2. Descargar el binario
DOWNLOAD_URL="$REPO_URL/releases/latest/download/dabura_linux"
echo -e "\033[0;33m[*] Descargando binario desde GitHub...\033[0m"
if curl -sSL -o "$BIN_PATH" "$DOWNLOAD_URL"; then
    chmod +x "$BIN_PATH"
    echo -e "\033[0;32m[+] Descarga completada: $BIN_PATH\033[0m"
else
    echo -e "\033[0;31m[X] Error: No se pudo descargar. Asegurate de que el Release v1.0.0 exista.\033[0m"
    exit 1
fi

# 3. Verificar PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "\033[0;35m[*] TIP: Agregá $INSTALL_DIR a tu PATH en .bashrc para usarlo siempre.\033[0m"
    export PATH="$PATH:$INSTALL_DIR"
fi

echo -e "\n\033[0;35m¡Listo! Ya podés usar '$APP_NAME'.\033[0m"
