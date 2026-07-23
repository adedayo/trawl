FROM golang:1.24-alpine AS build-tools

RUN apk add --no-cache git

RUN go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest && \
    go install github.com/owasp-amass/amass/v4/...@latest

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl jq bash whois bind-tools

COPY --from=build-tools /go/bin/subfinder /usr/local/bin/
COPY --from=build-tools /go/bin/amass /usr/local/bin/

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
