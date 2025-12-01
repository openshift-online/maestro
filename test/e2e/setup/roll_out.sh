#!/usr/bin/env bash

set -ex

maestro_latest_image_sha=$(curl -s -X GET https://quay.io/api/v1/repository/redhat-user-workloads/maestro-rhtap-tenant/maestro/maestro | jq -s -c 'sort_by(.tags[].last_modified) | .[].tags[] | select(.name | test("sha256-|on-pr-|maestro-on-pull-request")| not) | .name' | head -n 1 | sed 's/\"//g')
if [ $maestro_latest_image_sha == "" ]; then
    echo "Did not find the maestro latest image, exit!"
    exit 1
fi

# if on service cluster
if [ $(kubectl get deploy -n maestro -l app=maestro --no-headers | wc -l | sed 's/\ //g') -ge 1 ]; then
    maestro_current_image=$(kubectl get deploy/maestro -n maestro -o jsonpath='{.spec.template.spec.containers[?(@.name=="service")].image}')
    if [ $maestro_current_image == "" ]; then
        echo "Did not find the maestro image, exit!"
        exit 1
    fi
    image_sha=$(echo -n $maestro_current_image | awk -F ':' '{print $2}')
    if [ $image_sha != $maestro_latest_image_sha ]; then
        # Roll out the maestro
        kubectl set image deploy/maestro -n maestro *=quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro/maestro:$maestro_latest_image_sha
        kubectl rollout status deploy/maestro -n maestro --timeout=600s
    fi
fi

# if on management cluster
if [ $(kubectl get deploy -n maestro -l app=maestro-agent --no-headers | wc -l | sed 's/\ //g') -ge 1 ]; then
    maestro_current_image=$(kubectl get deploy/maestro-agent -n maestro -o jsonpath='{.spec.template.spec.containers[?(@.name=="maestro-agent")].image}')
    if [ $maestro_current_image == "" ]; then
        echo "Did not find the maestro agent image, exit!"
        exit 1
    fi
    image_sha=$(echo -n $maestro_current_image | awk -F ':' '{print $2}')
    if [ $image_sha != $maestro_latest_image_sha ]; then
        # Roll out the maestro
        kubectl set image deploy/maestro-agent -n maestro maestro-agent=quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro/maestro:$maestro_latest_image_sha
        kubectl rollout status deploy/maestro-agent -n maestro --timeout=600s
    fi
fi

set +ex
