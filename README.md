Maestro
---

## Run for the first time

### Make a build and run postgres

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

```

### Run database migrations

The initial migration will create the base data model as well as providing a way to add future migrations.

```shell

# Run migrations
./maestro migrate

# Verify they ran in the database
$ make db/login

root@f076ddf94520:/# psql -h localhost -U maestro maestro
psql (14.4 (Debian 14.4-1.pgdg110+1))
Type "help" for help.

maestro=# \dt
                 List of relations
 Schema |    Name    | Type  |        Owner        
--------+------------+-------+---------------------
 public | dinosaurs  | table | maestro
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

curl http://localhost:8000/api/maestro/v1/dinosaurs | jq

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

#### Get a new Dinosaur
This will be empty if no Dinosaur is ever created

```
ocm get /api/maestro/v1/dinosaurs
{
  "items": [],
  "kind": "DinosaurList",
  "page": 1,
  "size": 0,
  "total": 0
}
```

#### Post a new Dinosaur

```shell

ocm post /api/maestro/v1/dinosaurs << EOF
{
    "species": "foo"
}
EOF

```

#### Get your Dinosaur

```shell
ocm get /api/maestro/v1/dinosaurs
{
  "items": [
    {
      "created_at":"2023-10-26T08:15:54.509653Z",
      "href":"/api/maestro/v1/dinosaurs/2XIENcJIi9t2eBblhWVCtWLdbDZ",
      "id":"2XIENcJIi9t2eBblhWVCtWLdbDZ",
      "kind":"Dinosaur",
      "species":"foo",
      "updated_at":"2023-10-26T08:15:54.509653Z"
    }
  ],
  "kind":"DinosaurList",
  "page":1,
  "size":1,
  "total":1
}
```

#### Run in CRC

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

Log into CRC and try a deployment:

```shell

$ make crc/login
Logging into CRC
Logged into "https://api.crc.testing:6443" as "kubeadmin" using existing credentials.

You have access to 66 projects, the list has been suppressed. You can list all projects with 'oc projects'

Using project "ocm-mturansk".
Login Succeeded!

$ make deploy

$ ocm login --token=${OCM_ACCESS_TOKEN} --url=https://maestro.apps-crc.testing --insecure

$ ocm post /api/maestro/v1/dinosaurs << EOF
{
    "species": "foo"
}
EOF
```



### Make a new Kind

1. Add to openapi.yaml
2. Generate the new structs/clients (`make generate`)