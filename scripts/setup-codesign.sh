#!/usr/bin/env bash
#
# Creates a local self-signed code-signing identity ("MM Local Codesign") in your
# login keychain and imports it so `codesign` can use it without prompting.
#
# Why: the app is otherwise only ad-hoc signed, whose identity (cdhash) changes on
# every rebuild. macOS then treats each build as a new app and re-asks for the
# Accessibility permission. Signing with a stable identity gives the bundle a
# constant "designated requirement", so the permission is granted once and sticks
# across rebuilds.
#
# The private key never leaves your login keychain; nothing secret is committed.
# Run once, then use `make install`.
set -euo pipefail

IDENTITY="MM Local Codesign"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

if security find-identity -p codesigning | grep -q "$IDENTITY"; then
	echo "Code-signing identity '$IDENTITY' already exists. Nothing to do."
	exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cat > "$TMP/cfg.conf" <<'CFG'
[req]
distinguished_name = dn
x509_extensions = v3
prompt = no
[dn]
CN = MM Local Codesign
[v3]
basicConstraints=critical,CA:false
keyUsage=critical,digitalSignature
extendedKeyUsage=critical,codeSigning
CFG

openssl req -x509 -newkey rsa:2048 -keyout "$TMP/key.pem" -out "$TMP/cert.pem" \
	-days 3650 -nodes -config "$TMP/cfg.conf"
openssl pkcs12 -export -inkey "$TMP/key.pem" -in "$TMP/cert.pem" \
	-out "$TMP/mm.p12" -name "$IDENTITY" -passout pass:mmcert
security import "$TMP/mm.p12" -k "$KEYCHAIN" -P mmcert -T /usr/bin/codesign -A

echo
echo "Created code-signing identity '$IDENTITY'."
echo "It will show as 'not trusted' (self-signed) — that is expected and fine for"
echo "local signing and for keeping the Accessibility permission stable."
echo
echo "Next: run 'make install', then grant Accessibility to mm once."
