#!/bin/bash
#
# Copyright (c) 2021 - for information on the respective copyright owner
# see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
#
# SPDX-License-Identifier: Apache-2.0
#

#
# check that required tools are installed
#
if ! command -v jq &> /dev/null
then
    echo "jq could not be found! See https://stedolan.github.io/jq/download/"
    exit 1
fi
if ! command -v license-detector &> /dev/null
then
    echo "license-detector could not be found! See https://github.com/go-enry/go-license-detector"
    exit 1
fi
if ! command -v sponge &> /dev/null
then
    echo "sponge could not be found! See https://command-not-found.com/sponge"
    exit 1
fi
# Traverses the vendor folder and collects license information in the 3RD-PARTY-LICENSES/sbom.json file. License and
# notice files are copied to the respective subfolder in the 3RD-PARTY-LICENSES folder.
SBOM_FILE="3RD-PARTY-LICENSES/sbom.json"
echo "[]" > "${SBOM_FILE}"
COUNT=$(find vendor -type d | wc -l)
POS=0
FOUND=0
RES_FILE=$(mktemp /tmp/result.XXXXXX)
echo "Traversing ${COUNT} directories"
find vendor -type d | while IFS= read -r d; do
  echo -ne "\r${POS}/${COUNT}: ${FOUND} licenses found"
  license-detector "$d" -f json > "${RES_FILE}"
  if ! grep -q "error" "${RES_FILE}"; then
    jq -s ".[0] + [.[1][] | { project: .project, license: .matches[0].license }]" "${SBOM_FILE}" "${RES_FILE}" | sponge "${SBOM_FILE}"
    ARTIFACT_FOLDER="3RD-PARTY-LICENSES/${d#*/}"
    mkdir -p "${ARTIFACT_FOLDER}"
    cp "${d}"/LICENSE* "${d}"/LICENCE* "${d}"/Licence* "${d}"/NOTICE* "${ARTIFACT_FOLDER}" 2>/dev/null || true
    ((FOUND++))
  fi
  ((POS++))
done
echo -ne "\nDONE"