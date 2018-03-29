FROM golang:alpine as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN	apk add --no-cache \
	ca-certificates

COPY . /go/src/github.com/samkreter/Kirix

RUN set -x \
	&& apk add --no-cache --virtual .build-deps \
		git \
		gcc \
		libc-dev \
		libgcc \
        make \
	&& cd /go/src/github.com/samkreter/Kirix \
	&& make build \ 
	&& apk del .build-deps \
    && cp bin/Kirix /usr/bin/Kirix \
	&& rm -rf /go \
	&& echo "Build complete."

FROM scratch

COPY --from=builder /usr/bin/Kirix /usr/bin/Kirix
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs

ENTRYPOINT [ "/usr/bin/Kirix" ]
CMD [ "--help" ]
