FROM golang:1.23 AS builder
RUN apt-get update && apt-get install -y protobuf-compiler
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
WORKDIR /app
COPY . .
RUN sed -i '/^package transit_realtime;/a option go_package = "kv-transit/proto;gtfsrt";' proto/gtfs-realtime.proto
RUN protoc --go_out=paths=source_relative:. proto/gtfs-realtime.proto
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /server .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /server /server
CMD ["/server"]
