FROM golang:1.11-alpine
RUN apk update
RUN apk add openssl ca-certificates git curl
RUN curl https://glide.sh/get | sh
WORKDIR /go/src/region-api
COPY . .
RUN glide update --strip-vendor
RUN go build .
CMD ["./region-api"]
EXPOSE 3000