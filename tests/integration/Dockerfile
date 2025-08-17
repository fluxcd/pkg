# Using scratch base image results in `x509: certificate signed by unknown
# authority` error.
# Use alpine to include the necessary certificates.
FROM alpine:3.16

COPY app .

ENTRYPOINT ["/app"]
