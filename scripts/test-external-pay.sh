#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

container="aidea-externalpay-test-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run --rm -d --name "$container" \
  -e MARIADB_ROOT_PASSWORD=localtest \
  -e MARIADB_DATABASE=aidea_pay_test \
  -p 127.0.0.1::3306 mariadb:11 >/dev/null

port="$(docker port "$container" 3306/tcp | sed -n 's/.*://p')"
if [[ -z "$port" ]]; then
  echo "Could not determine temporary MariaDB port" >&2
  exit 1
fi

ready=false
for _ in {1..60}; do
  if docker exec "$container" mariadb -uroot -plocaltest -N -e 'SELECT 1' aidea_pay_test >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  docker logs --tail 30 "$container" >&2
  echo "Temporary MariaDB did not become ready" >&2
  exit 1
fi

export AIDEA_EXTERNALPAY_TEST_DSN="root:localtest@tcp(127.0.0.1:${port})/aidea_pay_test?parseTime=true&loc=Local"
go test -race ./internal/externalpay -count=1
go test ./server/controllers -run '^TestStrictPriceFen$' -count=1
go build ./...
