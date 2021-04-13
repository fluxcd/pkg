#!/bin/bash

set -eu

CRD_PATH=${1}
OUTPUT_DIR=${2-master-standalone}
mkdir -p ${OUTPUT_DIR}

/kubernetes-split-yaml $CRD_PATH

FILES=generated/*
for f in $FILES
do
  JSON=$(python /openapi2jsonschema.py ${f})
  mv ${JSON} ${OUTPUT_DIR}
done

echo "OpenAPI JSON schemas saved to ${OUTPUT_DIR}"
for f in ${OUTPUT_DIR}/*
do
  cat $f
done
