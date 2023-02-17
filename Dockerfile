FROM golang:1.18 as builder

COPY ./autodiscover /src/autodiscover
COPY ./common /src/common
COPY ./dnsprovider /src/dnsprovider
COPY main.go /src/main.go
COPY go.mod /src/go.mod
COPY go.sum /src/go.sum
WORKDIR /src
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN CGO_ENABLED=0  GOOS=linux  GOARCH=amd64  go build  -o tlsprobe main.go

FROM alpine

COPY --from=builder /src/tlsprobe /tlsprobe

ENTRYPOINT ["/tlsprobe"]