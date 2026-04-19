FROM node:25-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY prompts/ ./prompts/
COPY deploy/config.example.yaml ./deploy/config.example.yaml
COPY tasks/ ./tasks/
RUN CGO_ENABLED=0 go build -o /naturalize ./cmd/server

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /naturalize /usr/local/bin/naturalize
COPY --from=go-builder /app/prompts ./prompts
COPY --from=web-builder /app/web/dist ./web/dist
EXPOSE 8080
ENTRYPOINT ["naturalize"]
