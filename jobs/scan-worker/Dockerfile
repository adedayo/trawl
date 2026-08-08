FROM golang:1.26-alpine AS build-tools

# Install scanning tools from source
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git

# Pinned, not @latest. An image tag is supposed to identify a set of scanner
# behaviours; @latest means rebuilding vX.Y.Z next month produces a different
# scanner, so a finding cannot be attributed to a version and a regression
# between two runs cannot be distinguished from a tool upgrade. Renovate keeps
# these current — moving them is a reviewed change, not a side effect of a
# rebuild.
ARG NAABU_VERSION=v2.3.5
ARG HTTPX_VERSION=v1.7.1
ARG NUCLEI_VERSION=v3.4.10
RUN go install "github.com/projectdiscovery/naabu/v2/cmd/naabu@${NAABU_VERSION}" && \
    go install "github.com/projectdiscovery/httpx/cmd/httpx@${HTTPX_VERSION}" && \
    go install "github.com/projectdiscovery/nuclei/v3/cmd/nuclei@${NUCLEI_VERSION}"

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl jq bash bind-tools

COPY --from=build-tools /go/bin/naabu /usr/local/bin/
COPY --from=build-tools /go/bin/httpx /usr/local/bin/
COPY --from=build-tools /go/bin/nuclei /usr/local/bin/

# Download nuclei templates (KEV-tagged templates used by default)
RUN nuclei -update-templates 2>/dev/null || true

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
