package utils

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	deployName      = "maestro-test"
	deployNamespace = "maestro-test"
)

var DeploymentGVK = schema.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "Deployment",
}

var DeploymentGVR = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "deployments",
}

var ManifestWorkGVK = schema.GroupVersionKind{
	Group:   "work.open-cluster-management.io",
	Version: "v1",
	Kind:    "ManifestWork",
}

var ManifestWorkGVR = schema.GroupVersionResource{
	Group:    "work.open-cluster-management.io",
	Version:  "v1",
	Resource: "manifestworks",
}

func NewManifestWork(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, runtimeObj runtime.Object) (*workv1.ManifestWork, error) {
	obj, ok := runtimeObj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("unsupported object type %T", runtimeObj)
	}

	name := WorkName(gvk, obj)

	labels := obj.GetLabels() // Copy first
	if labels == nil {
		labels = map[string]string{}
	}
	labels["maestro.resource.type"] = gvkToUuid(gvk) // Set last

	updateStrategy := &workv1.UpdateStrategy{
		Type: workv1.UpdateStrategyTypeServerSideApply,
		ServerSideApply: &workv1.ServerSideApplyConfig{
			Force:        true,
			FieldManager: "maestro-agent",
		},
	}
	statusFeedbackJsonPath := workv1.JsonPath{
		Name: "status",
		Path: ".status",
	}
	if _, ok := obj.GetAnnotations()["maestro.readonly"]; ok {
		updateStrategy.Type = workv1.UpdateStrategyTypeReadOnly
		statusFeedbackJsonPath = workv1.JsonPath{
			Name: "resource",
			Path: "@",
		}
	}

	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: "0",
			Annotations:     obj.GetAnnotations(),
			Labels:          labels,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Object: runtimeObj,
						},
					},
				},
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     gvk.Group,
						Resource:  gvr.Resource,
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
					UpdateStrategy: updateStrategy,
					FeedbackRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								statusFeedbackJsonPath,
							},
						},
					},
				},
			},
		},
	}, nil
}

func WorkName(gvk schema.GroupVersionKind, obj metav1.Object) string {
	nameWithGvk := obj.GetName() + "-" + obj.GetNamespace() + "-" + gvk.String()
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(nameWithGvk)).String()
}

func NewNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

func NewDeployment(namespace, name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "nginx",
							Image:           "quay.io/nginx/nginx-unprivileged:latest",
							ImagePullPolicy: "IfNotPresent",
						},
					},
				},
			},
		},
	}
}

func NewDeploymentReadonly(namespace, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"maestro.readonly": "true",
			},
		},
	}
}

func NewDeploymentManifestWork(namespace, name string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "work.open-cluster-management.io/v1",
			Kind:       "ManifestWork",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"updatedat": fmt.Sprintf("%d", time.Now().Unix()),
			},
		},
		Spec: workv1.ManifestWorkSpec{
			DeleteOption: &workv1.DeleteOption{
				PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:    "",
						Resource: "namespaces",
						Name:     deployNamespace,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: workv1.UpdateStrategyTypeServerSideApply,
					},
					FeedbackRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: "phase",
									Path: ".status.phase",
								},
							},
						},
					},
				},
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     "apps",
						Resource:  "deployments",
						Name:      deployName,
						Namespace: deployNamespace,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: workv1.UpdateStrategyTypeServerSideApply,
					},
					FeedbackRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: "Available-Reason",
									Path: `.status.conditions[?(@.type=="Available")].reason`,
								},
								{
									Name: "Available-Status",
									Path: `.status.conditions[?(@.type=="Available")].status`,
								},
								{
									Name: "Available-Message",
									Path: `.status.conditions[?(@.type=="Available")].message`,
								},
								{
									Name: "Available-LastTransitionTime",
									Path: `.status.conditions[?(@.type=="Available")].lastTransitionTime`,
								},
								{
									Name: "Progressing-Reason",
									Path: `.status.conditions[?(@.type=="Progressing")].reason`,
								},
								{
									Name: "Progressing-Status",
									Path: `.status.conditions[?(@.type=="Progressing")].status`,
								},
								{
									Name: "Progressing-Message",
									Path: `.status.conditions[?(@.type=="Progressing")].message`,
								},
								{
									Name: "Progressing-LastTransitionTime",
									Path: `.status.conditions[?(@.type=="Progressing")].lastTransitionTime`,
								},
							},
						},
					},
				},
			},
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Object: NewNamespace(deployNamespace),
						},
					},
					{
						RawExtension: runtime.RawExtension{
							Object: NewDeployment(deployNamespace, deployName, 0),
						},
					},
				},
			},
		},
	}
}

func UpdateReplicas(lastReplicas int32) int32 {
	if lastReplicas >= 2 {
		return 1
	}

	return lastReplicas + 1
}

func gvkToUuid(gvk schema.GroupVersionKind) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(gvk.String())).String()
}
