# syntax=docker/dockerfile:1
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY cc-relay /usr/local/bin/cc-relay

EXPOSE 8787 9090

ENTRYPOINT ["/usr/local/bin/cc-relay"]
CMD ["serve"]
