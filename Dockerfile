FROM alpine:3.2
MAINTAINER WiFast Engineering <engineering@wifast.com>
ENV DEBIAN_FRONTEND noninteractive

VOLUME /tmp
ADD . /tmp/src

RUN cp -rf /tmp/src/ssh /root/.ssh && \
    chmod 400 /root/.ssh/id_rsa

ENV GOPATH=/tmp/gopath
RUN apk add --update build-base ca-certificates openssl openssh-client git go && \
    git config --global url."git@github.com:".insteadOf "https://github.com/" && \
    go get golang.org/x/tools/cmd/vet && \
    cd /tmp/src && make clean && make static && \
    cp bin/ballotd.static /usr/bin/ballotd && \
    apk del --purge build-base openssl openssh-client git go && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["/usr/bin/ballotd"]
EXPOSE 80
