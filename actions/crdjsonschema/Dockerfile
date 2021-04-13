FROM python:3.9

RUN pip install --no-cache-dir pyaml

RUN curl -sL https://github.com/mogensen/kubernetes-split-yaml/releases/download/v0.3.0/kubernetes-split-yaml_0.3.0_linux_amd64.tar.gz | \
    tar xz && chmod +x /kubernetes-split-yaml && /kubernetes-split-yaml -h

COPY openapi2jsonschema.py /openapi2jsonschema.py
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
