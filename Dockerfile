# Single-container deployment: the Go binary serves both /api and the Svelte
# build, so the frontend and the API always share an Origin.

FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=web /web/dist /app/web/dist
ENV WEB_DIST_DIR=/app/web/dist \
    ADDR=:8080
EXPOSE 8080
CMD ["/app/server"]
