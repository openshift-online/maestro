Maestro
---

## Run for the first time

### Make a build, run postgres and mqtt broker

```sh

# 1. build the project

$ go install gotest.tools/gotestsum@latest  
$ make binary

# 2. run a postgres database locally in docker 

$ make db/setup
$ make db/login
        
    root@f076ddf94520:/# psql -h localhost -U maestro maestro
    psql (14.4 (Debian 14.4-1.pgdg110+1))
    Type "help" for help.
    
    maestro=# \dt
    Did not find any relations.

# 3. run a mqtt broker locally in docker

$ make mqtt/setup
```

### Run database migrations

The initial migration will create the base data model as well as providing a way to add future migrations.

```shell

# Run migrations
./maestro migration

# Verify they ran in the database
$ make db/login

root@f076ddf94520:/# psql -h localhost -U maestro maestro
psql (14.4 (Debian 14.4-1.pgdg110+1))
Type "help" for help.

maestro=# \dt
                 List of relations
 Schema |    Name    | Type  |        Owner        
--------+------------+-------+---------------------
 public | resources  | table | maestro
 public | events     | table | maestro
 public | migrations | table | maestro
(3 rows)


```

### Test the application

```shell

make test
make test-integration

```

### Running the Service

```shell

make run

```

To verify that the server is working use the curl command:

```shell

curl http://localhost:8000/api/maestro/v1/resources | jq

```

That should return a 401 response like this, because it needs authentication:

```
{
  "kind": "Error",
  "id": "401",
  "href": "/api/maestro/errors/401",
  "code": "API-401",
  "reason": "Request doesn't contain the 'Authorization' header or the 'cs_jwt' cookie"
}
```


Authentication in the default configuration is done through the RedHat SSO, so you need to login with a Red Hat customer portal user in the right account (created as part of the onboarding doc) and then you can retrieve the token to use below on https://console.redhat.com/openshift/token
To authenticate, use the ocm tool against your local service. The ocm tool is available on https://console.redhat.com/openshift/downloads

#### Login to your local service
```
ocm login --token=${OCM_ACCESS_TOKEN} --url=http://localhost:8000

```

#### Get a new Resource
This will be empty if no Resource is ever created

```
ocm get /api/maestro/v1/resources
{
  "items": [],
  "kind": "ResourceList",
  "page": 1,
  "size": 0,
  "total": 0
}
```

#### Post a new Resource

```shell

ocm post /api/maestro/v1/resources << EOF
{
  "consumer_id": "cluster1",
  "version": 1,
  "manifest": {
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
      "name": "nginx",
      "namespace": "default"
    },
    "spec": {
      "replicas": 1,
      "selector": {
        "matchLabels": {
          "app": "nginx"
        }
      },
      "template": {
        "metadata": {
          "labels": {
            "app": "nginx"
          }
        },
        "spec": {
          "containers": [
            {
              "image": "nginxinc/nginx-unprivileged",
              "name": "nginx"
            }
          ]
        }
      }
    }
  }
}
EOF

```

#### Get your Resource

```shell
ocm get /api/maestro/v1/resources
{
  "items": [
    {
      "consumer_id": "cluster1",
      "created_at": "2023-11-23T09:26:13.43061Z",
      "href": "/api/maestro/v1/resources/f428e21d-71cb-47a4-8d7f-82a65d9a4048",
      "id": "f428e21d-71cb-47a4-8d7f-82a65d9a4048",
      "kind": "Resource",
      "manifest": {
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": {
          "name": "nginx",
          "namespace": "default"
        },
        "spec": {
          "replicas": 1,
          "selector": {
            "matchLabels": {
              "app": "nginx"
            }
          },
          "template": {
            "metadata": {
              "labels": {
                "app": "nginx"
              }
            },
            "spec": {
              "containers": [
                {
                  "image": "nginxinc/nginx-unprivileged",
                  "name": "nginx"
                }
              ]
            }
          }
        }
      },
      "status": {
        "ContentStatus": {
          "availableReplicas": 1,
          "conditions": [
            {
              "lastTransitionTime": "2023-11-23T07:05:50Z",
              "lastUpdateTime": "2023-11-23T07:05:50Z",
              "message": "Deployment has minimum availability.",
              "reason": "MinimumReplicasAvailable",
              "status": "True",
              "type": "Available"
            },
            {
              "lastTransitionTime": "2023-11-23T07:05:47Z",
              "lastUpdateTime": "2023-11-23T07:05:50Z",
              "message": "ReplicaSet \"nginx-5d6b548959\" has successfully progressed.",
              "reason": "NewReplicaSetAvailable",
              "status": "True",
              "type": "Progressing"
            }
          ],
          "observedGeneration": 1,
          "readyReplicas": 1,
          "replicas": 1,
          "updatedReplicas": 1
        },
        "ReconcileStatus": {
          "Conditions": [
            {
              "lastTransitionTime": "2023-11-23T09:26:13Z",
              "message": "Apply manifest complete",
              "reason": "AppliedManifestComplete",
              "status": "True",
              "type": "Applied"
            },
            {
              "lastTransitionTime": "2023-11-23T09:26:13Z",
              "message": "Resource is available",
              "reason": "ResourceAvailable",
              "status": "True",
              "type": "Available"
            },
            {
              "lastTransitionTime": "2023-11-23T09:26:13Z",
              "message": "",
              "reason": "StatusFeedbackSynced",
              "status": "True",
              "type": "StatusFeedbackSynced"
            }
          ],
          "ObservedGeneration": 1
        }
      },
      "updated_at": "2023-11-23T09:26:13.457419Z",
      "version": 1
    }
  ],
  "kind": "",
  "page": 1,
  "size": 1,
  "total": 1
}
```

#### Run in OpenShift

Take OpenShift Local as an example to deploy the maestro. If you want to deploy maestro in an OpenShift cluster, you need to set the `external_apps_domain` environment variable to point your cluster.
```shell
$ export external_apps_domain=`oc -n openshift-ingress-operator get ingresscontroller default -o jsonpath='{.status.domain}'`
```

Use OpenShift Local to deploy to a local openshift cluster. Be sure to have CRC running locally:

```shell
$ crc status
CRC VM:          Running
OpenShift:       Running (v4.13.12)
RAM Usage:       7.709GB of 30.79GB
Disk Usage:      23.75GB of 32.68GB (Inside the CRC VM)
Cache Usage:     37.62GB
Cache Directory: /home/mturansk/.crc/cache
```

Log into CRC:

```shell
$ make crc/login
Logging into CRC
Logged into "https://api.crc.testing:6443" as "kubeadmin" using existing credentials.

You have access to 66 projects, the list has been suppressed. You can list all projects with 'oc projects'

Using project "ocm-mturansk".
Login Succeeded!
```

Deploy maestro:

We will push the image to your OpenShift cluster default registry and then deploy it to the cluster. You need to follow [this document](https://docs.openshift.com/container-platform/4.13/registry/securing-exposing-registry.html) to expose a default registry manually and login into the registry with podman.

```shell
$ make deploy

$ oc get pod -n maestro-root
NAME                            READY   STATUS      RESTARTS   AGE
maestro-85c847764-4xdt6         1/1     Running     0          62s
maestro-db-1-deploy             0/1     Completed   0          62s
maestro-db-1-kwv4h              1/1     Running     0          61s
maestro-mqtt-6cb7bdf46c-kcczm   1/1     Running     0          63s
```

Create a consumer:

```shell
$ ocm login --token=${OCM_ACCESS_TOKEN} --url=https://maestro.${external_apps_domain} --insecure

$ ocm post /api/maestro/v1/consumers << EOF
{
  "name": "cluster1"
}
EOF

{
  "created_at":"2023-12-08T11:35:08.557450505Z",
  "href":"/api/maestro/v1/consumers/3f28c601-5028-47f4-9264-5cc43f2f27fb",
  "id":"3f28c601-5028-47f4-9264-5cc43f2f27fb",
  "kind":"Consumer",
  "name":"cluster1",
  "updated_at":"2023-12-08T11:35:08.557450505Z"
}

```

Deploy maestro agent:

You need to set `consumer_id` to the consumer id you received in the previous step.

```shell
$ export consumer_id=3f28c601-5028-47f4-9264-5cc43f2f27fb
$ make deploy-agent
$ oc get pod -n maestro-agent-root
NAME                             READY   STATUS    RESTARTS   AGE
maestro-agent-5dc9f5b4bf-8jcvq   1/1     Running   0          13s
```

Create a resource:
```shell
$ ocm post /api/maestro/v1/resources << EOF
{
  "consumer_id": "3f28c601-5028-47f4-9264-5cc43f2f27fb",
  "version": 1,
  "manifest": {
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
      "name": "nginx",
      "namespace": "default"
    },
    "spec": {
      "replicas": 1,
      "selector": {
        "matchLabels": {
          "app": "nginx"
        }
      },
      "template": {
        "metadata": {
          "labels": {
            "app": "nginx"
          }
        },
        "spec": {
          "containers": [
            {
              "image": "nginxinc/nginx-unprivileged",
              "name": "nginx"
            }
          ]
        }
      }
    }
  }
}
EOF
```
You should be able to see the pod is created in default namespace.
```shell
$ oc get pod -n default
NAME                     READY   STATUS    RESTARTS   AGE
nginx-5d6b548959-829c7   1/1     Running   0          70s
```

### Make a new Kind

1. Add to openapi.yaml
2. Generate the new structs/clients (`make generate`)