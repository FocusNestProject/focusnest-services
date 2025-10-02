#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

modules=(
	shared-libs
	gateway-api
	auth-gateway
	user-service
	activity-service
	session-service
	notification-service
	media-service
	webhook-service
	analytics-service
)

for module in "${modules[@]}"; do
	echo "Running go test for ${module}"
	(cd "${ROOT_DIR}/${module}" && go test ./...)
done

echo "✅ FocusNest e2e smoke suite complete"
