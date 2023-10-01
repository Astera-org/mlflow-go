#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset
set -o xtrace

tempdir=$(mktemp -d)

release_url=$(curl --location --silent https://api.github.com/repos/mlflow/mlflow/releases/latest | grep zipball_url | cut -d '"' -f 4)
curl --location --output "${tempdir}/mlflow.zip" "${release_url}"
unzip "${tempdir}/mlflow.zip" '*/mlflow/protos/**.proto' -d "${tempdir}"

cd "$(dirname "$0")/.."
rm -rf protos
cp -r ${tempdir}/mlflow-*/mlflow/protos .
git restore protos/BUILD.bazel

# These have name conflicts with the others, and we don't need them.
rm protos/mlflow_artifacts.proto protos/databricks_uc_registry_messages.proto protos/databricks_uc_registry_service.proto

# generate the Go code
bazel build //protos:protos_go @org_golang_x_tools//cmd/goimports:goimports
cp bazel-bin/protos/protos_go_/github.com/Astera-org/mlflow-go/protos/*.go protos
# format the generated code
find protos -name "*.go" -exec chmod +w {} \;
find protos -name "*.go" -exec bazel-bin/external/org_golang_x_tools/cmd/goimports/goimports_/goimports -w {} \;

echo "# Protocol Buffers

The .proto files in this directory are copied from the official mlflow repo.

Unfortunately not everybody uses Bazel, and so we have to check in the generated
protocol buffer Go code.
To download the latest .proto files and regenerate the .pb.go files, run
[update_protos.sh](/tools/update_protos.sh).

These were copied from MLFlow version $(basename ${release_url})." > protos/README.md
