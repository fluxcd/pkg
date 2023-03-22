FROM python:3-alpine

RUN pip install --no-cache-dir pyaml

ARG KUBERNETES_SPLIT_YAML_VERSION=0.4.0
RUN wget -q -O - \
    https://github.com/mogensen/kubernetes-split-yaml/releases/download/v${KUBERNETES_SPLIT_YAML_VERSION}/kubernetes-split-yaml_${KUBERNETES_SPLIT_YAML_VERSION}_linux_amd64.tar.gz | \
    tar xz kubernetes-split-yaml -C /usr/bin/ && \
    chmod +x /usr/bin/kubernetes-split-yaml && \
    kubernetes-split-yaml -h

COPY openapi2jsonschema.py /usr/bin/openapi2jsonschema
RUN chmod +x /usr/bin/openapi2jsonschema

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
