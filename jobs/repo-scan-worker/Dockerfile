FROM alpine:3.22

# Target architecture (amd64, arm64) automatically provided by Docker BuildKit
ARG TARGETARCH
ARG CACHEBUST=1

RUN apk add --no-cache ca-certificates curl jq bash git

# Bust cache whenever a new release is published to GitHub
ADD https://api.github.com/repos/adedayo/checkmate/releases/latest /tmp/checkmate_latest.json

RUN LATEST_TAG=$(jq -r '.tag_name' /tmp/checkmate_latest.json | sed 's/^v//') && \
    if [ "$TARGETARCH" = "arm64" ]; then ARCH="arm64"; else ARCH="x86_64"; fi && \
    echo "Installing Checkmate v${LATEST_TAG} for ${ARCH}..." && \
    curl -sL "https://github.com/adedayo/checkmate/releases/download/v${LATEST_TAG}/checkmate_Linux_${ARCH}.tar.gz" -o /tmp/checkmate.tar.gz && \
    tar -xzf /tmp/checkmate.tar.gz -C /usr/local/bin/ checkmate && \
    chmod +x /usr/local/bin/checkmate && \
    rm -rf /tmp/checkmate*

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

