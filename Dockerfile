# 1コンテナ構成。Go のバイナリが /api と Svelte のビルドの両方を配信するため、
# フロントエンドと API は常に同一 Origin になる。

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
# プロフィール写真の書き込み先。日単位で管理し毎日掃除されるため、再起動で
# 失われても失うのは当日ぶんの写真だけ。
RUN mkdir -p /app/data/photos
ENV WEB_DIST_DIR=/app/web/dist \
    PHOTO_DIR=/app/data/photos \
    ADDR=:8080
EXPOSE 8080
CMD ["/app/server"]
