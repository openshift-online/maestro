package test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

var testManifestJSON = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
	  "name": "nginx",
	  "namespace": "default"
	},
	"spec": {
	  "replicas": %d,
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
`

func (helper *Helper) NewAPIResource(consumerID string, replicas int) openapi.Resource {
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testManifestJSON, replicas)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling test manifest: %q", err)
	}

	return openapi.Resource{
		Manifest:   testManifest,
		ConsumerId: &consumerID,
	}
}

func (helper *Helper) CreateResource(consumerID string, replicas int) *api.Resource {
	resourceService := helper.Env().Services.Resources()

	resource := helper.NewResource(consumerID, replicas)

	res, err := resourceService.Create(context.Background(), resource)
	if err != nil {
		helper.T.Errorf("error creating resource: %q", err)
	}

	return res
}

func (helper *Helper) NewResource(consumerID string, replicas int) *api.Resource {

	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testManifestJSON, replicas)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling test manifest: %q", err)
	}

	resource := &api.Resource{
		ConsumerID: consumerID,
		Manifest:   testManifest,
	}

	return resource
}

func (helper *Helper) NewResourceList(consumerID string, count int) (resources []*api.Resource) {
	for i := 1; i <= count; i++ {
		resources = append(resources, helper.CreateResource(consumerID, 1))
	}
	return resources
}

func (helper *Helper) NewConsumer(name string) *api.Consumer {
	consumerService := helper.Env().Services.Consumers()

	consumer, err := consumerService.Create(context.Background(), &api.Consumer{Name: name})
	if err != nil {
		helper.T.Errorf("error creating resource: %q", err)
	}
	return consumer
}

func (helper *Helper) NewConsumerList(count int) (consumers []*api.Consumer) {
	for i := 1; i <= count; i++ {
		consumers = append(consumers, helper.NewConsumer(fmt.Sprintf("consumer-%d", i)))
	}
	return consumers
}
