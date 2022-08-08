#!/usr/bin/env bash

# Copyright 2022 Chainguard, Inc.
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

if [[ ! -f imagerefs ]]; then
    echo "imagerefs not found"
    exit 1
fi

echo "Signing images with Keyless..."
cosign sign --force -a GIT_HASH="$GIT_HASH" -a GIT_TAG="$GIT_TAG" "$(cat imagerefs)"
