FROM alpine:3.21

RUN apk add --no-cache \
    bash \
    git \
    curl \
    wget \
    ca-certificates \
    python3 \
    py3-pip \
    && rm -rf /var/cache/apk/*

RUN adduser -D -u 1000 worker

WORKDIR /workspace

USER worker

LABEL org.opencontainers.image.title="ZClaw Tool Worker"
LABEL org.opencontainers.image.description="Sandboxed command execution worker"

ENTRYPOINT ["bash"]
CMD ["--help"]
