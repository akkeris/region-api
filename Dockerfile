FROM golang:1.11-alpine
RUN apk update
RUN apk add openssl ca-certificates git curl build-base bzr
WORKDIR /go/src/region-api
COPY . .
ENV GO111MODULE=on
RUN go build .
RUN chmod +x start.sh
CMD ["./start.sh"]
EXPOSE 3000
