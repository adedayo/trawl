FROM golang:1.24-alpine AS build-tools

# Install scanning tools from source
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git

RUN go install github.com/projectdiscovery/naabu/v2/cmd/naabu@latest && \
    go install github.com/projectdiscovery/httpx/cmd/httpx@latest && \
    go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl jq bash

COPY --from=build-tools /go/bin/naabu /usr/local/bin/
COPY --from=build-tools /go/bin/httpx /usr/local/bin/
COPY --from=build-tools /go/bin/nuclei /usr/local/bin/

# Download nuclei templates (KEV-tagged templates used by default)
RUN nuclei -update-templates 2>/dev/null || true

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
