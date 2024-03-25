#!/usr/bin/env python

# Copyright 2021 The Flux authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script generates OpenAPI v3 JSON schema from Kubernetes CRD YAML
# Derived from https://github.com/yannh/kubeconform
# Derived from https://github.com/instrumenta/openapi2jsonschema

import yaml
import json
import sys
import os
import urllib.request
from dataclasses import dataclass
from typing import Any


@dataclass
class Schema:
    kind: str
    group: str
    version: str
    definition: dict[Any, Any]

def additional_properties(data):
    "This recreates the behaviour of kubectl at https://github.com/kubernetes/kubernetes/blob/225b9119d6a8f03fcbe3cc3d590c261965d928d0/pkg/kubectl/validation/schema.go#L312"
    new = {}
    try:
        for k, v in data.items():
            new_v = v
            if isinstance(v, dict):
                if "properties" in v:
                    if "additionalProperties" not in v:
                        v["additionalProperties"] = False
                new_v = additional_properties(v)
            else:
                new_v = v
            new[k] = new_v
        return new
    except AttributeError:
        return data


def replace_int_or_string(data):
    new = {}
    try:
        for k, v in data.items():
            new_v = v
            if isinstance(v, dict):
                if "format" in v and v["format"] == "int-or-string":
                    new_v = {"oneOf": [{"type": "string"}, {"type": "integer"}]}
                else:
                    new_v = replace_int_or_string(v)
            elif isinstance(v, list):
                new_v = list()
                for x in v:
                    new_v.append(replace_int_or_string(x))
            else:
                new_v = v
            new[k] = new_v
        return new
    except AttributeError:
        return data


def allow_null_optional_fields(data, parent=None, grand_parent=None, key=None):
    new = {}
    try:
        for k, v in data.items():
            new_v = v
            if isinstance(v, dict):
                new_v = allow_null_optional_fields(v, data, parent, k)
            elif isinstance(v, list):
                new_v = list()
                for x in v:
                    new_v.append(allow_null_optional_fields(x, v, parent, k))
            elif isinstance(v, str):
                is_non_null_type = k == "type" and v != "null"
                has_required_fields = grand_parent and "required" in grand_parent
                if is_non_null_type and not has_required_field:
                    new_v = [v, "null"]
            new[k] = new_v
        return new
    except AttributeError:
        return data


def append_no_duplicates(obj, key, value):
    """
    Given a dictionary, lookup the given key, if it doesn't exist create a new array.
    Then check if the given value already exists in the array, if it doesn't add it.
    """
    if key not in obj:
        obj[key] = []
    if value not in obj[key]:
        obj[key].append(value)


def write_schema_file(schema, filename):
    schema = additional_properties(schema)
    schema = replace_int_or_string(schema)

    # Dealing with user input here..
    filename = os.path.basename(filename)
    f = open(filename, "w")
    f.write(json.dumps(schema, indent=2))
    f.close()
    print("{filename}".format(filename=filename))


if len(sys.argv) == 0:
    print("missing file")
    exit(1)

schemas: list[Schema] = []

for crdFile in sys.argv[1:]:
    if crdFile.startswith("http"):
        f = urllib.request.urlopen(crdFile)
    else:
        f = open(crdFile)
    with f:

        for y in yaml.load_all(f, Loader=yaml.SafeLoader):
            if "kind" not in y:
                continue
            if y["kind"] != "CustomResourceDefinition":
                continue

            if "spec" in y and "validation" in y["spec"] and "openAPIV3Schema" in y["spec"]["validation"]:
                schemas.append(Schema(
                    kind=y["spec"]["names"]["kind"],
                    group=y["spec"]["group"],
                    version=y["spec"]["version"],
                    definition=y["spec"]["validation"]["openAPIV3Schema"]
                ))

            elif "spec" in y and "versions" in y["spec"]:
                for version in y["spec"]["versions"]:
                    if "schema" in version and "openAPIV3Schema" in version["schema"]:
                        schemas.append(Schema(
                            kind=y["spec"]["names"]["kind"],
                            group=y["spec"]["group"],
                            version=version["name"],
                            definition=version["schema"]["openAPIV3Schema"]
                        ))


filename_format = os.getenv("FILENAME_FORMAT", "{kind}-{group}-{version}")

# Write down all separate schema files.
for schema in schemas:
    filename = filename_format.format(
        kind=schema.kind,
        group=schema.group.split(".")[0],
        version=schema.version,
    ).lower() + ".json"
    write_schema_file(schema.definition, filename)


# Make a single definitions file that has the enum field set for kind and apiVersion so we can have automatic matching
# for JSON schema.
with open('_definitions.json', 'w') as definitions_file:
    definitions: dict[str, dict[Any, Any]] = {}

    # NOTE: No deep copy is needed as we already wrote the schema files, so let's modify the original structures.
    for schema in schemas:
        append_no_duplicates(schema.definition['properties']['apiVersion'], 'enum', f'{schema.group}/{schema.version}')
        append_no_duplicates(schema.definition['properties']['kind'], 'enum', schema.kind)
        definitions[f'{schema.group}.{schema.version}.{schema.kind}'] = schema.definition

    definitions_file.write(json.dumps({"definitions": definitions}, indent=2))

# Finally we write a flux2 main schema file that can be used to automatically validate any Flux2 schema using oneOf
# semantics.
with open('all.json', 'w') as all_file:
    refs = [{'$ref': f'_definitions.json#/definitions/{schema.group}.{schema.version}.{schema.kind}'} for schema in schemas]
    all_file.write(json.dumps({ "oneOf": refs }, indent=2))

exit(0)
