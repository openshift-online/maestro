#!/bin/bash

work_name=${work_name:-""}
manifest_kind=${manifest_kind:-"manifestworks"}
manifest_name=${manifest_name:-""}
manifest_namespace=${manifest_namespace:-"default"}

extract_uuid() {
    local name="$1"
    echo "$name" | awk -F'-' '{
        if (NF >= 5) {
            printf "%s-", $(NF-4)
            printf "%s-", $(NF-3)
            printf "%s-", $(NF-2)
            printf "%s-", $(NF-1)
            printf "%s", $NF
        } else {
            print ""
        }
    }'
}

if [ -n "$work_name" ]; then
    result=$(kubectl get appliedmanifestworks | grep "$work_name")
    if [ -z "$result" ]; then
        echo "The work $work_name may have been deleted, check the database"
        exit 1
    fi

    amw=$(echo $result | awk '{print $1}')
    echo "The appliedmanifestwork of the work $work_name:"
    kubectl get appliedmanifestwork $amw
    echo ""
    echo "The manifests of the work $work_name:"
    kubectl get appliedmanifestwork $amw -o jsonpath='{range .status.appliedResources[*]}{.resource}{"\t"}{.namespace}{"\t"}{.name}{"\n"}{end}'
    exit 0
fi

if [ -n "$manifest_name" ]; then
    echo "Manifest Type: $manifest_kind"
    echo "Manifest Name: $manifest_name"

    owner_names=()
    while IFS= read -r name; do
        [[ -n "$name" ]] && owner_names+=("$name")
    done < <(
        if [[ -n "$manifest_namespace" ]]; then
            kubectl get "$manifest_kind" "$manifest_name" -n "$manifest_namespace" -o json 2>/dev/null
        else
            kubectl get "$manifest_kind" "$manifest_name" -o json 2>/dev/null
        fi | jq -r '.metadata.ownerReferences[].name // empty'
    )

    for name in "${owner_names[@]}"; do
        uuid=$(extract_uuid $name)
        if [ -z "$uuid" ]; then
            echo "Error: Invalid work name $name, may not be created by cs works"
            exit 1
        fi

        echo "Work Name: $uuid"
        echo "The appliedmanifestwork of the work $name:"
        kubectl get appliedmanifestwork "$name"
    done

    exit 0
fi

echo "Error: At least one of the following variables must be provided:"
echo "  - work_name"
echo "  - manifest_name"
exit 1
