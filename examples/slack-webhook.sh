#!/bin/bash
set -e

echo "=== Kahook - Slack Webhook Example ==="

WEBHOOK_URL="${WEBHOOK_URL:-http://localhost:8080/slack-events}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-changeme}"

PAYLOAD=$(cat <<'EOF'
{
  "token": "verification_token_here",
  "team_id": "T0001",
  "api_app_id": "A0001",
  "event": {
    "type": "message",
    "channel": "C0001",
    "user": "U0001",
    "text": "Hello, world!",
    "ts": "1234567890.000001"
  },
  "type": "event_callback",
  "event_id": "Ev0001",
  "event_time": 1234567890
}
EOF
)

echo "Sending webhook to: $WEBHOOK_URL"
echo ""

curl -X POST "$WEBHOOK_URL" \
  -u "$USERNAME:$PASSWORD" \
  -H "Content-Type: application/json" \
  -H "X-Slack-Request-Timestamp: $(date +%s)" \
  -H "X-Slack-Signature: v0=abc123fake_signature" \
  -d "$PAYLOAD" \
  -v

echo ""
echo "=== Done ==="
