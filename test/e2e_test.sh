# Copyright 2021 Chainguard, Inc.
# SPDX-License-Identifier: Apache-2.0

set +o pipefail
set -e

OUTFILE=/tmp/proxyout

echo '::group:: Create the policies we use'
kubectl create -f ./testdata/cip-static-fail.yaml
kubectl create -f ./testdata/cip-static-pass.yaml
sleep 5
echo '::endgroup::'

echo '::group:: Test fail with non-fully specified image'
curl -s -X POST "http://localhost:8088/admit/policy.sigstore.dev" -H "Content-Type: application/json" -d @./testdata/testrequest.json > ${OUTFILE}
if ! grep -q "nginx must be an image digest" ${OUTFILE} ; then
  echo Did not get expected failure message, got:
  cat ${OUTFILE}
  exit 1
fi
echo '::endgroup::'

echo '::group:: Test mutate with non-fully specified image'
curl -s -X POST "http://localhost:8088/mutate/policy.sigstore.dev" -H "Content-Type: application/json" -d @./testdata/testrequest.json > ${OUTFILE}
if ! grep -q "JSONPatch" ${OUTFILE} ; then
  echo Did not get expected failure message, got:
  cat ${OUTFILE}
  exit 1
fi

echo '::group:: Test mutate with fully specified image'
curl -s -X POST "http://localhost:8088/admit/policy.sigstore.dev" -H "Content-Type: application/json" -d @./testdata/testrequest-full-image.json > ${OUTFILE}
if ! grep -q "disallowed by static policy" ${OUTFILE} ; then
  echo Did not get expected failure message, got:
  cat ${OUTFILE}
  exit 1
fi
