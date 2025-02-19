package test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"gorm.io/datatypes"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"

	workv1 "open-cluster-management.io/api/work/v1"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

var manifestJSON = `
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

// NewManifestJSON creates a resource manifest in JSON format with the given deploy name and replicas.
// It generates a deployment for nginx using the manifestJSON template, assigning a random deploy name to avoid conflicts.
func (helper *Helper) NewManifestJSON(deployName, serviceAccount string, replicas int) string {
	namespace := "default" // default namespace
	return fmt.Sprintf(manifestJSON, deployName, namespace, replicas, serviceAccount)
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
	// create a cloud event with the manifest bundle as the data
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
		return nil, fmt.Errorf("failed to convert cloudevent to resource manifest bundle: %v", err)
	}

	return manifestBundle, nil
}

// NewResource creates a resource with the given consumer name, deploy name, replicas, and resource version.
func (helper *Helper) NewResource(consumerName, deployName, serviceAccount string, replicas int, resourceVersion int32) (*api.Resource, error) {
	namespace := "default" // default namespace
	manifestJSON := helper.NewManifestJSON(deployName, serviceAccount, replicas)
	payload, err := helper.EncodeManifestBundle(manifestJSON, deployName, namespace)
	if err != nil {
		return nil, err
	}

	resource := &api.Resource{
		ConsumerName: consumerName,
		Payload:      payload,
		Version:      resourceVersion,
	}

	return resource, nil
}

// CreateResource creates a resource with the given consumer name, deploy name and replicas.
// It generates a deployment for nginx using the manifestJSON template, assigning a random deploy name to avoid conflicts.
func (helper *Helper) CreateResource(consumerName, deployName, serviceAccount string, replicas int) (*api.Resource, error) {
	resource, err := helper.NewResource(consumerName, deployName, serviceAccount, replicas, 1)
	if err != nil {
		return nil, err
	}
	resourceService := helper.Env().Services.Resources()
	res, svcErr := resourceService.Create(context.Background(), resource)
	if svcErr != nil {
		return nil, svcErr.AsError()
	}

	return res, nil
}

// UpdateResource attempts to update a resource, resource ID must not be empty.
func (helper *Helper) UpdateResource(resource *api.Resource) (*api.Resource, error) {
	resourceService := helper.Env().Services.Resources()
	res, err := resourceService.Update(context.Background(), resource)
	if err != nil {
		return nil, err.AsError()
	}

	return res, nil
}

// CreateResourceList generates a list of resources with the specified consumer name and count.
// Each resource gets a randomly generated deploy name for nginx deployments to avoid conflicts.
func (helper *Helper) CreateResourceList(consumerName string, count int) ([]*api.Resource, error) {
	resources := make([]*api.Resource, count)
	for i := 0; i < count; i++ {
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		resource, err := helper.CreateResource(consumerName, deployName, "default", 1)
		if err != nil {
			return resources, err
		}
		resources[i] = resource
	}

	return resources, nil
}

// DeleteResource attempts to delete a resource and returns an error if it fails.
func (helper *Helper) DeleteResource(id string) error {
	resourceService := helper.Env().Services.Resources()
	if err := resourceService.MarkAsDeleting(context.Background(), id); err != nil {
		return err.AsError()
	}

	return nil
}

// NewManifest creats a manifest with the given deploy name and replicas.
// It generates a deployment for nginx using the manifestJSON template, assigning random
// deploy name to avoid conflicts.
func (helper *Helper) NewManifest(deployName, serviceAccount string, replicas int) workv1.Manifest {
	manifestJSON := helper.NewManifestJSON(deployName, serviceAccount, replicas)
	return workv1.Manifest{
		RawExtension: runtime.RawExtension{
			Raw: []byte(manifestJSON),
		},
	}
}

// NewManifestWork creates a manifestwork with the given manifestwork name, deploy name and replicas.
// It generates a deployment for nginx using the manifestJSON template, assigning random manifestwork name
// and deploy name to avoid conflicts.
func (helper *Helper) NewManifestWork(workName, deployName, serviceAccount string, replicas int) *workv1.ManifestWork {
	manifest := helper.NewManifest(deployName, serviceAccount, replicas)
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: workName,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{manifest},
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     "apps",
						Resource:  "deployments",
						Name:      deployName,
						Namespace: "default",
					},
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
			},
		},
	}

}

func (helper *Helper) CreateConsumer(name string) (*api.Consumer, error) {
	return helper.CreateConsumerWithLabels(name, nil)
}

func (helper *Helper) CreateConsumerWithLabels(name string, labels map[string]string) (*api.Consumer, error) {
	consumerService := helper.Env().Services.Consumers()

	consumer, err := consumerService.Create(context.Background(), &api.Consumer{Name: name, Labels: db.EmptyMapToNilStringMap(&labels)})
	if err != nil {
		return nil, err
	}

	return consumer, nil
}

func (helper *Helper) CreateConsumerList(count int) ([]*api.Consumer, error) {
	consumers := make([]*api.Consumer, count)
	for i := 0; i < count; i++ {
		consumer, err := helper.CreateConsumer(fmt.Sprintf("consumer-%d", i))
		if err != nil {
			return consumers, err
		}
		consumers[i] = consumer
	}

	return consumers, nil
}

// NewEvent creates a CloudEvent with the given source, action, consumer name, resource ID, deployment name, resource version, and replicas.
// It generates a nginx deployment using the manifestJSON template, assigning a random deploy name to avoid conflicts.
// If the action is "delete_request," the event includes a deletion timestamp.
func (helper *Helper) NewEvent(source, action, consumerName, resourceID, deployName string, resourceVersion int64, replicas int) (*cloudevents.Event, error) {
	sa := "default"              // default service account
	deployNamespace := "default" // default namespace
	manifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(fmt.Sprintf(manifestJSON, deployName, deployNamespace, replicas, sa)), &manifest); err != nil {
		return nil, err
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
		return &evt, nil
	}

	eventPayload := &workpayload.ManifestBundle{
		Manifests: []workv1.Manifest{
			{
				RawExtension: runtime.RawExtension{
					Object: &unstructured.Unstructured{Object: manifest},
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
		return nil, err
	}

	return &evt, nil
}

func (helper *Helper) CreateGRPCAuthRule(ctx context.Context, kubeClient kubernetes.Interface, ruleName, resourceType, resourceID string, actions []string) error {
	// create the cluster rolefor grpc authz
	nonResourceUrl := ""
	switch resourceType {
	case "source":
		nonResourceUrl = fmt.Sprintf("/sources/%s", resourceID)
	case "cluster":
		nonResourceUrl = fmt.Sprintf("/clusters/%s", resourceID)
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	_, err := kubeClient.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: ruleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{nonResourceUrl},
				Verbs:           actions,
			},
		},
	}, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		// update the cluster role
		_, err = kubeClient.RbacV1().ClusterRoles().Update(ctx, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ruleName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					NonResourceURLs: []string{nonResourceUrl},
					Verbs:           actions,
				},
			},
		}, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return err
}

func (helper *Helper) CreateGRPCConn(serverAddr, serverCAFile, tokenFile string) (*grpc.ClientConn, error) {
	if serverCAFile == "" || tokenFile == "" {
		// no TLS and authz
		return grpc.Dial(serverAddr, grpc.WithInsecure())
	} else {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		caPEM, err := os.ReadFile(serverCAFile)
		if err != nil {
			return nil, err
		}

		ok := certPool.AppendCertsFromPEM(caPEM)
		if !ok {
			return nil, fmt.Errorf("failed to append server CA certificate")
		}

		tlsConfig := &tls.Config{
			RootCAs:    certPool,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}

		token, err := os.ReadFile(tokenFile)
		if err != nil {
			return nil, err
		}

		perRPCCred := oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: string(token),
			})}

		return grpc.Dial(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithPerRPCCredentials(perRPCCred))
	}
}
