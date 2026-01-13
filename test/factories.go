package test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	pubsubv2 "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
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
			  "image": "quay.io/nginx/nginx-unprivileged:latest",
			  "imagePullPolicy": "IfNotPresent",
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
func (helper *Helper) EncodeManifestBundle(resID, manifestJSON, deployName, deployNamespace string) (datatypes.JSONMap, error) {
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
	evt := cetypes.NewEventBuilder(source, cetypes.CloudEventsType{}).WithResourceID(resID).NewEvent()
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
func (helper *Helper) NewResource(resID, consumerName, deployName, serviceAccount string, replicas int, resourceVersion int32) (*api.Resource, error) {
	namespace := "default" // default namespace
	manifestJSON := helper.NewManifestJSON(deployName, serviceAccount, replicas)
	payload, err := helper.EncodeManifestBundle(resID, manifestJSON, deployName, namespace)
	if err != nil {
		return nil, err
	}

	resource := &api.Resource{
		Meta: api.Meta{
			ID: resID,
		},
		ConsumerName: consumerName,
		Payload:      payload,
		Version:      resourceVersion,
	}

	return resource, nil
}

// CreateResource creates a resource with the given consumer name, deploy name and replicas.
// It generates a deployment for nginx using the manifestJSON template, assigning a random deploy name to avoid conflicts.
func (helper *Helper) CreateResource(resID, consumerName, deployName, serviceAccount string, replicas int) (*api.Resource, error) {
	resource, err := helper.NewResource(resID, consumerName, deployName, serviceAccount, replicas, 1)
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
		resource, err := helper.CreateResource(uuid.NewString(), consumerName, deployName, "default", 1)
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

	// If using PubSub message broker, create the corresponding subscriptions and config file
	if helper.Broker == "pubsub" {
		configPath, err := helper.createPubSubSubscriptions(name)
		if err != nil {
			// Log the error but don't fail consumer creation
			// The subscriptions can be created manually if needed
			helper.T.Logf("Warning: failed to create PubSub subscriptions for consumer %s: %v\n", name, err)
		} else {
			// Store the config file path in the map
			helper.consumerPubSubConfigMap[name] = configPath
			helper.T.Logf("Created PubSub agent config for consumer %s at: %s\n", name, configPath)
		}
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

func (helper *Helper) AddGRPCAuthRule(ctx context.Context, kubeClient kubernetes.Interface, ruleName, resourceType, resourceID string) error {
	nonResourceUrl := ""
	switch resourceType {
	case "source":
		nonResourceUrl = fmt.Sprintf("/sources/%s", resourceID)
	case "cluster":
		nonResourceUrl = fmt.Sprintf("/clusters/%s", resourceID)
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	clusterRole, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, ruleName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, newErr := kubeClient.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ruleName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					NonResourceURLs: []string{nonResourceUrl},
					Verbs:           []string{"pub", "sub"},
				},
			},
		}, metav1.CreateOptions{})

		return newErr
	}

	if err != nil {
		return err
	}

	if len(clusterRole.Rules) != 1 {
		return fmt.Errorf("unexpected rules in: %s", ruleName)
	}

	policyRule := clusterRole.Rules[0]
	if slices.Contains(policyRule.NonResourceURLs, nonResourceUrl) {
		// no change, do nothing
		return nil
	}

	newClusterRole := clusterRole.DeepCopy()
	newClusterRole.Rules = []rbacv1.PolicyRule{
		{
			NonResourceURLs: append(policyRule.NonResourceURLs, nonResourceUrl),
			Verbs:           []string{"pub", "sub"},
		},
	}

	// update the cluster role
	_, err = kubeClient.RbacV1().ClusterRoles().Update(ctx, newClusterRole, metav1.UpdateOptions{})
	return err
}

func (helper *Helper) CreateGRPCConn(serverAddr, serverCAFile, token string) (*grpc.ClientConn, error) {
	if serverCAFile == "" || token == "" {
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

		perRPCCred := oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: string(token),
			})}

		return grpc.Dial(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithPerRPCCredentials(perRPCCred))
	}
}

// pubSubConfig represents the structure of the PubSub configuration file
type pubSubConfig struct {
	ProjectID string `json:"projectID"`
	Endpoint  string `json:"endpoint"`
	Insecure  bool   `json:"insecure"`
}

// parsePubSubConfig reads and parses the PubSub configuration file
func parsePubSubConfig(configFile string) (*pubSubConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read pubsub config file: %w", err)
	}

	var config pubSubConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse pubsub config: %w", err)
	}

	return &config, nil
}

// createPubSubSubscriptions creates PubSub subscriptions for a consumer using v2 API
// and returns the path to the generated agent config file
func (helper *Helper) createPubSubSubscriptions(consumerName string) (string, error) {
	// Parse the config file
	config, err := parsePubSubConfig(helper.Env().Config.MessageBroker.MessageBrokerConfig)
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	// Create gRPC connection
	var opts []option.ClientOption
	opts = append(opts, option.WithEndpoint(config.Endpoint))
	if config.Insecure {
		opts = append(opts, option.WithoutAuthentication(), option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	}

	// Create PubSub client using v2 API
	client, err := pubsubv2.NewClient(ctx, config.ProjectID, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to create pubsub client: %w", err)
	}
	defer client.Close()

	// Define topic paths
	sourceEventsTopic := fmt.Sprintf("projects/%s/topics/sourceevents", config.ProjectID)
	sourceBroadcastTopic := fmt.Sprintf("projects/%s/topics/sourcebroadcast", config.ProjectID)

	// Create subscriptions for the consumer
	subscriptions := []struct {
		name   string
		topic  string
		filter string
	}{
		{
			name:   fmt.Sprintf("projects/%s/subscriptions/sourceevents-%s", config.ProjectID, consumerName),
			topic:  sourceEventsTopic,
			filter: fmt.Sprintf(`attributes."ce-clustername"="%s"`, consumerName),
		},
		{
			name:   fmt.Sprintf("projects/%s/subscriptions/sourcebroadcast-%s", config.ProjectID, consumerName),
			topic:  sourceBroadcastTopic,
			filter: "",
		},
	}

	for _, sub := range subscriptions {
		// Check if subscription already exists
		_, err := client.SubscriptionAdminClient.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{
			Subscription: sub.name,
		})
		if err == nil {
			// Subscription already exists, skip
			continue
		}

		// Create the subscription
		subConfig := &pubsubpb.Subscription{
			Name:  sub.name,
			Topic: sub.topic,
		}
		if sub.filter != "" {
			subConfig.Filter = sub.filter
		}

		_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, subConfig)
		if err != nil {
			// Ignore already exists errors
			if !strings.Contains(err.Error(), "already exists") {
				return "", fmt.Errorf("failed to create subscription %s: %w", sub.name, err)
			}
		}
	}

	// Create agent config file
	agentConfig := map[string]interface{}{
		"projectID": config.ProjectID,
		"endpoint":  config.Endpoint,
		"insecure":  config.Insecure,
		"topics": map[string]string{
			"agentEvents":    fmt.Sprintf("projects/%s/topics/agentevents", config.ProjectID),
			"agentBroadcast": fmt.Sprintf("projects/%s/topics/agentbroadcast", config.ProjectID),
		},
		"subscriptions": map[string]string{
			"sourceEvents":    fmt.Sprintf("projects/%s/subscriptions/sourceevents-%s", config.ProjectID, consumerName),
			"sourceBroadcast": fmt.Sprintf("projects/%s/subscriptions/sourcebroadcast-%s", config.ProjectID, consumerName),
		},
	}

	agentConfigJSON, err := json.Marshal(agentConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent config: %w", err)
	}

	// Create temporary file for agent config
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("pubsub-agent-%s-*.json", consumerName))
	if err != nil {
		return "", fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(agentConfigJSON); err != nil {
		return "", fmt.Errorf("failed to write agent config: %w", err)
	}

	return tmpFile.Name(), nil
}
