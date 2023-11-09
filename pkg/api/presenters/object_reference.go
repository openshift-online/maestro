package presenters

import (
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

func PresentReference(id, obj interface{}) openapi.ObjectReference {
	refId, ok := makeReferenceId(id)

	if !ok {
		return openapi.ObjectReference{}
	}

	return openapi.ObjectReference{
		Id:   openapi.PtrString(refId),
		Kind: ObjectKind(obj),
		Href: ObjectPath(refId, obj),
	}
}

func makeReferenceId(id interface{}) (string, bool) {
	var refId string

	if i, ok := id.(string); ok {
		refId = i
	}

	if i, ok := id.(*string); ok {
		if i != nil {
			refId = *i
		}
	}

	return refId, refId != ""
}
