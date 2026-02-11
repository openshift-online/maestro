package services

import (
	"fmt"
	"strings"

	"gorm.io/datatypes"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift-online/maestro/pkg/api"
)

func ValidateResourceName(resource *api.Resource) error {
	errs := field.ErrorList{}
	for _, msg := range apivalidation.ValidateNamespaceName(resource.Name, false) {
		errs = append(errs, field.Invalid(field.NewPath("resource").Child("name"), resource.Name, msg))
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", errs.ToAggregate().Error())
}

func ValidateConsumer(consumer *api.Consumer) error {
	errs := field.ErrorList{}
	for _, msg := range apivalidation.ValidateNamespaceName(consumer.Name, false) {
		errs = append(errs, field.Invalid(field.NewPath("consumer").Child("name"), consumer.Name, msg))
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", errs.ToAggregate().Error())
}

func ValidateManifestBundle(manifestBundle datatypes.JSONMap) error {
	manifestBundleWrapper, err := api.DecodeManifestBundle(manifestBundle)
	if err != nil {
		return fmt.Errorf("failed to decode manifest bundle: %v", err)
	}
	if manifestBundleWrapper == nil {
		return fmt.Errorf("manifest bundle is empty")
	}

	// Track seen manifests to detect duplicates
	seen := sets.New[string]()

	for i, manifest := range manifestBundleWrapper.Manifests {
		if err := ValidateObject(manifest); err != nil {
			return err
		}
		// Check for duplicate manifests
		info, err := extractManifestInfo(manifest)
		if err != nil {
			return fmt.Errorf("failed to extract metadata from manifest at index %d: %w", i, err)
		}

		if seen.Has(info.key) {
			return fmt.Errorf("duplicate manifest for resource %s/%s with resource type %s", info.namespace, info.name, info.gvk)
		}
		seen.Insert(info.key)
	}

	return nil
}

func ValidateObject(obj datatypes.JSONMap) error {
	errs := field.ErrorList{}
	unstructuredObj := unstructured.Unstructured{Object: obj}

	errs = append(errs, validatedAPIVersion(unstructuredObj.GetAPIVersion())...)

	if unstructuredObj.GetKind() == "" {
		errs = append(errs, field.Required(field.NewPath("kind"), "field not set"))
	}

	if unstructuredObj.GetName() == "" {
		errs = append(errs, field.Required(field.NewPath("metadata").Child("name"), "field not set"))
	}

	if unstructuredObj.GetNamespace() != "" {
		ns := unstructuredObj.GetNamespace()
		for _, msg := range apivalidation.ValidateNamespaceName(ns, false) {
			errs = append(errs, field.Invalid(field.NewPath("metadata").Child("namespace"), ns, msg))
		}
	}

	errs = append(errs, validateMetaData(unstructuredObj)...)

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", errs.ToAggregate().Error())
}

// manifestInfo contains the metadata needed for duplicate detection and error messages.
type manifestInfo struct {
	key       string // unique key for duplicate detection: apiVersion/kind/namespace/name
	name      string
	namespace string
	gvk       string // apiVersion.kind format for error messages
}

// extractManifestInfo extracts metadata from a manifest for duplicate detection.
func extractManifestInfo(manifest datatypes.JSONMap) (*manifestInfo, error) {
	unstructuredObj := unstructured.Unstructured{Object: manifest}
	return &manifestInfo{
		key: fmt.Sprintf("%s/%s/%s/%s", unstructuredObj.GetAPIVersion(), unstructuredObj.GetKind(),
			unstructuredObj.GetNamespace(), unstructuredObj.GetName()),
		name:      unstructuredObj.GetName(),
		namespace: unstructuredObj.GetNamespace(),
		gvk:       fmt.Sprintf("%s.%s", unstructuredObj.GetAPIVersion(), unstructuredObj.GetKind()),
	}, nil
}

// validatedAPIVersion tests whether the value passed is a valid apiVersion. A
// valid apiVersion contains a version string that matches DNS_LABEL format,
// with an optional group/ prefix, where the group string matches DNS_SUBDOMAIN
// format. If the value is not valid, a list of error strings is returned.
// Otherwise an empty list (or nil) is returned.
func validatedAPIVersion(apiVersion string) field.ErrorList {
	var version string

	fldPath := field.NewPath("apiVersion")
	errs := field.ErrorList{}

	if apiVersion == "" {
		errs = append(errs, field.Required(fldPath, "field not set"))
		return errs
	}

	parts := strings.Split(apiVersion, "/")
	switch len(parts) {
	case 1:
		version = parts[0]
	case 2:
		var group string
		group, version = parts[0], parts[1]
		if len(group) == 0 {
			errs = append(errs, field.Invalid(fldPath, apiVersion, "group not set"))
		} else {
			for _, msg := range utilvalidation.IsDNS1123Subdomain(group) {
				errs = append(errs, field.Invalid(fldPath, apiVersion, msg))
			}
		}
	default:
		errs = append(errs, field.Invalid(fldPath, apiVersion, "bad format"))
		return errs
	}

	if len(version) == 0 {
		errs = append(errs, field.Invalid(fldPath, apiVersion, "version not set"))
	} else {
		for _, msg := range utilvalidation.IsDNS1035Label(version) {
			errs = append(errs, field.Invalid(fldPath, apiVersion, msg))
		}
	}

	return errs
}

func validateMetaData(unstructuredObj unstructured.Unstructured) field.ErrorList {
	fldPath := field.NewPath("metadata")
	errs := field.ErrorList{}

	if unstructuredObj.GetGenerateName() != "" {
		errs = append(errs, field.Forbidden(fldPath.Child("generateName"), "field cannot be set"))
	}

	if unstructuredObj.GetResourceVersion() != "" {
		errs = append(errs, field.Forbidden(fldPath.Child("resourceVersion"), "field cannot be set"))
	}

	if unstructuredObj.GetDeletionGracePeriodSeconds() != nil {
		errs = append(errs, field.Forbidden(fldPath.Child("deletionGracePeriodSeconds"), "field cannot be set"))
	}

	errs = append(errs, apivalidation.ValidateAnnotations(unstructuredObj.GetAnnotations(), fldPath.Child("annotations"))...)
	errs = append(errs, apivalidation.ValidateFinalizers(unstructuredObj.GetFinalizers(), fldPath.Child("finalizers"))...)
	errs = append(errs, v1validation.ValidateLabels(unstructuredObj.GetLabels(), fldPath.Child("labels"))...)

	return errs
}
