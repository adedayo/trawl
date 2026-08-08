FROM golang:1.26-alpine AS build-tools

ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git

# Pinned, not @latest. Discovery output feeds the asset inventory, and an
# unpinned tool means a change in the inventory cannot be told apart from a
# change in the tool that produced it. Renovate proposes bumps; a rebuild does
# not take them silently.
ARG SUBFINDER_VERSION=v2.9.0
ARG AMASS_VERSION=v4.2.0
RUN go install "github.com/projectdiscovery/subfinder/v2/cmd/subfinder@${SUBFINDER_VERSION}" && \
    go install "github.com/owasp-amass/amass/v4/...@${AMASS_VERSION}"

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl jq bash whois bind-tools

COPY --from=build-tools /go/bin/subfinder /usr/local/bin/
COPY --from=build-tools /go/bin/amass /usr/local/bin/

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
