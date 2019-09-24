FROM alpine

MAINTAINER Dmitry Orzhehovsky <dorzheho@cisco.com>

RUN mkdir -p /opt/cisco/apphc/bin

COPY staging/apphcd /opt/cisco/apphc/bin/apphcd

#RUN apk update && apk add ca-certificates && apk add curl && apk add rm -rf /var/cache/apk/*

#CMD ["/opt/cisco/apphc/bin/apphcd"]
