FROM golang:alpine AS builder

WORKDIR $GOPATH/src/mypackage/myapp/

COPY . .

ENV GOPROXY=https://proxy.golang.com.cn,direct

RUN go get -d -v

RUN go build -o /go/bin/ddsu666

FROM alpine

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk --no-cache add tzdata

WORKDIR /data

COPY --from=builder /go/bin/ddsu666 /data/ddsu666

ENTRYPOINT ["/data/ddsu666"]