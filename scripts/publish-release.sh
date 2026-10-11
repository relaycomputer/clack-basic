#!/bin/bash
set -e

TAG="$1"
if [ -z "$TAG" ]; then
  echo "Usage: ./scripts/publish-release.sh <vX.Y.Z> [Release Title]"
  exit 1
fi
TITLE="${2:-Release $TAG}"
TOKEN="${FORGEJO_TOKEN:-41a3c242c766575b10db98047bf3acd9106958b9}"
API_URL="https://git.latentforge.dev/api/v1/repos/relay-computing/clack-basic"

echo "==> 1. Compiling Linux, Windows & macOS Binaries..."
mkdir -p /tmp/clack-build
# Linux x86_64 (Standard PC - default for 'clack')
GOOS=linux GOARCH=amd64 go build -o /tmp/clack-build/clack .
cp /tmp/clack-build/clack /tmp/clack-build/clack-linux-amd64
# Linux ARM64 (aarch64)
GOOS=linux GOARCH=arm64 go build -o /tmp/clack-build/clack-linux-arm64 .
# Windows x64
GOOS=windows GOARCH=amd64 go build -o /tmp/clack-build/clack.exe .
cp /tmp/clack-build/clack.exe /tmp/clack-build/clack-windows-amd64.exe
# macOS Apple Silicon & Intel
GOOS=darwin GOARCH=arm64 go build -o /tmp/clack-build/clack-darwin-arm64 .
GOOS=darwin GOARCH=amd64 go build -o /tmp/clack-build/clack-darwin-amd64 .

# Copy to local bin/ permanently as well
mkdir -p bin
cp /tmp/clack-build/clack bin/
cp /tmp/clack-build/clack.exe bin/
cp /tmp/clack-build/clack-darwin-arm64 bin/

echo "==> 2. Creating/Fetching Release $TAG on Forgejo..."
RESP=$(curl -k -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tag_name\": \"$TAG\", \"name\": \"$TITLE\", \"body\": \"Automated release build for $TAG with Linux (x86_64 & ARM64), Windows x64 and macOS binaries.\", \"draft\": false}" \
  "$API_URL/releases")

REL_ID=$(echo "$RESP" | jq -r .id 2>/dev/null || echo "null")
if [ "$REL_ID" = "null" ] || [ -z "$REL_ID" ]; then
  echo "Release might already exist, fetching existing ID for tag $TAG..."
  REL_ID=$(curl -k -s -H "Authorization: token $TOKEN" "$API_URL/releases/tags/$TAG" | jq -r .id)
fi

echo "==> Release ID: $REL_ID"

echo "==> 3. Uploading Assets..."
upload_asset() {
  local FILE="$1"
  local NAME="$2"
  if [ -f "$FILE" ]; then
    echo "  -> Uploading $NAME..."
    # If asset exists, delete it first to ensure overwrite
    ASSET_ID=$(curl -k -s -H "Authorization: token $TOKEN" "$API_URL/releases/$REL_ID/assets" | jq -r ".[] | select(.name==\"$NAME\") | .id" 2>/dev/null || echo "")
    if [ -n "$ASSET_ID" ] && [ "$ASSET_ID" != "null" ]; then
      curl -k -s -X DELETE -H "Authorization: token $TOKEN" "$API_URL/releases/$REL_ID/assets/$ASSET_ID" >/dev/null
    fi
    curl -k -s -X POST -H "Authorization: token $TOKEN" \
      -F "attachment=@$FILE;filename=$NAME" \
      "$API_URL/releases/$REL_ID/assets?name=$NAME" >/dev/null
  fi
}

upload_asset "/tmp/clack-build/clack" "clack"
upload_asset "/tmp/clack-build/clack-linux-amd64" "clack-linux-amd64"
upload_asset "/tmp/clack-build/clack-linux-arm64" "clack-linux-arm64"
upload_asset "/tmp/clack-build/clack.exe" "clack.exe"
upload_asset "/tmp/clack-build/clack-windows-amd64.exe" "clack-windows-amd64.exe"
upload_asset "/tmp/clack-build/clack-darwin-arm64" "clack-darwin-arm64"
upload_asset "/tmp/clack-build/clack-darwin-amd64" "clack-darwin-amd64"

rm -rf /tmp/clack-build
echo "==> ✅ SUCCESS! Release $TAG is live on Forgejo with Linux, Windows and macOS binaries attached."
