#!/bin/bash
set -e

echo "=== Kahook - Stripe Webhook Example ==="

WEBHOOK_URL="${WEBHOOK_URL:-http://localhost:8080/stripe-events}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-changeme}"

TIMESTAMP=$(date +%s)
PAYLOAD=$(cat <<EOF
{
  "id": "evt_1234567890",
  "object": "event",
  "api_version": "2023-10-16",
  "created": $TIMESTAMP,
  "data": {
    "object": {
      "id": "pi_1234567890",
      "object": "payment_intent",
      "amount": 2000,
      "currency": "usd",
      "status": "succeeded",
      "customer": "cus_1234567890"
    }
  },
  "type": "payment_intent.succeeded"
}
EOF
)

echo "Sending webhook to: $WEBHOOK_URL"
echo ""

curl -X POST "$WEBHOOK_URL" \
  -u "$USERNAME:$PASSWORD" \
  -H "Content-Type: application/json" \
  -H "Stripe-Signature: t=$TIMESTAMP,v1=abc123fake_signature_for_testing" \
  -d "$PAYLOAD" \
  -v

echo ""
echo "=== Done ==="
