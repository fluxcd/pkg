#!/bin/bash

set -eu

CRD_PATH=${1}
OUTPUT_DIR=${2-master-standalone}
COMBINED_FILENAME=${3}
mkdir -p ${OUTPUT_DIR}

/kubernetes-split-yaml $CRD_PATH

# this python script processes a list of files,
# so we call it once without required a loop over var FILES
FILES=generated/*
JSON_FILES=$(python /openapi2jsonschema.py $COMBINED_FILENAME ${FILES})
mv ${JSON_FILES} ${OUTPUT_DIR}

for f in ${OUTPUT_DIR}/*
do
  cat $f
done
