# Use the same Compose project and configuration for deployment and backups.
compose() {
    if [ "${LINGUA_DEPLOY_HTTPS:-0}" = 1 ]; then
        if [ -f compose.override.yaml ]; then
            docker compose -f compose.yaml -f compose.https.yaml -f compose.override.yaml "$@"
        else
            docker compose -f compose.yaml -f compose.https.yaml "$@"
        fi
    else
        docker compose "$@"
    fi
}
