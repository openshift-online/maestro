#!/usr/bin/env bash

ARO_HCP_REPO_PATH="$HOME/go/src/github.com/Azure/ARO-HCP"

ls _output/performance/aro/pids | xargs kill
kind delete clusters --all

pushd $ARO_HCP_REPO_PATH/dev-infrastructure
AKSCONFIG=svc-cluster make clean
popd
