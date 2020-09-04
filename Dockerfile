FROM alpine:3.9
LABEL maintainer="jan.knipper@sap.com"
LABEL source_repository="https://github.com/sapcc/reject-proxy"

RUN apk --no-cache add ca-certificates
COPY reject-proxy /reject-proxy

ENTRYPOINT ["/reject-proxy"]
