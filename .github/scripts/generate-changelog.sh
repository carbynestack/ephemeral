#!/bin/bash

#
# Copyright (c) 2021 - for information on the respective copyright owner
# see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
#
# SPDX-License-Identifier: Apache-2.0
#

if [[ -z "${VERSION}" ]]; then
  echo "Artifact version must be defined in env variable \"VERSION\""
  exit -1
fi

if [[ -z  "${GITHUB_WORKSPACE}" ]]; then
  echo "Repository base directory must be defined in env variable \"GITHUB_WORKSPACE\""
  exit -2
fi

tmpChangelog=/tmp/cs.repository-changelog.${VERSION}
touch $tmpChangelog

appendGitLog() {
  recentTag=$(git describe --tags --abbrev=0)
  if (( $? != 0 )); then
    git log --all --format="- %s" --no-merges >> $1
  else
    git log ${recentTag}..HEAD --format="- %s" --no-merges >> $1
  fi
  echo -e "" >> $1
}

appendMavenArtifactRefs() {
  if [ ! -f "${GITHUB_WORKSPACE}/pom.xml" ]; then
    echo "Project is not a Maven project"
    return
  fi

  maven_artifacts=($(cd ${GITHUB_WORKSPACE} && cat pom.xml| grep -E "<module>(.*)</module>" | sed 's/<.*>\(.*\)<\/.*>/\1/g'))

  if [ ${#maven_artifacts[@]} -eq 0 ]; then
    echo "No Maven modules found - apparently single module project"
    maven_artifacts=($(cd ${GITHUB_WORKSPACE} && cat pom.xml| grep -m 1 -E "<artifactId>(.*)</artifactId>" | sed 's/<.*>\(.*\)<\/.*>/\1/g'))
  fi
  if [ ${#maven_artifacts[@]} -eq 0 ]; then
    echo "No Maven artifact found"
    return
  fi

  echo -e "## Maven Artifacts\n" >> $1
  for artifact in ${maven_artifacts[@]}; do
    echo -e "### ${artifact}" >> $1
    echo -e "\`\`\`xml" >> $1
    echo -e "<dependency>" >> $1
    echo -e "  <groupId>io.carbynestack</groupId>" >> $1
    echo -e "  <artifactId>${artifact}</artifactId>" >> $1
    echo -e "  <version>${VERSION}</version>" >> $1
    echo -e "</dependency>" >> $1
    echo -e "\`\`\`" >> $1
  done
}

echo -e "# Change Log\n" > $tmpChangelog

appendGitLog $tmpChangelog
appendMavenArtifactRefs $tmpChangelog
