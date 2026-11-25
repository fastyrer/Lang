#!/bin/sh
set -eu
cd "$(dirname "$0")"
. ./deploy/compose.sh
umask 077

case "${1:-}" in
    backup)
        target=${2:-"backups/lingua-$(date -u +%Y%m%dT%H%M%SZ).json"}
        mkdir -p "$(dirname "$target")"
        tmp=$(mktemp "${target}.XXXXXX")
        trap 'rm -f "$tmp"' EXIT HUP INT TERM
        compose exec -T app sh -eu -c 'cat "$LINGUA_DATA"' > "$tmp"
        test -s "$tmp"
        ln "$tmp" "$target"
        printf 'Резервная копия сохранена в %s\n' "$target"
        ;;
    restore)
        source=${2:?'Укажите путь к резервной копии.'}
        test -f "$source"
        compose stop app
        trap 'compose up --wait --wait-timeout 90 app >&2 || true' EXIT
        compose run --rm --no-deps -T --entrypoint sh app -eu -c '
            store="$LINGUA_DATA"
            tmp=$(mktemp "${store}.restore.XXXXXX")
            trap '\''rm -f "$tmp"'\'' EXIT
            cat > "$tmp"
            /usr/local/bin/lingua --check-store "$tmp"
            if [ -f "$store" ]; then
                previous=$(mktemp "${store}.before-restore.XXXXXX")
                cat "$store" > "$previous"
                printf "Предыдущая база сохранена в томе как %s\n" "$previous"
            fi
            mv "$tmp" "$store"
        ' < "$source"
        compose up --wait --wait-timeout 90 app
        trap - EXIT
        printf '%s\n' 'Данные восстановлены.'
        ;;
    *)
        printf '%s\n' 'Команды: ./data.sh backup [файл] или ./data.sh restore файл.' >&2
        exit 2
        ;;
esac
