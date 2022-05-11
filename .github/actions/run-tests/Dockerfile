ARG BASE_VARIANT=bullseye
ARG GO_VERSION=1.18
ARG XX_VERSION=1.1.0

FROM tonistiigi/xx:${XX_VERSION} AS xx

FROM golang:${GO_VERSION}-${BASE_VARIANT} as gostable

# Copy the build utiltiies
COPY --from=xx / /

# Use the GitHub Actions uid:gid combination for proper fs permissions
RUN groupadd -g 116 test && \
    useradd -u 1001 --gid test --shell /bin/sh --create-home test

# Set path to envtest binaries.
ENV PATH="/github/workspace/envtest:${PATH}"

# Run as test user
USER test

ENTRYPOINT [ "/bin/sh", "-c" ]
