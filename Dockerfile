FROM golang:1.13-alpine
RUN apk update
RUN apk add openssl ca-certificates git curl build-base bzr
WORKDIR /go/src/region-api
COPY . .
ENV GO111MODULE on
RUN go build .
CMD ["./start.sh"]
EXPOSE 3000
