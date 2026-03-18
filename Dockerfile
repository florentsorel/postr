FROM node:24-alpine AS assets

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm install

COPY web/ ./
RUN npm run build


FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=assets /app/internal/web/dist ./internal/web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o /tmp/postr ./cmd/api


FROM gcr.io/distroless/static-debian13:latest

COPY --from=builder /tmp/postr /postr

EXPOSE 8080

CMD ["/postr"]
