package test

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
)

func (helper *Helper) NewResource(consumerID string) *api.Resource {
	resourceService := helper.Env().Services.Resources()

	resource := &api.Resource{
		ConsumerID: consumerID,
		Manifest:   map[string]interface{}{"data": 0},
	}

	res, err := resourceService.Create(context.Background(), resource)
	if err != nil {
		helper.T.Errorf("error creating resource: %q", err)
	}

	return res
}

func (helper *Helper) NewResourceList(consumerID string, count int) (resource []*api.Resource) {
	for i := 1; i <= count; i++ {
		resource = append(resource, helper.NewResource(consumerID))
	}
	return resource
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
