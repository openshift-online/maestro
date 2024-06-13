package grpcsource

import (
	"encoding/json"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

// ToManifestWork converts an openapi.ResourceBundle object to workv1.ManifestWork object
func ToManifestWork(rb *openapi.ResourceBundle) (*workv1.ManifestWork, error) {
	work := &workv1.ManifestWork{}

	// get meta from resource
	metaJson, err := marshal(rb.Metadata)
	if err != nil {
		return nil, err
	}
	objectMeta := metav1.ObjectMeta{}
	if err := json.Unmarshal(metaJson, &objectMeta); err != nil {
		return nil, err
	}
	work.ObjectMeta = objectMeta

	// get spec from resource
	manifests := []workv1.Manifest{}
	for _, manifest := range rb.Manifests {
		raw, err := marshal(manifest)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: raw}})
	}
	work.Spec.Workload.Manifests = manifests

	if len(rb.DeleteOption) != 0 {
		optionJson, err := marshal(rb.DeleteOption)
		if err != nil {
			return nil, err
		}
		option := &workv1.DeleteOption{}
		if err := json.Unmarshal(optionJson, option); err != nil {
			return nil, err
		}
		work.Spec.DeleteOption = option
	}

	configs := []workv1.ManifestConfigOption{}
	for _, manifestConfig := range rb.ManifestConfigs {
		configJson, err := marshal(manifestConfig)
		if err != nil {
			return nil, err
		}
		config := workv1.ManifestConfigOption{}
		if err := json.Unmarshal(configJson, &config); err != nil {
			return nil, err
		}
		configs = append(configs, config)

	}
	work.Spec.ManifestConfigs = configs

	// get status from resource
	if len(rb.Status) != 0 {
		status, err := json.Marshal(rb.Status)
		if err != nil {
			return nil, err
		}
		manifestStatus := &payload.ManifestBundleStatus{}
		if err := json.Unmarshal(status, manifestStatus); err != nil {
			return nil, err
		}

		work.Status = workv1.ManifestWorkStatus{
			Conditions: manifestStatus.Conditions,
			ResourceStatus: workv1.ManifestResourceStatus{
				Manifests: manifestStatus.ResourceStatus,
			},
		}
	}

	return work, nil
}

func marshal(obj map[string]any) ([]byte, error) {
	unstructuredObj := unstructured.Unstructured{Object: obj}
	data, err := unstructuredObj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return data, nil
}
