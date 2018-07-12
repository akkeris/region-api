FROM golang:1.9-alpine
RUN apk update
RUN apk add openssl ca-certificates git
RUN go get  "github.com/bitly/go-simplejson"
RUN go get  "github.com/go-martini/martini"
RUN go get  "github.com/martini-contrib/binding"
RUN go get  "github.com/martini-contrib/render"
RUN go get  "github.com/lib/pq"
RUN go get  "github.com/nu7hatch/gouuid"
RUN go get  "github.com/martini-contrib/auth"
RUN go get  "github.com/aws/aws-sdk-go/aws"
RUN go get  "github.com/aws/aws-sdk-go/aws/session"
RUN go get  "github.com/aws/aws-sdk-go/service/route53"
RUN go get  "github.com/robfig/cron"
RUN go get  "gopkg.in/mgo.v2"
RUN go get  "gopkg.in/mgo.v2/bson"
RUN go get  "gopkg.in/Shopify/sarama.v1"
RUN go get 	"github.com/octanner/f5er/f5"
RUN go get  "github.com/akkeris/vault-client"
RUN go get  "github.com/stackimpact/stackimpact-go"
COPY . /go/src/region-api
WORKDIR /go/src/region-api
RUN go build .
CMD ["./region-api"]
EXPOSE 3000
