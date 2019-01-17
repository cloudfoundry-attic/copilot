#!/bin/bash
# Copyright 2017 Istio Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
################################################################################

# This script primarily exists for Cloud Builder.  This script
# reads artifacts from a specified directory, generates tar files
# based on those artifacts, and then stores the tar files
# back to the directory.

TEMP_DIR="$(mktemp -d /tmp/istio.version.XXXX)"
BASE_DIR="$TEMP_DIR"
ISTIOCTL_SUBDIR=istioctl
OUTPUT_PATH=""
VER_STRING=""

function cleanup() {
  rm -rf "$TEMP_DIR"
}

# do cleanup before the script exits
trap cleanup EXIT

function usage() {
  echo "$0
    -d <path> path to use for temp directory                  (optional, randomized default is ${BASE_DIR} )
    -o <path> path where build output/artifacts are stored    (required)
    -i <name> subdirectory in -o path to use for istioctl     (optional)
    -v <ver>  version info to include in filename (e.g., 1.0) (required)"
  exit 1
}

function error_exit() {
  # ${BASH_SOURCE[1]} is the file name of the caller.
  echo "${BASH_SOURCE[1]}: line ${BASH_LINENO[0]}: ${1:-Unknown Error.} (exit ${2:-1})" 1>&2
  exit "${2:-1}"
}

# since there are 2 required options, should show usage and exit with no args specified
if (($# == 0)); then
  usage
fi

while getopts d:i:o:v: arg ; do
  case "${arg}" in
    d) BASE_DIR="${OPTARG}";;
    i) ISTIOCTL_SUBDIR="${OPTARG}";;
    o) OUTPUT_PATH="${OPTARG}";;
    v) VER_STRING="${OPTARG}";;
    *) usage;;
  esac
done

set -o errexit
set -o pipefail
set -x

[[ -z "${BASE_DIR}"    ]] && usage
[[ -z "${OUTPUT_PATH}" ]] && usage
[[ -z "${VER_STRING}"  ]] && usage

COMMON_FILES_DIR="${BASE_DIR}/istio/istio-${VER_STRING}"
BIN_DIR="${COMMON_FILES_DIR}/bin"
mkdir -p "${BIN_DIR}"

# On mac, brew install gnu-tar gnu-cp
# and set CP="gcp" TAR="gtar"

CP=${CP:-"cp"}
TAR=${TAR:-"tar"}

function create_linux_archive() {
  local istioctl_path="${BIN_DIR}/istioctl"

  ${CP} "${OUTPUT_PATH}/${ISTIOCTL_SUBDIR}/istioctl-linux" "${istioctl_path}"
  chmod 755 "${istioctl_path}"

  ${TAR} --owner releng --group releng -czf \
    "${OUTPUT_PATH}/istio-${VER_STRING}-linux.tar.gz" "istio-${VER_STRING}" \
    || error_exit 'Could not create linux archive'
  rm "${istioctl_path}"
}

function create_osx_archive() {
  local istioctl_path="${BIN_DIR}/istioctl"

  ${CP} "${OUTPUT_PATH}/${ISTIOCTL_SUBDIR}/istioctl-osx" "${istioctl_path}"
  chmod 755 "${istioctl_path}"

  ${TAR} --owner releng --group releng -czf \
    "${OUTPUT_PATH}/istio-${VER_STRING}-osx.tar.gz" "istio-${VER_STRING}" \
    || error_exit 'Could not create osx archive'
  rm "${istioctl_path}"
}

function create_windows_archive() {
  local istioctl_path="${BIN_DIR}/istioctl.exe"

  ${CP} "${OUTPUT_PATH}/${ISTIOCTL_SUBDIR}/istioctl-win.exe" "${istioctl_path}"

  zip -r -q "${OUTPUT_PATH}/istio-${VER_STRING}-win.zip" "istio-${VER_STRING}" \
    || error_exit 'Could not create windows archive'
  rm "${istioctl_path}"
}

pushd "${OUTPUT_PATH}"
${CP} istio.VERSION LICENSE README.md "${COMMON_FILES_DIR}"/
find samples install -type f \( \
  -name "*.yaml" \
  -o -name "*.yml" \
  -o -name "*.json" \
  -o -name "*.cfg" \
  -o -name "*.j2" \
  -o -name "cleanup*" \
  -o -name "*.md" \
  -o -name "*.conf" \
  -o -name "*.pem" \
  -o -name "*.tpl" \
  -o -name "*.txt" \
  -o -name "kubeconfig" \
  -o -name "*.jinja*" \
  -o -name "webhook-create-signed-cert.sh" \
  -o -name "webhook-patch-ca-bundle.sh" \
  \) \
  -exec "${CP}" --parents {} "${COMMON_FILES_DIR}" \;
find install/tools -type f -exec "${CP}" --parents {} "${COMMON_FILES_DIR}" \;
find tools -type f -not -name "githubContrib*" -not -name ".*" -exec "${CP}" --parents {} "${COMMON_FILES_DIR}" \;
popd

for unwanted_manifest in \
    istio-one-namespace.yaml \
    istio-one-namespace-auth.yaml \
    istio-multicluster.yaml \
    istio-auth-multicluster.yaml \
    istio.yaml \
    addons/zipkin.yaml \
    istio-auth.yaml \
    istio-remote.yaml; do
  rm -f "${COMMON_FILES_DIR}/install/kubernetes/${unwanted_manifest}"
done

ls -l  "${COMMON_FILES_DIR}/install/kubernetes/"


for unwanted_values_yaml in \
    values-istio.yaml \
    values-istio-one-namespace.yaml \
    values-istio-one-namespace-auth.yaml \
    values-istio-auth.yaml; do
  rm -f "${COMMON_FILES_DIR}/install/kubernetes/helm/istio/${unwanted_values_yaml}"
done

ls -l  "${COMMON_FILES_DIR}/install/kubernetes/helm/istio"

# Changing dir such that tar and zip files are
# created with right hiereachy
pushd "${COMMON_FILES_DIR}/.."
create_linux_archive
create_osx_archive
create_windows_archive
popd

