#!/bin/bash
set -e

echo "=== Kahook - GitHub Webhook Example ==="

WEBHOOK_URL="${WEBHOOK_URL:-http://localhost:8080/github-webhooks}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-changeme}"

PAYLOAD=$(cat <<'EOF'
{
  "ref": "refs/heads/main",
  "before": "a10867b14bb761a232cd80139fbd4c0d33264240",
  "after": "d8fcb41c4d0b9b2b4c5b9f3b6d2c8f4b3d1c9e5f",
  "repository": {
    "id": 123456789,
    "name": "my-repo",
    "full_name": "user/my-repo",
    "html_url": "https://github.com/user/my-repo"
  },
  "pusher": {
    "name": "username",
    "email": "user@example.com"
  },
  "sender": {
    "login": "username",
    "id": 123456
  }
}
EOF
)

echo "Sending webhook to: $WEBHOOK_URL"
echo ""

curl -X POST "$WEBHOOK_URL" \
  -u "$USERNAME:$PASSWORD" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-GitHub-Delivery: $(uuidgen | tr '[:upper:]' '[:lower:]')" \
  -d "$PAYLOAD" \
  -v

echo ""
echo "=== Done ==="
