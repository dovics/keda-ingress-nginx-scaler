FROM golang:1.24 AS builder

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o external-scaler cmd/main.go


FROM alpine:latest

WORKDIR /

COPY --from=builder /src/external-scaler .

ENTRYPOINT ["/external-scaler"]