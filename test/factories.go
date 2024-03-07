package test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/db"
)

var testObjectJSON = `
{
	"apiVersion": "v1",
	"kind": "ConfigMap",
	"metadata": {
		"name": "test",
		"namespace": "test"
	},
	"data": {
		"test_key": "%s"
	}
}
`

var testManifestJSON = `
{
    "id": "75479c10-b537-4261-8058-ca2e36bac384",
    "time": "2024-03-07T03:29:03.194843266Z",
    "type": "io.open-cluster-management.works.v1alpha1.manifests.spec.create_request",
    "source": "maestro",
    "specversion": "1.0",
    "datacontenttype": "application/json",
    "data": {
        "manifest": {
			"apiVersion": "v1",
            "kind": "ConfigMap",
            "metadata": {
                "name": "test",
                "namespace": "test"
            },
			"data": {
				"test_key": "%s"
			}
        },
        "configOption": {
            "feedbackRules": [
                {
                    "type": "JSONPaths",
                    "jsonPaths": [
                        {
                            "name": "status",
                            "path": ".status"
                        }
                    ]
                }
            ],
            "updateStrategy": {
                "type": "ServerSideApply"
            }
        },
        "deleteOption": {
            "propagationPolicy": "Foreground"
        }
    }
}
`

func (helper *Helper) NewAPIResource(consumerID string, value string) openapi.Resource {
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testObjectJSON, value)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling test object: %q", err)
	}

	return openapi.Resource{
		Manifest:   testManifest,
		ConsumerId: &consumerID,
	}
}

func (helper *Helper) NewResource(consumerID string, value string) *api.Resource {
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testManifestJSON, value)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling test manifest: %q", err)
	}

	resource := &api.Resource{
		ConsumerID: consumerID,
		Type:       api.ResourceTypeSingle,
		Manifest:   testManifest,
	}

	return resource
}

func (helper *Helper) CreateResource(consumerID string, value string) *api.Resource {
	resourceService := helper.Env().Services.Resources()
	resource := helper.NewResource(consumerID, value)

	res, err := resourceService.Create(context.Background(), resource)
	if err != nil {
		helper.T.Errorf("error creating resource: %q", err)
	}

	return res
}

func (helper *Helper) CreateResourceList(consumerID string, count int) (resources []*api.Resource) {
	for i := 1; i <= count; i++ {
		resources = append(resources, helper.CreateResource(consumerID, "test_value"))
	}
	return resources
}

func (helper *Helper) CreateConsumer(name string) *api.Consumer {
	return helper.CreateConsumerWithLabels(name, nil)
}

func (helper *Helper) CreateConsumerWithLabels(name string, labels map[string]string) *api.Consumer {
	consumerService := helper.Env().Services.Consumers()

	consumer, err := consumerService.Create(context.Background(), &api.Consumer{Name: &name, Labels: db.EmptyMapToNilStringMap(&labels)})
	if err != nil {
		helper.T.Errorf("error creating resource: %q", err)
	}
	return consumer
}

func (helper *Helper) CreateConsumerList(count int) (consumers []*api.Consumer) {
	for i := 1; i <= count; i++ {
		consumers = append(consumers, helper.CreateConsumer(fmt.Sprintf("consumer-%d", i)))
	}
	return consumers
}
