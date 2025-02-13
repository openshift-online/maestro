#!/bin/bash -ex
#
# Copyright (c) 2023 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# get the last image tag from quay.io
img_repo_api="https://quay.io/api/v1/repository/redhat-user-workloads/maestro-rhtap-tenant/maestro/maestro"
img_registry="quay.io/redhat-user-workloads/maestro-rhtap-tenant"
last_tag=$(curl -s -X GET "${img_repo_api}" | jq -s -c -r 'sort_by(.tags[].last_modified) | .[].tags[].name' | grep -E '^[a-z0-9]{40}$' | head -n 1)

# use the last tag as the default commit sha
commit_sha=${commit_sha:-"$last_tag"}

output_dir="./_output/migration"

rm -rf $output_dir
mkdir -p $output_dir

# run the e2e-test in the main repo with the commit sha
git clone https://github.com/openshift-online/maestro.git "$output_dir/maestro"
pushd $output_dir/maestro
git checkout $commit_sha
image_tag=$commit_sha external_image_registry=$img_registry internal_image_registry=$img_registry make e2e-test
popd

# copy the configurations
cp $output_dir/maestro/test/e2e/.kubeconfig ./test/e2e/.kubeconfig
cp $output_dir/maestro/test/e2e/.consumer_name ./test/e2e/.consumer_name
cp $output_dir/maestro/test/e2e/.external_host_ip ./test/e2e/.external_host_ip
cp -r $output_dir/maestro/test/e2e/certs ./test/e2e/certs

# run the e2e test in the current repo (upgrade)
make e2e-test/setup

sleep 180 # wait for the upgrade env is ready

make e2e-test/run
