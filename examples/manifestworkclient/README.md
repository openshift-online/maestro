# gRPC Source ManifestWork Client

This example shows how to build a source ManifestWork client with Maestro gRPC service and watch/get/list/create/patch/delete works by this client.

## Build the client

```golang
sourceID := "mw-client-example"

workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
	ctx,
	maestroAPIClient,
	maestroGRPCOptions,
	sourceID,
)

if err != nil {
		log.Fatal(err)
}

// watch/get/list/create/patch/delete by workClient
```

## List works

The `List` of the gRPC source ManifestWork client supports to paging list works, this will help to list the works when there are a lot of works in the maestro sever, e.g.

```golang

workClient, err := ...

// There are 2500 works in the maestro, we will list these works with paging

// First list: this will return 1000 (this is controlled by `ListOptions.Limit`) works and
// with a next page (2) in the `workList.ListMeta.Continue`
workList, err := workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		Limit: 1000
})
if err != nil {
  log.Fatal(err)
}

// Second list: we list works with last returned `ListMeta.Continue`, this will also return
// 1000 works and with a next page (3) in the `workList.ListMeta.Continue`
workList, err = workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		Limit: 1000,
    Continue: workList.ListMeta.Continue,
})
if err != nil {
  log.Fatal(err)
}

// Third list: we also list works with last returned `ListMeta.Continue`, this will return
// all remaining works (500) and the `workList.ListMeta.Continue` will be empty
workList, err = workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		Limit: 1000,
    Continue: workList.ListMeta.Continue,
})
if err != nil {
  log.Fatal(err)
}
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
