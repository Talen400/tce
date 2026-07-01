#!/bin/sh
# go-env.sh — Configura PATH para Go 1.24+ (necessário para compilar o TCE)
#
# Uso: source go-env.sh

GO_VERSION_MIN="1.24"

# Verifica se já temos Go >= 1.24 no PATH
if command -v go >/dev/null 2>&1; then
    ver=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
    if echo "$ver $GO_VERSION_MIN" | awk '{exit !($1 >= $2)}' 2>/dev/null; then
        echo "Go $ver encontrado no PATH. Tudo ok."
        return 0
    fi
fi

# Procura Go 1.24+ em locais comuns
for candidate in \
    "$HOME/go1.24/bin/go" \
    "$HOME/.local/go/bin/go" \
    "$HOME/sdk/go1.24/bin/go" \
    "/usr/local/go/bin/go" \
    "/usr/local/go1.24/bin/go" \
    "/opt/go/bin/go" \
    "/snap/go/current/bin/go"; do
    if [ -x "$candidate" ]; then
        dir=$(dirname "$candidate")
        export PATH="$dir:$PATH"
        ver=$("$candidate" version | grep -oP 'go\K[0-9]+\.[0-9]+')
        echo "Go $ver encontrado em $dir. PATH atualizado."
        return 0
    fi
done

# Tenta instalar Go 1.24
echo "Go >= $GO_VERSION_MIN não encontrado."
printf "Descarregar e instalar Go 1.24 em ~/.local/go? (y/N): "
read answer
if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
    esac
    tarball="go1.24.4.$os-$arch.tar.gz"
    url="https://go.dev/dl/$tarball"

    echo "A descarregar $url ..."
    mkdir -p "$HOME/.local"
    curl -L "$url" | tar -C "$HOME/.local" -xzf -

    export PATH="$HOME/.local/go/bin:$PATH"
    echo "Go 1.24 instalado em ~/.local/go e adicionado ao PATH."
    echo
    echo "Para tornar permanente, adiciona ao ~/.zshrc:"
    echo '  export PATH="$HOME/.local/go/bin:$PATH"'
else
    echo "Podes instalar manualmente: https://go.dev/dl/go1.24.4.linux-amd64.tar.gz"
fi
