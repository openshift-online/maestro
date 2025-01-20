# Maestro Resource in Database

```psql
# select jsonb_pretty(payload) from resources;
                                jsonb_pretty
-----------------------------------------------------------------------------
 {                                                                          +
     "id": "0fd2b15f-e4d5-4ecb-a5fb-455284d27e2b",                          +
     "data": {                                                              +
         "manifests": [                                                     +
             {                                                              +
                 "kind": "Deployment",                                      +
                 "spec": {                                                  +
                     "replicas": 1,                                         +
                     "selector": {                                          +
                         "matchLabels": {                                   +
                             "app": "nginx"                                 +
                         }                                                  +
                     },                                                     +
                     "template": {                                          +
                         "spec": {                                          +
                             "containers": [                                +
                                 {                                          +
                                     "name": "nginx",                       +
                                     "image": "nginxinc/nginx-unprivileged",+
                                     "imagePullPolicy": "IfNotPresent"      +
                                 }                                          +
                             ],                                             +
                             "serviceAccount": "default"                    +
                         },                                                 +
                         "metadata": {                                      +
                             "labels": {                                    +
                                 "app": "nginx"                             +
                             }                                              +
                         }                                                  +
                     }                                                      +
                 },                                                         +
                 "metadata": {                                              +
                     "name": "nginx",                                       +
                     "namespace": "default"                                 +
                 },                                                         +
                 "apiVersion": "apps/v1"                                    +
             }                                                              +
         ],                                                                 +
         "deleteOption": {                                                  +
             "propagationPolicy": "Foreground"                              +
         },                                                                 +
         "manifestConfigs": [                                               +
             {                                                              +
                 "feedbackRules": [                                         +
                     {                                                      +
                         "type": "JSONPaths",                               +
                         "jsonPaths": [                                     +
                             {                                              +
                                 "name": "status",                          +
                                 "path": ".status"                          +
                             }                                              +
                         ]                                                  +
                     }                                                      +
                 ],                                                         +
                 "updateStrategy": {                                        +
                     "type": "ServerSideApply"                              +
                 },                                                         +
                 "resourceIdentifier": {                                    +
                     "name": "nginx",                                       +
                     "group": "apps",                                       +
                     "resource": "deployments",                             +
                     "namespace": "default"                                 +
                 }                                                          +
             }                                                              +
         ]                                                                  +
     },                                                                     +
     "time": "2025-01-20T11:04:54.621227032Z",                              +
     "type": "....",                                                        +
     "source": "maestro",                                                   +
     "clustername": "",                                                     +
     "specversion": "1.0",                                                  +
     "originalsource": "",                                                  +
     "datacontenttype": "application/json"                                  +
 }
(1 row)


# select jsonb_pretty(status) from resources;
                              jsonb_pretty
--------------------------------------------------------------------
  {
      "id": "0fd2b15f-e4d5-4ecb-a5fb-455284d27e2b",
      "data": {
          "status": {
              "conditions": [
                  {
                      "type": "Applied",
                      "reason": "AppliedManifestComplete",
                      "status": "True",
                      "message": "Apply manifest complete",
                      "lastTransitionTime": "2024-03-18T12:57:58Z"
                  },
                  {
                      "type": "Available",
                      "reason": "ResourceAvailable",
                      "status": "True",
                      "message": "Resource is available",
                      "lastTransitionTime": "2024-03-18T12:57:58Z"
                  },
                  {
                      "type": "StatusFeedbackSynced",
                      "reason": "StatusFeedbackSynced",
                      "status": "True",
                      "message": "",
                      "lastTransitionTime": "2024-03-18T12:57:58Z"
                  }
              ],
              "resourceMeta": {
                  "kind": "Deployment",
                  "name": "nginx1",
                  "group": "apps",
                  "ordinal": 0,
                  "version": "v1",
                  "resource": "deployments",
                  "namespace": "default"
              },
              "statusFeedback": {
                  "values": [
                      {
                          "name": "status",
                          "fieldValue": {
                              "type": "JsonRaw",
                              "jsonRaw": "{\"availableReplicas\":1,\"conditions\":[{\"lastTransitionTime\":\"2024-03-18T12:58:04Z\",\"lastUpdateTime\":\"2024-03-18T12:58:04Z\",\"m
  essage\":\"Deployment has minimum availability.\",\"reason\":\"MinimumReplicasAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2024-03-18T12:57:58Z\",\"lastUpdateTime\":\"2024-03-18T12:58:04Z\",\"message\":\"ReplicaSet \\\"nginx1-5d6b548959\\\" has successfully progressed.\",\"reason\":\"NewReplicaSetAvailable\",\"status\
  ":\"True\",\"type\":\"Progressing\"}],\"observedGeneration\":1,\"readyReplicas\":1,\"replicas\":1,\"updatedReplicas\":1}"
                          }
                      }
                  ]
              }
          },
          "conditions": [
              {
                  "type": "Applied",
                  "reason": "AppliedManifestWorkComplete",
                  "status": "True",
                  "message": "Apply manifest work complete",
                  "lastTransitionTime": "2024-03-18T12:57:58Z"
              },
              {
                  "type": "Available",
                  "reason": "ResourcesAvailable",
                  "status": "True",
                  "message": "All resources are available",
                  "lastTransitionTime": "2024-03-18T12:57:58Z"
              }
          ]
      },
      "time": "2024-03-18T12:58:11.020848168Z",
      "type": "io.open-cluster-management.works.v1alpha1.manifests.status.update_request",
      "source": "b288a9da-8bfe-4c82-94cc-2b48e773fc46-work-agent",
      "resourceid": "dc970bd3-da6d-4a63-992b-dc0ae0419a7c",
      "sequenceid": "1769709885668200448",
      "clustername": "b288a9da-8bfe-4c82-94cc-2b48e773fc46",
      "specversion": "1.0",
      "originalsource": "maestro",
      "datacontenttype": "application/json",
      "resourceversion": "1"
  }
(1 row)
```
