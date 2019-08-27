FROM alpine:3.9
LABEL maintainer="jan.knipper@sap.com"

RUN apk --no-cache add ca-certificates
COPY reject-proxy /reject-proxy

ENTRYPOINT ["/reject-proxy"]
