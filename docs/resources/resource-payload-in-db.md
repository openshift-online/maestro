# Maestro Resource Payload in Database

```sql
select id, jsonb_pretty(payload) as payload from resources;
```

```
 id                                   |                                         payload
--------------------------------------+----------------------------------------------------------------------------------------------
 55c61e54-a3f6-563d-9fec-b1fe297bdfdb | {                                                                                           +
                                      |     "id": "1aaad717-7daa-4d33-b087-8e258dab9c5f",                                           +
                                      |     "data": {                                                                               +
                                      |         "manifests": [                                                                      +
                                      |             {                                                                               +
                                      |                 "kind": "Deployment",                                                       +
                                      |                 "spec": {                                                                   +
                                      |                     "replicas": 0,                                                          +
                                      |                     "selector": {                                                           +
                                      |                         "matchLabels": {                                                    +
                                      |                             "app": "nginx"                                                  +
                                      |                         }                                                                   +
                                      |                     },                                                                      +
                                      |                     "strategy": {                                                           +
                                      |                     },                                                                      +
                                      |                     "template": {                                                           +
                                      |                         "spec": {                                                           +
                                      |                             "containers": [                                                 +
                                      |                                 {                                                           +
                                      |                                     "name": "nginx",                                        +
                                      |                                     "image": "quay.io/nginx/nginx-unprivileged:latest",     +
                                      |                                     "resources": {                                          +
                                      |                                     },                                                      +
                                      |                                     "imagePullPolicy": "IfNotPresent"                       +
                                      |                                 }                                                           +
                                      |                             ]                                                               +
                                      |                         },                                                                  +
                                      |                         "metadata": {                                                       +
                                      |                             "labels": {                                                     +
                                      |                                 "app": "nginx"                                              +
                                      |                             }                                                               +
                                      |                         }                                                                   +
                                      |                     }                                                                       +
                                      |                 },                                                                          +
                                      |                 "status": {                                                                 +
                                      |                 },                                                                          +
                                      |                 "metadata": {                                                               +
                                      |                     "name": "maestro-e2e-upgrade-test",                                     +
                                      |                     "namespace": "default"                                                  +
                                      |                 },                                                                          +
                                      |                 "apiVersion": "apps/v1"                                                     +
                                      |             }                                                                               +
                                      |         ],                                                                                  +
                                      |         "manifestConfigs": [                                                                +
                                      |             {                                                                               +
                                      |                 "feedbackRules": [                                                          +
                                      |                     {                                                                       +
                                      |                         "type": "JSONPaths",                                                +
                                      |                         "jsonPaths": [                                                      +
                                      |                             {                                                               +
                                      |                                 "name": "status",                                           +
                                      |                                 "path": ".status"                                           +
                                      |                             }                                                               +
                                      |                         ]                                                                   +
                                      |                     }                                                                       +
                                      |                 ],                                                                          +
                                      |                 "updateStrategy": {                                                         +
                                      |                     "type": "ServerSideApply",                                              +
                                      |                     "serverSideApply": {                                                    +
                                      |                         "force": true,                                                      +
                                      |                         "fieldManager": "maestro-agent"                                     +
                                      |                     }                                                                       +
                                      |                 },                                                                          +
                                      |                 "resourceIdentifier": {                                                     +
                                      |                     "name": "maestro-e2e-upgrade-test",                                     +
                                      |                     "group": "apps",                                                        +
                                      |                     "resource": "deployments",                                              +
                                      |                     "namespace": "default"                                                  +
                                      |                 }                                                                           +
                                      |             }                                                                               +
                                      |         ]                                                                                   +
                                      |     },                                                                                      +
                                      |     "time": "2025-12-13T13:32:30.308693Z",                                                  +
                                      |     "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request",+
                                      |     "source": "mw-client-example",                                                          +
                                      |     "metadata": {                                                                           +
                                      |         "uid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",                                      +
                                      |         "name": "e44ec579-9646-549a-b679-db8d19d6da37",                                     +
                                      |         "labels": {                                                                         +
                                      |             "maestro.resource.type": "2d18a7dc-c731-5753-8aa9-6b2f22e3b12b"                 +
                                      |         },                                                                                  +
                                      |         "namespace": "7ff1a8f4-8811-49b4-8c19-9271bc51975e",                                +
                                      |         "resourceVersion": "0"                                                              +
                                      |     },                                                                                      +
                                      |     "logtracing": "{}",                                                                     +
                                      |     "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",                                   +
                                      |     "clustername": "7ff1a8f4-8811-49b4-8c19-9271bc51975e",                                  +
                                      |     "specversion": "1.0",                                                                   +
                                      |     "originalsource": "",                                                                   +
                                      |     "datacontenttype": "application/json",                                                  +
                                      |     "resourceversion": 0                                                                    +
                                      | }
(1 row)
```
