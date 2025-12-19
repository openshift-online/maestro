# Maestro Resource Status in Database

```sql
select id, jsonb_pretty(status) as status from resources;
```

```
 id                                   |          status
--------------------------------------+--------------------------------------------------------------------
 55c61e54-a3f6-563d-9fec-b1fe297bdfdb |{                                                                  +
                                      |     "id": "1d5c5ec2-861d-4b9c-b7ce-6be9fae9610a",                 +
                                      |     "data": {                                                     +
                                      |         "conditions": [                                           +
                                      |             {                                                     +
                                      |                 "type": "Applied",                                +
                                      |                 "reason": "AppliedManifestWorkComplete",          +
                                      |                 "status": "True",                                 +
                                      |                 "message": "Apply manifest work complete",        +
                                      |                 "lastTransitionTime": "2025-12-13T13:32:30Z",     +
                                      |                 "observedGeneration": 1                           +
                                      |             },                                                    +
                                      |             {                                                     +
                                      |                 "type": "Available",                              +
                                      |                 "reason": "ResourcesAvailable",                   +
                                      |                 "status": "True",                                 +
                                      |                 "message": "All resources are available",         +
                                      |                 "lastTransitionTime": "2025-12-13T13:32:30Z",     +
                                      |                 "observedGeneration": 1                           +
                                      |             }                                                     +
                                      |         ],                                                        +
                                      |         "resourceStatus": [                                       +
                                      |             {                                                     +
                                      |                 "conditions": [+
                                      |                     {                                              +
                                      |                         "type": "Applied",+
                                      |                         "reason": "AppliedManifestComplete",+
                                      |                         "status": "True",+
                                      |                         "message": "Apply manifest complete",+
                                      |                         "lastTransitionTime": "2025-12-13T13:32:30Z"+
                                      |                     },+
                                      |                     {+
                                      |                         "type": "Available",+
                                      |                         "reason": "ResourceAvailable",+
                                      |                         "status": "True",+
                                      |                         "message": "Resource is available",+
                                      |                         "lastTransitionTime": "2025-12-13T13:32:30Z"+
                                      |                     },+
                                      |                     {+
                                      |                         "type": "StatusFeedbackSynced",+
                                      |                         "reason": "StatusFeedbackSynced",+
                                      |                         "status": "True",+
                                      |                         "message": "",+
                                      |                         "lastTransitionTime": "2025-12-13T13:32:30Z"+
                                      |                     }+
                                      |                 ],+
                                      |                 "resourceMeta": {+
                                      |                     "kind": "Deployment",+
                                      |                     "name": "maestro-e2e-upgrade-test",+
                                      |                     "group": "apps",+
                                      |                     "ordinal": 0,+
                                      |                     "version": "v1",+
                                      |                     "resource": "deployments",+
                                      |                     "namespace": "default"+
                                      |                 },+
                                      |                 "statusFeedback": {+
                                      |                     "values": [+
                                      |                         {+
                                      |                             "name": "status",+
                                      |                             "fieldValue": {+
                                      |                                 "type": "JsonRaw",+
                                      |                                 "jsonRaw": "{\"conditions\":[{\"lastTransitionTime\":\"2025-12-13T13:32:30Z\",\"lastUpdateTime\":\"2025-12-13T13:32:30Z\",\"message\":\"Deployment has minimum availability.\",\"reason\":\"MinimumRepli
casAvailable\",\"status\":\"True\",\"type\":\"Available\"},{\"lastTransitionTime\":\"2025-12-13T13:32:30Z\",\"lastUpdateTime\":\"2025-12-13T13:32:30Z\",\"message\":\"ReplicaSet \\\"maestro-e2e-upgrade-test-68bd67c4c7\\\" has successfully progressed\",\"reason\":\"NewReplicaSetAvailable\",\"status\":\"True\",\"type\":\"Progressing\"}],\"observedGeneration\":1}"+
                                      |                             }+
                                      |                         }+
                                      |                     ]+
                                      |                 }+
                                      |             }+
                                      |         ]+
                                      |     },+
                                      |     "time": "2025-12-13T13:32:42.264601288Z",+
                                      |     "type": "io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request",+
                                      |     "source": "7ff1a8f4-8811-49b4-8c19-9271bc51975e-work-agent",+
                                      |     "metadata": {+
                                      |         "uid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",+
                                      |         "name": "e44ec579-9646-549a-b679-db8d19d6da37",+
                                      |         "labels": {+
                                      |             "maestro.resource.type": "2d18a7dc-c731-5753-8aa9-6b2f22e3b12b"+
                                      |         },+
                                      |         "namespace": "7ff1a8f4-8811-49b4-8c19-9271bc51975e",+
                                      |         "resourceVersion": "0",+
                                      |         "creationTimestamp": "2025-12-13T13:32:30Z"+
                                      |     },+
                                      |     "logtracing": "{}",+
                                      |     "resourceid": "55c61e54-a3f6-563d-9fec-b1fe297bdfdb",+
                                      |     "sequenceid": "1999832099518943232",+
                                      |     "statushash": "7e55dc136458c2db603cda945895c50261564139010189e9ab8c7c6d570566a8",+
                                      |     "clustername": "7ff1a8f4-8811-49b4-8c19-9271bc51975e",+
                                      |     "specversion": "1.0",+
                                      |     "originalsource": "maestro",+
                                      |     "datacontenttype": "application/json",+
                                      |     "resourceversion": "1"+
                                      | }
(1 row)
```
