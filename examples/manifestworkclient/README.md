# gRPC Source ManifestWork Client

This example shows how to build a source ManifestWork client with `RESTFullAPIWatcherStore` and watch/create/get/update/delete works by the client.

## Build the client

Using sdk-go to build a source ManifestWork client with `RESTFullAPIWatcherStore`

```golang
sourceID := "mw-client-example"

workClient, err := work.NewClientHolderBuilder(grpcOptions).
    WithClientID(fmt.Sprintf("%s-client", sourceID)).
    WithSourceID(sourceID).
    WithCodecs(codec.NewManifestBundleCodec()).
    WithWorkClientWatcherStore(grpcsource.NewRESTFullAPIWatcherStore(ctx, maestroAPIClient, sourceID)).
    WithResyncEnabled(false).
    NewSourceClientHolder(ctx)

if err != nil {
    log.Fatal(err)
}

// watch/create/patch/get/delete/list by workClient
```


## Run the example

1. Run `make e2e-test/setup` to prepare the environment

2. Run the client-a `go run examples/manifestworkclient/client-a/main.go --consumer-name=$(cat test/e2e/.consumer_name)` to watch the status of works in a terminal

3. Run the client-b `go run examples/manifestworkclient/client-b/main.go --consumer-name=$(cat test/e2e/.consumer_name)` to create/get/update/delete work in other new terminal

The output of the client-b

```
the work 432999ad-b13e-4f24-8c0b-ceca24ad79ba/work-dpfqk (uid=b33b9c35-0dd4-5b4d-b419-fe9a21045fa7) is created
the work 432999ad-b13e-4f24-8c0b-ceca24ad79ba/work-dpfqk (uid=b33b9c35-0dd4-5b4d-b419-fe9a21045fa7) is updated
the work 432999ad-b13e-4f24-8c0b-ceca24ad79ba/work-dpfqk (uid=b33b9c35-0dd4-5b4d-b419-fe9a21045fa7) is deleted
```

The output of the client-a

```
watched work (uid=b33b9c35-0dd4-5b4d-b419-fe9a21045fa7) is modified
{
  "metadata": {
    "name": "work-dpfqk",
    "namespace": "432999ad-b13e-4f24-8c0b-ceca24ad79ba",
    "uid": "b33b9c35-0dd4-5b4d-b419-fe9a21045fa7",
    "resourceVersion": "2",
    "creationTimestamp": null
    "labels": {
      "work.label": "example"
    },
    "annotations": {
      "work.annotations": "example"
    }
  },
  "spec": {
    "workload": {
      "manifests": [
        {
          "apiVersion": "v1",
          "data": {
            "test": "zpchv"
          },
          "kind": "ConfigMap",
          "metadata": {
            "name": "work-dpfqk",
            "namespace": "default"
          }
        }
      ]
    }
  },
  "status": {
    "conditions": [
      {
        "type": "Applied",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:13Z",
        "reason": "AppliedManifestWorkComplete",
        "message": "Apply manifest work complete"
      },
      {
        "type": "Available",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:13Z",
        "reason": "ResourcesAvailable",
        "message": "All resources are available"
      }
    ],
    "resourceStatus": {
      "manifests": [
        {
          "resourceMeta": {
            "ordinal": 0,
            "group": "",
            "version": "v1",
            "kind": "ConfigMap",
            "resource": "configmaps",
            "name": "work-dpfqk",
            "namespace": "default"
          },
          "statusFeedback": {},
          "conditions": [
            {
              "type": "Applied",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "AppliedManifestComplete",
              "message": "Apply manifest complete"
            },
            {
              "type": "Available",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "ResourceAvailable",
              "message": "Resource is available"
            },
            {
              "type": "StatusFeedbackSynced",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "NoStatusFeedbackSynced",
              "message": ""
            }
          ]
        }
      ]
    }
  }
}
watched work (uid=b33b9c35-0dd4-5b4d-b419-fe9a21045fa7) is modified
{
  "metadata": {
    "name": "work-dpfqk",
    "namespace": "432999ad-b13e-4f24-8c0b-ceca24ad79ba",
    "uid": "b33b9c35-0dd4-5b4d-b419-fe9a21045fa7",
    "resourceVersion": "2",
    "creationTimestamp": null,
    "labels": {
      "work.label": "example"
    },
    "annotations": {
      "work.annotations": "example"
    }
  },
  "spec": {
    "workload": {
      "manifests": [
        {
          "apiVersion": "v1",
          "data": {
            "test": "zpchv"
          },
          "kind": "ConfigMap",
          "metadata": {
            "name": "work-dpfqk",
            "namespace": "default"
          }
        }
      ]
    }
  },
  "status": {
    "conditions": [
      {
        "type": "Applied",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:13Z",
        "reason": "AppliedManifestWorkComplete",
        "message": "Apply manifest work complete"
      },
      {
        "type": "Available",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:13Z",
        "reason": "ResourcesAvailable",
        "message": "All resources are available"
      }
    ],
    "resourceStatus": {
      "manifests": [
        {
          "resourceMeta": {
            "ordinal": 0,
            "group": "",
            "version": "v1",
            "kind": "ConfigMap",
            "resource": "configmaps",
            "name": "work-dpfqk",
            "namespace": "default"
          },
          "statusFeedback": {},
          "conditions": [
            {
              "type": "Applied",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "AppliedManifestComplete",
              "message": "Apply manifest complete"
            },
            {
              "type": "Available",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "ResourceAvailable",
              "message": "Resource is available"
            },
            {
              "type": "StatusFeedbackSynced",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "NoStatusFeedbackSynced",
              "message": ""
            }
          ]
        }
      ]
    }
  }
}
watched work (uid=b33b9c35-0dd4-5b4d-b419-fe9a21045fa7) is deleted
{
  "metadata": {
    "name": "work-dpfqk",
    "namespace": "432999ad-b13e-4f24-8c0b-ceca24ad79ba",
    "uid": "b33b9c35-0dd4-5b4d-b419-fe9a21045fa7",
    "resourceVersion": "1",
    "creationTimestamp": null,
    "labels": {
      "work.label": "example"
    },
    "annotations": {
      "work.annotations": "example"
    }
  },
  "spec": {
    "workload": {
      "manifests": [
        {
          "apiVersion": "v1",
          "data": {
            "test": "zpchv"
          },
          "kind": "ConfigMap",
          "metadata": {
            "name": "work-dpfqk",
            "namespace": "default"
          }
        }
      ]
    }
  },
  "status": {
    "conditions": [
      {
        "type": "Applied",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:13Z",
        "reason": "AppliedManifestWorkComplete",
        "message": "Apply manifest work complete"
      },
      {
        "type": "Available",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:13Z",
        "reason": "ResourcesAvailable",
        "message": "All resources are available"
      },
      {
        "type": "Deleted",
        "status": "True",
        "lastTransitionTime": "2024-06-11T03:49:18Z",
        "reason": "ManifestsDeleted",
        "message": "The manifests are deleted from the cluster 432999ad-b13e-4f24-8c0b-ceca24ad79ba"
      }
    ],
    "resourceStatus": {
      "manifests": [
        {
          "resourceMeta": {
            "ordinal": 0,
            "group": "",
            "version": "v1",
            "kind": "ConfigMap",
            "resource": "configmaps",
            "name": "work-dpfqk",
            "namespace": "default"
          },
          "statusFeedback": {},
          "conditions": [
            {
              "type": "Applied",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "AppliedManifestComplete",
              "message": "Apply manifest complete"
            },
            {
              "type": "Available",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "ResourceAvailable",
              "message": "Resource is available"
            },
            {
              "type": "StatusFeedbackSynced",
              "status": "True",
              "lastTransitionTime": "2024-06-11T03:49:13Z",
              "reason": "NoStatusFeedbackSynced",
              "message": ""
            }
          ]
        }
      ]
    }
  }
}
```
