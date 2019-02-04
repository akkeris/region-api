FROM golang:1.11-alpine
RUN apk update
RUN apk add openssl ca-certificates git curl
RUN curl https://glide.sh/get | sh
RUN wget https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 -O /usr/bin/dep
RUN chmod +x /usr/bin/dep
WORKDIR /go/src/region-api
COPY . .
RUN dep ensure
RUN go build .
RUN chmod +x start.sh
CMD ["./start.sh"]
EXPOSE 3000
