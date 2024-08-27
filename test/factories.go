package test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/db"
	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	workv1 "open-cluster-management.io/api/work/v1"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

var testManifestJSON = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
	  "name": "%s",
	  "namespace": "%s"
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
		  "serviceAccount": "%s",
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

var testReadOnlyManifestJSON = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
	  "name": "%s",
	  "namespace": "%s"
	},
	"update_strategy": {
	  "type": "ReadOnly"
	}
}
`

// NewAPIResource creates an API resource with the given consumer name, deploy name, and replicas.
// It generates a deployment for nginx using the testManifestJSON template, giving it a random deploy
// name to avoid testing conflicts.
func (helper *Helper) NewAPIResource(consumerName, deployName string, replicas int) openapi.Resource {
	sa := "default" // default service account
	return helper.NewAPIResourceWithSA(consumerName, deployName, sa, replicas)
}

// NewAPIResourceWithSA creates an API resource with the given consumer name, deploy name, service account, and replicas.
// It generates a nginx deployment using the testManifestJSON template, assigning a random deploy name to avoid testing conflicts.
func (helper *Helper) NewAPIResourceWithSA(consumerName, deployName, sa string, replicas int) openapi.Resource {
	namespace := "default" // default namespace
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testManifestJSON, deployName, namespace, replicas, sa)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling manifest: %q", err)
	}

	return openapi.Resource{
		Manifest:     testManifest,
		ConsumerName: &consumerName,
	}
}

// NewResourceManifestJSON creates a resource manifest in JSON format with the given deploy name and replicas.
// It generates a deployment for nginx using the testManifestJSON template, assigning a random deploy name to avoid
// testing conflicts.
func (helper *Helper) NewResourceManifestJSON(deployName string, replicas int) string {
	namespace := "default" // default namespace
	sa := "default"        // default service account
	return fmt.Sprintf(testManifestJSON, deployName, namespace, replicas, sa)
}

// NewReadOnlyAPIResource creates an API resource with the given consumer name and deploy name.
// It generates a read-only deployment manifests for nginx using the testReadOnlyManifestJSON template,
// giving it a random deploy name to avoid testing conflicts.
func (helper *Helper) NewReadOnlyAPIResource(consumerName, deployName string) openapi.Resource {
	namespace := "default" // default namespace
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testReadOnlyManifestJSON, deployName, namespace)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling test manifest: %q", err)
	}

	return openapi.Resource{
		Manifest:     testManifest,
		ConsumerName: &consumerName,
	}
}

// NewReadOnlyResourceManifestJSON creates a resource with the given consumer name, deploy name, replicas, and resource version.
// It generates a deployment for nginx using the testManifestJSON template, assigning a random deploy name to avoid testing conflicts.
func (helper *Helper) NewResource(consumerName, deployName string, replicas int, resourceVersion int32) *api.Resource {
	testResource := helper.NewAPIResource(consumerName, deployName, replicas)
	testPayload, err := api.EncodeManifest(testResource.Manifest, testResource.DeleteOption, testResource.UpdateStrategy)
	if err != nil {
		helper.T.Errorf("error encoding manifest: %q", err)
	}

	resource := &api.Resource{
		ConsumerName: consumerName,
		Type:         api.ResourceTypeSingle,
		Payload:      testPayload,
		Version:      resourceVersion,
	}

	return resource
}

// CreateResource creates a resource with the given consumer name, deploy name, and replicas.
// It generates a deployment for nginx using the testManifestJSON template, assigning a random deploy name to avoid testing conflicts.
func (helper *Helper) CreateResource(consumerName, deployName string, replicas int) *api.Resource {
	resource := helper.NewResource(consumerName, deployName, replicas, 1)
	resourceService := helper.Env().Services.Resources()

	res, err := resourceService.Create(context.Background(), resource)
	if err != nil {
		helper.T.Errorf("error creating resource: %q", err)
	}

	return res
}

// CreateResourceList generates a list of resources with the specified consumer name and count.
// Each resource gets a randomly generated deploy name for nginx deployments to avoid testing conflicts.
func (helper *Helper) CreateResourceList(consumerName string, count int) (resources []*api.Resource) {
	for i := 1; i <= count; i++ {
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		resources = append(resources, helper.CreateResource(consumerName, deployName, 1))
		time.Sleep(10 * time.Millisecond)
	}

	return resources
}

// EncodeManifestBundle converts resource manifest JSON into a CloudEvent JSONMap representation.
func (helper *Helper) EncodeManifestBundle(manifestJSON, deployName, deployNamespace string) (datatypes.JSONMap, error) {
	if len(manifestJSON) == 0 {
		return nil, nil
	}

	// unmarshal manifest JSON
	manifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		return nil, fmt.Errorf("error unmarshalling manifest: %v", err)
	}

	// default deletion option
	delOption := &workv1.DeleteOption{
		PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
	}

	// default update strategy
	upStrategy := &workv1.UpdateStrategy{
		Type: workv1.UpdateStrategyTypeServerSideApply,
	}

	source := "maestro"
	// create a cloud event with the manifest as the data
	evt := cetypes.NewEventBuilder(source, cetypes.CloudEventsType{}).NewEvent()
	eventPayload := &workpayload.ManifestBundle{
		Manifests: []workv1.Manifest{
			{
				RawExtension: runtime.RawExtension{
					Object: &unstructured.Unstructured{Object: manifest},
				},
			},
		},
		DeleteOption: delOption,
		ManifestConfigs: []workv1.ManifestConfigOption{
			{
				FeedbackRules: []workv1.FeedbackRule{
					{
						Type: workv1.JSONPathsType,
						JsonPaths: []workv1.JsonPath{
							{
								Name: "status",
								Path: ".status",
							},
						},
					},
				},
				UpdateStrategy: upStrategy,
				ResourceIdentifier: workv1.ResourceIdentifier{
					Group:     "apps",
					Resource:  "deployments",
					Name:      deployName,
					Namespace: deployNamespace,
				},
			},
		},
	}

	// set the event data
	if err := evt.SetData(cloudevents.ApplicationJSON, eventPayload); err != nil {
		return nil, fmt.Errorf("failed to set cloud event data: %v", err)
	}

	// convert cloudevent to JSONMap
	manifestBundle, err := api.CloudEventToJSONMap(&evt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cloudevent to resource manifest: %v", err)
	}

	return manifestBundle, nil
}

// NewResourceBundle creates a resource bundle with the given consumer name, deploy name, replicas, and resource version.
func (helper *Helper) NewResourceBundle(consumerName, deployName string, replicas int, resourceVersion int32) *api.Resource {
	namespace := "default" // default namespace
	manifestJSON := helper.NewResourceManifestJSON(deployName, replicas)
	payload, err := helper.EncodeManifestBundle(manifestJSON, deployName, namespace)
	if err != nil {
		helper.T.Errorf("error encoding manifest bundle: %q", err)
	}

	resource := &api.Resource{
		ConsumerName: consumerName,
		Type:         api.ResourceTypeBundle,
		Payload:      payload,
		Version:      resourceVersion,
	}

	return resource
}

// CreateResourceBundle creates a resource bundle with the given consumer name, deploy name and replicas.
// It generates a deployment for nginx using the testManifestJSON template, assigning a random deploy name to avoid testing conflicts.
func (helper *Helper) CreateResourceBundle(consumerName, deployName string, replicas int) *api.Resource {
	resourceBundle := helper.NewResourceBundle(consumerName, deployName, replicas, 1)
	resourceService := helper.Env().Services.Resources()

	res, err := resourceService.Create(context.Background(), resourceBundle)
	if err != nil {
		helper.T.Errorf("error creating resource bundle: %q", err)
	}

	return res
}

// CreateResourceBundleList generates a list of resource bundles with the specified consumer name and count.
// Each resource gets a randomly generated deploy name for nginx deployments to avoid testing conflicts.
func (helper *Helper) CreateResourceBundleList(consumerName string, count int) (resourceBundles []*api.Resource) {
	for i := 1; i <= count; i++ {
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		resourceBundles = append(resourceBundles, helper.CreateResourceBundle(consumerName, deployName, 1))
	}

	return resourceBundles
}

func (helper *Helper) CreateConsumer(name string) *api.Consumer {
	return helper.CreateConsumerWithLabels(name, nil)
}

func (helper *Helper) CreateConsumerWithLabels(name string, labels map[string]string) *api.Consumer {
	consumerService := helper.Env().Services.Consumers()

	consumer, err := consumerService.Create(context.Background(), &api.Consumer{Name: name, Labels: db.EmptyMapToNilStringMap(&labels)})
	if err != nil {
		helper.T.Errorf("error creating consumer: %q", err)
	}
	return consumer
}

func (helper *Helper) CreateConsumerList(count int) (consumers []*api.Consumer) {
	for i := 1; i <= count; i++ {
		consumers = append(consumers, helper.CreateConsumer(fmt.Sprintf("consumer-%d", i)))
	}

	return consumers
}

// NewEvent creates a CloudEvent with the given source, action, consumer name, resource ID, deploy name, resource version, and replicas.
// It generates a nginx deployment using the testManifestJSON template, assigning a random deploy name to avoid testing conflicts.
// If the action is "delete_request," the event includes a deletion timestamp.
func (helper *Helper) NewEvent(source, action, consumerName, resourceID, deployName string, resourceVersion int64, replicas int) *cloudevents.Event {
	sa := "default"              // default service account
	deployNamespace := "default" // default namespace
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testManifestJSON, deployName, deployNamespace, replicas, sa)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling manifest: %q", err)
	}

	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.EventAction(action),
	}
	evtBuilder := cetypes.NewEventBuilder(source, eventType).
		WithClusterName(consumerName).
		WithResourceID(resourceID).
		WithResourceVersion(resourceVersion)

	// add deletion timestamp if action is delete_request
	if action == "delete_request" {
		evtBuilder.WithDeletionTimestamp(time.Now())
	}

	evt := evtBuilder.NewEvent()

	// if action is delete_request, no data is needed
	if action == "delete_request" {
		evt.SetData(cloudevents.ApplicationJSON, nil)
		return &evt
	}

	eventPayload := &workpayload.Manifest{
		Manifest: unstructured.Unstructured{Object: testManifest},
		DeleteOption: &workv1.DeleteOption{
			PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
		},
		ConfigOption: &workpayload.ManifestConfigOption{
			FeedbackRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: "status",
							Path: ".status",
						},
					},
				},
			},
			UpdateStrategy: &workv1.UpdateStrategy{
				Type: workv1.UpdateStrategyTypeServerSideApply,
			},
		},
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, eventPayload); err != nil {
		helper.T.Errorf("failed to set cloud event data: %q", err)
	}

	return &evt
}

// NewBundleEvent creates a CloudEvent with the given source, action, consumer name, resource ID, resource version, and replicas.
// It generates a bundle of nginx deployments using the testManifestJSON template, assigning a random deploy name to avoid testing conflicts.
// If the action is "delete_request," the event includes a deletion timestamp.
func (helper *Helper) NewBundleEvent(source, action, consumerName, resourceID, deployName string, resourceVersion int64, replicas int) *cloudevents.Event {
	sa := "default"              // default service account
	deployNamespace := "default" // default namespace
	testManifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(testManifestJSON, deployName, deployNamespace, replicas, sa)), &testManifest); err != nil {
		helper.T.Errorf("error unmarshalling manifest: %q", err)
	}

	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              cetypes.EventAction(action),
	}

	// create a cloud event with the manifest as the data
	evtBuilder := cetypes.NewEventBuilder(source, eventType).
		WithClusterName(consumerName).
		WithResourceID(resourceID).
		WithResourceVersion(resourceVersion)

	// add deletion timestamp if action is delete_request
	if action == "delete_request" {
		evtBuilder.WithDeletionTimestamp(time.Now())
	}

	evt := evtBuilder.NewEvent()

	// if action is delete_request, no data is needed
	if action == "delete_request" {
		evt.SetData(cloudevents.ApplicationJSON, nil)
		return &evt
	}

	eventPayload := &workpayload.ManifestBundle{
		Manifests: []workv1.Manifest{
			{
				RawExtension: runtime.RawExtension{
					Object: &unstructured.Unstructured{Object: testManifest},
				},
			},
		},
		DeleteOption: &workv1.DeleteOption{
			PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
		},
		ManifestConfigs: []workv1.ManifestConfigOption{
			{
				FeedbackRules: []workv1.FeedbackRule{
					{
						Type: workv1.JSONPathsType,
						JsonPaths: []workv1.JsonPath{
							{
								Name: "status",
								Path: ".status",
							},
						},
					},
				},
				UpdateStrategy: &workv1.UpdateStrategy{
					Type: workv1.UpdateStrategyTypeServerSideApply,
				},
				ResourceIdentifier: workv1.ResourceIdentifier{
					Group:     "apps",
					Resource:  "deployments",
					Name:      deployName,
					Namespace: deployNamespace,
				},
			},
		},
	}

	if err := evt.SetData(cloudevents.ApplicationJSON, eventPayload); err != nil {
		helper.T.Errorf("failed to set cloud event data: %q", err)
	}

	return &evt
}
