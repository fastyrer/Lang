FROM node:24-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/index.html web/vite.config.js ./
COPY web/src ./src
COPY web/public ./public
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /build
COPY go.mod ./
COPY cmd ./cmd
ENV CGO_ENABLED=0
RUN go test ./... && go build -trimpath -ldflags="-s -w" -o /out/lingua ./cmd/lingua

FROM alpine:3.23
WORKDIR /app
RUN addgroup -S -g 10001 lingua \
    && adduser -S -D -H -u 10001 -G lingua lingua \
    && mkdir -m 0700 /app/data \
    && chown lingua:lingua /app/data
COPY --from=backend /out/lingua /usr/local/bin/lingua
COPY --from=frontend /build/web/dist /app/web
ENV LINGUA_ADDR=0.0.0.0:8080 \
    LINGUA_DATA=/app/data/lingua.json \
    LINGUA_WEB=/app/web
USER lingua:lingua
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=6 \
    CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/"]
ENTRYPOINT ["/usr/local/bin/lingua"]
