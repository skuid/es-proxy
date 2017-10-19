# To build:
# $ docker run --rm -v $(pwd):/go/src/github.com/skuid/es-proxy -w /go/src/github.com/skuid/es-proxy golang:1.7  go build -v -a -tags netgo -installsuffix netgo -ldflags '-w'
# $ docker build -t skuid/es-proxy .
#
# To run:
# $ docker run skuid/es-proxy

FROM alpine

RUN apk -U add ca-certificates

LABEL maintainer=devops@skuid.com
EXPOSE 3000
EXPOSE 3001

COPY es-proxy /bin/es-proxy
RUN chmod 755 /bin/es-proxy

ENTRYPOINT ["/bin/es-proxy"]
