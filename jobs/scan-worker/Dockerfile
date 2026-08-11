# golang:1.24, not 1.26, and GOTOOLCHAIN=local to hold it there.
#
# nuclei v3.4.10 depends on bytedance/sonic v1.14.0, which reaches into runtime
# internals by linkname. Those moved in Go 1.26, so the build fails on
# `undefined: GoMapIterator` — a fault in a transitive dependency of a pinned
# tool, which we cannot fix and should not wait for.
#
# GOTOOLCHAIN=auto would defeat the pin in one direction only: Go upgrades to
# satisfy a go.mod directive but never downgrades, so on a 1.26 base it stays
# on 1.26. `local` is what actually holds the compiler still. This is
# independent of the Go version the Trawl engine itself is built with; these
# are third-party binaries that happen to be written in Go.
FROM golang:1.24-alpine AS build-tools

ENV GOTOOLCHAIN=local
# naabu binds libpcap through cgo. The alpine image carries no C compiler, so
# cgo defaults off and the pcap package compiles to nothing but undefined
# symbols — pcapErrorNotActivated, pcapTPtr and the rest. build-base and
# libpcap-dev are what make that package real.
RUN apk add --no-cache git build-base libpcap-dev

# Pinned, not @latest. An image tag is supposed to identify a set of scanner
# behaviours; @latest means rebuilding vX.Y.Z next month produces a different
# scanner, so a finding cannot be attributed to a version and a regression
# between two runs cannot be distinguished from a tool upgrade. Renovate keeps
# these current — moving them is a reviewed change, not a side effect of a
# rebuild.
ARG NAABU_VERSION=v2.3.5
ARG HTTPX_VERSION=v1.7.1
ARG NUCLEI_VERSION=v3.4.10

# naabu alone is built with cgo, so it alone is dynamically linked and needs
# libpcap present at runtime. httpx and nuclei stay static, which is worth
# keeping: a static binary cannot be broken by a library upgrade in the base
# image.
RUN CGO_ENABLED=1 go install "github.com/projectdiscovery/naabu/v2/cmd/naabu@${NAABU_VERSION}"
RUN CGO_ENABLED=0 go install "github.com/projectdiscovery/httpx/cmd/httpx@${HTTPX_VERSION}" && \
    CGO_ENABLED=0 go install "github.com/projectdiscovery/nuclei/v3/cmd/nuclei@${NUCLEI_VERSION}"

FROM alpine:3.22

# libpcap is a runtime dependency of naabu, not merely a build one.
RUN apk add --no-cache ca-certificates curl jq bash bind-tools libpcap

COPY --from=build-tools /go/bin/naabu /usr/local/bin/
COPY --from=build-tools /go/bin/httpx /usr/local/bin/
COPY --from=build-tools /go/bin/nuclei /usr/local/bin/

# Download nuclei templates (KEV-tagged templates used by default)
RUN nuclei -update-templates 2>/dev/null || true

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
