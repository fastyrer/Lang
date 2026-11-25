#!/bin/sh
set -eu

cd "$(dirname "$0")"
. ./deploy/compose.sh

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    printf '%s\n' 'Нужен Docker с плагином Docker Compose.' >&2
    exit 1
fi
if ! docker info --format '{{.ServerVersion}}' >/dev/null; then
    printf '%s\n' 'Запустите Docker Engine или Docker Desktop и повторите команду.' >&2
    exit 1
fi

if ! compose up --build --wait --wait-timeout 90; then
    compose logs --tail=50 app >&2 || true
    exit 1
fi

if [ "${LINGUA_DEPLOY_HTTPS:-0}" = 1 ]; then
    address=$(compose exec -T app printenv LINGUA_PUBLIC_URL)
    printf 'Lingua запущена. Адрес сайта %s\n' "$address"
    exit
fi

binding=$(compose port app 8080)
case "$binding" in
    0.0.0.0:*|'[::]:'*) binding="127.0.0.1:${binding##*:}" ;;
esac
printf 'Lingua запущена. Откройте http://%s\n' "$binding"
