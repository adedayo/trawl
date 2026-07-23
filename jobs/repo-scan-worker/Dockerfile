FROM golang:1.24-alpine AS build-tools

RUN apk add --no-cache git

RUN go install github.com/zricethezav/gitleaks/v8@latest

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl jq bash git

COPY --from=build-tools /go/bin/gitleaks /usr/local/bin/

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
