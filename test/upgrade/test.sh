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

#### get the latest image tag from quay.io as the last release image
img_registry="quay.io/redhat-user-workloads/maestro-rhtap-tenant"
last_tag=${last_tag:-"latest"}

#### Initial test env with last stable release
# 1. deploy the last maestro server and agent
image_tag=$last_tag external_image_registry=$img_registry internal_image_registry=$img_registry make test-env/deploy-server
image_tag=$last_tag external_image_registry=$img_registry internal_image_registry=$img_registry make test-env/deploy-agent
# 2. deploy the mock workserver with the last maestro grpc work client and init the workloads
IMAGE="$img_registry/maestro-e2e:$last_tag" ${PWD}/test/upgrade/script/run.sh

#### server upgrade test
# 1. upgrade the maestro server with the latest image
make test-env/deploy-server
# 2. run last upgrade test with the latest maestro server
IMAGE="$img_registry/maestro-e2e:$last_tag" ${PWD}/test/upgrade/script/run.sh
# 3. run last e2e test with the latest maestro server
IMAGE="$img_registry/maestro-e2e:$last_tag" ${PWD}/test/e2e/istio/test.sh

#### maestro agent and grpc work client upgrade test
# 1. upgrade the maestro agent and the workserver with the latest image
make test-env/deploy-agent
# 2. run upgrade test with the latest test image
${PWD}/test/upgrade/script/run.sh
# 3. run e2e test with the latest maestro agent and grpc work client
${PWD}/test/e2e/istio/test.sh
