#!/bin/bash

# Copyright 2018 Istio Authors

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)
ROOT=$(dirname "$WD")

# Runs after a submit is merged to master:
# - run the unit tests, in local environment
# - push the docker images to gcr.io

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print commands
set -x

# shellcheck source=prow/lib.sh
source "${ROOT}/prow/lib.sh"
setup_and_export_git_sha

cd "$ROOT"

function create_gcb_env() {
  mkdir /workspace
  mkdir /output
  mkdir "$ROOT/../../../../src"
  touch /workspace/manifest.txt

cat << EOF > "/workspace/gcb_env.sh"
export CB_BRANCH="${GIT_SHA}"
export CB_VERSION="${GIT_SHA}"
export CB_ISTIOCTL_DOCKER_HUB="docker.io/istio"
export CB_PIPELINE_TYPE=daily
EOF
}

create_gcb_env
clone_cni

time ./release/gcb/cloud_builder.sh
