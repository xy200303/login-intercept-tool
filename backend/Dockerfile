FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/fenx-server ./cmd/server
FROM alpine:3.20
RUN adduser -D -H fenx
USER fenx
COPY --from=build /out/fenx-server /fenx-server
EXPOSE 8080
ENTRYPOINT ["/fenx-server"]
