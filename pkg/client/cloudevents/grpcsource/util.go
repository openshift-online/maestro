package grpcsource

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/payload"
)

const jsonbPrefix = `payload->'metadata'->'labels'`

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

func ToLabelSearch(opts metav1.ListOptions) (labels.Selector, string, bool, error) {
	if len(opts.LabelSelector) == 0 {
		return labels.Everything(), "", false, nil
	}

	labelSelector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, "", false, fmt.Errorf("invalid labels selector %q: %v", opts.LabelSelector, err)
	}

	requirements, selectable := labelSelector.Requirements()
	if !selectable {
		return labels.Everything(), "", false, nil
	}

	equalsLabels := []string{}
	notEqualsLabels := []string{}

	existsKeys := []string{}
	doesNotExistKeys := []string{}

	inLabels := map[string][]string{}

	// refer to below links to find how to use the label selector in kubernetes
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#equality-based-requirement
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#set-based-requirement
	for _, requirement := range requirements {
		switch requirement.Operator() {
		case selection.Equals, selection.DoubleEquals:
			values := requirement.Values()
			if len(values) != 1 {
				return nil, "", false, fmt.Errorf("too many values in equals operation")
			}

			equalsLabels = append(equalsLabels, fmt.Sprintf(`"%s":"%s"`, requirement.Key(), values.List()[0]))
		case selection.NotEquals:
			values := requirement.Values()
			if len(values) != 1 {
				return nil, "", false, fmt.Errorf("too many values in not equals operation")
			}

			notEqualsLabels = append(notEqualsLabels, fmt.Sprintf(`%s->>'%s'<>'%s'`, jsonbPrefix, requirement.Key(), values.List()[0]))
		case selection.Exists:
			existsKeys = append(existsKeys, fmt.Sprintf(`%s->>'%s'<>null`, jsonbPrefix, requirement.Key()))
		case selection.In:
			vals := []string{}
			for val := range requirement.Values() {
				vals = append(vals, fmt.Sprintf("'%s'", val))
			}

			inLabels[requirement.Key()] = vals
		case selection.NotIn:
			for val := range requirement.Values() {
				notEqualsLabels = append(notEqualsLabels, fmt.Sprintf(`%s->>'%s'<>'%s'`, jsonbPrefix, requirement.Key(), val))
			}
		default:
			// only DoesNotExist cannot be supported
			return nil, "", false, fmt.Errorf("unsupported operator %s", requirement.Operator())
		}
	}

	labelSearch := []string{}
	if len(equalsLabels) != 0 {
		labelSearch = append(labelSearch, fmt.Sprintf(`%s@>'{%s}'`, jsonbPrefix, strings.Join(equalsLabels, ",")))
	}

	if len(inLabels) != 0 {
		for key, vals := range inLabels {
			labelSearch = append(labelSearch, fmt.Sprintf(`%s->>'%s'in(%s)`, jsonbPrefix, key, strings.Join(vals, ",")))
		}
	}

	labelSearch = append(labelSearch, notEqualsLabels...)
	labelSearch = append(labelSearch, existsKeys...)
	labelSearch = append(labelSearch, doesNotExistKeys...)
	return labelSelector, strings.Join(labelSearch, " and "), true, nil
}

func marshal(obj map[string]any) ([]byte, error) {
	unstructuredObj := unstructured.Unstructured{Object: obj}
	data, err := unstructuredObj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return data, nil
}
