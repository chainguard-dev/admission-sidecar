#!/usr/bin/env bash

# Copyright 2022 Chainguard, Inc.
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

: "${GIT_HASH:?Environment variable empty or not defined.}"
: "${GIT_TAG:?Environment variable empty or not defined.}"

if [[ ! -f imagerefs ]]; then
    echo "imagerefs not found"
    exit 1
fi

echo "Signing images with Keyless..."
readarray -t server_images < <(cat imagerefs || true)
cosign sign --yes -a GIT_HASH="${GIT_HASH}" -a GIT_TAG="${GIT_TAG}" "${server_images[@]}"
cosign verify --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" "${server_images[@]}"
