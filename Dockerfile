FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/sdms-api \
    ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S sdms \
    && adduser -S -G sdms sdms

WORKDIR /app
COPY --from=build /out/sdms-api ./sdms-api

USER sdms
EXPOSE 8081

ENTRYPOINT ["/app/sdms-api"]
