#!/usr/bin/env bash
# Generates a fresh age key and re-encrypts all .ward files for case3.
# Run this once after cloning, or whenever you need to rotate the key.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
KEYFILE="$DIR/.ward.key"

age-keygen -o "$KEYFILE"
PUBKEY=$(grep "public key:" "$KEYFILE" | awk '{print $NF}')
echo "Generated key: $PUBKEY"

reencrypt() {
  local file="$1"
  local plain
  plain=$(SOPS_AGE_KEY_FILE="$KEYFILE" sops decrypt --input-type yaml --output-type yaml "$file")
  echo "$plain" | SOPS_AGE_KEY_FILE="$KEYFILE" sops encrypt --age "$PUBKEY" --input-type yaml --output-type yaml /dev/stdin > "$file.tmp"
  mv "$file.tmp" "$file"
  echo "re-encrypted: $file"
}

find "$DIR/secrets" -name "*.ward" | while read -r f; do
  reencrypt "$f"
done
