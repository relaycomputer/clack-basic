#!/bin/bash
set -e

TAG="$1"
if [ -z "$TAG" ]; then
  echo "Usage: ./scripts/publish-release.sh <vX.Y.Z> [Release Title]"
  exit 1
fi
TITLE="${2:-Release $TAG}"
TOKEN="${FORGEJO_TOKEN:-15377d6bff6b91b76ccfbbd13f6d1effcb7782be}"
API_URL="https://forge.scrapheap.eu/api/v1/repos/relay-computing/clack-basic"

echo "==> 1. Compiling Linux & Windows Binaries..."
mkdir -p /tmp/clack-build
go build -o /tmp/clack-build/clack .
GOOS=windows GOARCH=amd64 go build -o /tmp/clack-build/clack.exe .

echo "==> 2. Creating Release $TAG on Forgejo..."
RESP=$(curl -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tag_name\": \"$TAG\", \"name\": \"$TITLE\", \"body\": \"Automated release build for $TAG\", \"draft\": false}" \
  "$API_URL/releases")

REL_ID=$(echo "$RESP" | jq -r .id)
echo "==> Release created with ID: $REL_ID"

echo "==> 3. Uploading Assets..."
curl -s -X POST -H "Authorization: token $TOKEN" \
  -F "attachment=@/tmp/clack-build/clack.exe;filename=clack.exe" \
  "$API_URL/releases/$REL_ID/assets?name=clack.exe" >/dev/null

curl -s -X POST -H "Authorization: token $TOKEN" \
  -F "attachment=@/tmp/clack-build/clack;filename=clack" \
  "$API_URL/releases/$REL_ID/assets?name=clack" >/dev/null

rm -rf /tmp/clack-build
echo "==> ✅ SUCCESS! Release $TAG is live with Windows and Linux binaries attached."
