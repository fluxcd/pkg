#!/usr/bin/env sh

set -e

[ -z "$1" ] && echo "No CRD file specified" && exit 1
[ -z "$2" ] && echo "No output directory specified" && exit 1

CRD_PATH=${1}
if [ ! -f "$CRD_PATH" ]; then
  echo "CRD file not found at ${CRD_PATH}"
  exit 1
fi

OUTPUT_DIR=${2}
mkdir -p "$OUTPUT_DIR"

WORK_DIR=$(mktemp -dt crd-json-schemas-XXXXXX)
trap 'rm -rf $WORK_DIR' EXIT

SPLIT_DIR="$WORK_DIR/split"
mkdir -p "$SPLIT_DIR"

kubernetes-split-yaml --outdir "$SPLIT_DIR" "$CRD_PATH"

( cd "$WORK_DIR"
  for f in "$SPLIT_DIR"/*
  do
    openapi2jsonschema "$f"
  done
)

mv "$WORK_DIR"/*.json "$OUTPUT_DIR"
echo "OpenAPI JSON schemas saved to ${OUTPUT_DIR}"
