#!/bin/sh
# Instala a última versão do FinFlow (Linux e macOS) em ~/.local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts/install.sh | sh
#
# Variáveis opcionais: FINFLOW_INSTALL_DIR (destino do binário), FINFLOW_VERSION (ex.: v0.2.0; padrão: última),
# FINFLOW_DOWNLOAD_URL (base alternativa dos arquivos: espelho ou teste local).
set -eu

REPO="danielharrisonlf/finflow"
INSTALL_DIR="${FINFLOW_INSTALL_DIR:-$HOME/.local/bin}"
if [ -n "${FINFLOW_DOWNLOAD_URL:-}" ]; then
  BASE_URL="${FINFLOW_DOWNLOAD_URL%/}"
elif [ -n "${FINFLOW_VERSION:-}" ]; then
  BASE_URL="https://github.com/$REPO/releases/download/$FINFLOW_VERSION"
else
  BASE_URL="https://github.com/$REPO/releases/latest/download"
fi

fail() {
  echo "finflow: $*" >&2
  exit 1
}

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) fail "sistema não suportado: $(uname -s). No Windows, use scripts/install.ps1." ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) fail "arquitetura não suportada: $(uname -m)" ;;
esac
# Terminal rodando sob Rosetta num Mac Apple Silicon: prefira o binário nativo.
if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ] &&
  [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = "1" ]; then
  arch="arm64"
fi

asset="finflow-$os-$arch.tar.gz"

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$1" -O "$2"
  else
    fail "instale curl ou wget"
  fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Baixando $asset..."
download "$BASE_URL/$asset" "$tmp/$asset" || fail "não foi possível baixar $BASE_URL/$asset"

if download "$BASE_URL/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
  expected="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$tmp/$asset" | cut -d' ' -f1)"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)"
  else
    actual=""
  fi
  if [ -n "$expected" ] && [ -n "$actual" ] && [ "$expected" != "$actual" ]; then
    fail "checksum SHA-256 não confere para $asset"
  fi
fi

tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$INSTALL_DIR"
mv "$tmp/finflow" "$INSTALL_DIR/finflow"
chmod +x "$INSTALL_DIR/finflow"
if [ "$os" = "darwin" ]; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/finflow" 2>/dev/null || true
fi

echo "FinFlow instalado em $INSTALL_DIR/finflow ($("$INSTALL_DIR/finflow" version 2>/dev/null || echo 'versão desconhecida'))"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "Adicione $INSTALL_DIR ao PATH (por exemplo, no ~/.bashrc ou ~/.zshrc):"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac
echo ""
echo "Para começar: finflow   (abre http://localhost:3333)"
echo "Com dados de exemplo: finflow serve --seed"
