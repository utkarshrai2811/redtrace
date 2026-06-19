#!/usr/bin/env bash
# Install the RedTrace CA certificate into the local trust store so the proxy
# can intercept HTTPS. Run `redtrace ca` or `redtrace serve` first to generate it.
set -euo pipefail

CA="${REDTRACE_CA:-$HOME/.config/redtrace/ca/redtrace-ca.pem}"

if [ ! -f "$CA" ]; then
  echo "RedTrace CA not found at: $CA"
  echo "Generate it first with:  redtrace ca"
  exit 1
fi

echo "Using CA: $CA"

case "$(uname -s)" in
  Darwin)
    echo "Installing into the macOS System keychain (requires sudo)…"
    sudo security add-trusted-cert -d -r trustRoot \
      -k /Library/Keychains/System.keychain "$CA"
    echo "Done."
    ;;
  Linux)
    if command -v update-ca-certificates >/dev/null 2>&1; then
      sudo cp "$CA" /usr/local/share/ca-certificates/redtrace-ca.crt
      sudo update-ca-certificates
      echo "Done (update-ca-certificates)."
    elif command -v update-ca-trust >/dev/null 2>&1; then
      sudo cp "$CA" /etc/pki/ca-trust/source/anchors/redtrace-ca.pem
      sudo update-ca-trust
      echo "Done (update-ca-trust)."
    else
      echo "No supported system trust store found. Import this file manually: $CA"
      exit 1
    fi
    ;;
  *)
    echo "Automated install is not supported on this OS."
    echo "Windows (run in an elevated PowerShell):"
    echo "    certutil -addstore -f \"ROOT\" \"$CA\""
    ;;
esac

echo
echo "Note: Firefox uses its own trust store. Import the CA under"
echo "  Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import"
echo "and select: $CA"
