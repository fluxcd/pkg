#!/bin/bash

set -eu

CRD_PATH=${1}
OUTPUT_DIR=${2-master-standalone}

JSON=$(python /openapi2jsonschema.py ${CRD_PATH})

mkdir -p ${OUTPUT_DIR}

mv ${JSON} ${OUTPUT_DIR}

echo "OpenAPI JSON schema saved to ${OUTPUT_DIR}/${JSON}"
