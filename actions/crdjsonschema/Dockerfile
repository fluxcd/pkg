FROM python:3.9

RUN pip install --no-cache-dir pyaml

COPY openapi2jsonschema.py /openapi2jsonschema.py
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
