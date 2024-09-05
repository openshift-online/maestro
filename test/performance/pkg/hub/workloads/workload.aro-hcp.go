package workloads

import (
	"embed"
	"fmt"

	"github.com/openshift-online/maestro/test/performance/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	workv1 "open-cluster-management.io/api/work/v1"
)

//go:embed manifests
var ManifestFiles embed.FS

var aroHCPWorkloadFiles = []string{
	"manifests/aro-hcp/managedcluster.yaml",
	"manifests/aro-hcp/manifestwork.namespace.yaml",
	"manifests/aro-hcp/manifestwork.hypershift.yaml",
}

func ToAROHCPManifestWorks(clusterName string) ([]*workv1.ManifestWork, error) {
	works := []*workv1.ManifestWork{}

	cluster := string(uuid.NewUUID())
	clusterManifest, err := toManifest(clusterName, cluster, aroHCPWorkloadFiles[0])
	if err != nil {
		return nil, err
	}

	work := string(uuid.NewUUID())
	namespace := fmt.Sprintf("%s-ns", work)
	namespaceManifest, err := toManifest(clusterName, namespace, aroHCPWorkloadFiles[1])
	if err != nil {
		return nil, err
	}

	hypershift := fmt.Sprintf("%s-hs", work)
	hypershiftManifest, err := toManifest(clusterName, hypershift, aroHCPWorkloadFiles[2])
	if err != nil {
		return nil, err
	}

	works = append(works, &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster,
			Namespace: clusterName,
			Labels: map[string]string{
				"maestro.performance.test":         "mc",
				"cluster.maestro.performance.test": clusterName,
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{Raw: clusterManifest},
					},
				},
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
					ResourceIdentifier: workv1.ResourceIdentifier{
						Name:     cluster,
						Group:    "cluster.open-cluster-management.io",
						Resource: "managedclusters",
					},
				},
			},
		},
	})
	works = append(works, &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      work,
			Namespace: clusterName,
			Labels: map[string]string{
				"maestro.performance.test":         "hypershift",
				"cluster.maestro.performance.test": clusterName,
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{Raw: namespaceManifest},
					},
					{
						RawExtension: runtime.RawExtension{Raw: hypershiftManifest},
					},
				},
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
					ResourceIdentifier: workv1.ResourceIdentifier{
						Name:      fmt.Sprintf("%s-namespace", namespace),
						Namespace: clusterName,
						Group:     "work.open-cluster-management.io",
						Resource:  "manifestworks",
					},
				},
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
					ResourceIdentifier: workv1.ResourceIdentifier{
						Name:      fmt.Sprintf("%s-hypershift", hypershift),
						Namespace: clusterName,
						Group:     "work.open-cluster-management.io",
						Resource:  "manifestworks",
					},
				},
			},
		},
	})

	return works, nil
}

func toManifest(clusterName, name, file string) ([]byte, error) {
	data, err := ManifestFiles.ReadFile(file)
	if err != nil {
		return nil, err
	}

	raw, err := util.Render(
		file,
		data,
		&struct {
			Name             string
			ClusterName      string
			DockerConfigJSON string
			IDRsa            string
			IDRsaPub         string
			SecretKey        string
			HTPasswd         string
			AzureClientInfo  string
			SubID            string
			BaseDomain       string
		}{
			Name:             name,
			ClusterName:      clusterName,
			DockerConfigJSON: rand.String(236),
			IDRsa:            rand.String(2272),
			IDRsaPub:         rand.String(604),
			SecretKey:        rand.String(44),
			HTPasswd:         rand.String(92),
			AzureClientInfo:  rand.String(52),
			SubID:            string(uuid.NewUUID()),
			BaseDomain:       "az.test.red-chesterfield-test.com",
		},
	)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
