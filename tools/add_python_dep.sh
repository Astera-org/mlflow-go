#!/usr/bin/env bash

set -o errexit
set -o pipefail

if [[ "$#" -ne 1 ]]; then
    echo "Usage: $0 <python package>"
    exit 1
fi

cd "$(dirname $(dirname "$0"))"

VENV_DIR="$(mktemp -d)"

rm -rf "${VENV_DIR}"

# Use the Python interpeter that Bazel uses
bazel run @python_3_11//:python3 -- -m venv "${VENV_DIR}"

# Create a venv with existing dependencies
source "${VENV_DIR}/bin/activate"
pip install -U pip
pip install -r python_requirements_lock.txt

# add the new dependency
pip install "$@"

# freeze the new requirements
pip freeze --all > python_requirements_lock.txt
# bazel run //:gazelle_python_manifest.update

deactivate
