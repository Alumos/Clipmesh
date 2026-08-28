# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS web-builder
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
ARG VITE_API_BASE_URL=""
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

FROM golang:1.23-alpine AS api-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/clipmesh ./cmd/server

FROM nginx:1.27-alpine
RUN apk add --no-cache ca-certificates wget
COPY --from=web-builder /src/frontend/dist/ /usr/share/nginx/html/
COPY --from=api-builder /out/clipmesh /usr/local/bin/clipmesh
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/start.sh /usr/local/bin/start-clipmesh
RUN chmod +x /usr/local/bin/start-clipmesh

ENV CLIPMESH_ADDR=:9000 \
    CLIPMESH_DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/start-clipmesh"]
