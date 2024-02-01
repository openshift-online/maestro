package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"

	"k8s.io/apimachinery/pkg/api/meta"

	workv1 "open-cluster-management.io/api/work/v1"
)

const sleepJob = `
{
	"apiVersion": "batch/v1",
	"kind": "Job",
	"metadata": {
	  "name": "sleep",
	  "namespace": "default"
	},
	"spec": {
	  "template": {
		"spec": {
		  "containers": [
			{
			  "name": "sleep",
			  "image": "busybox:1.36",
			  "command": [
				"/bin/sh",
				"-c",
				"sleep 10"
			  ]
			}
		  ],
		  "restartPolicy": "Never"
		}
	  },
	  "backoffLimit": 4
	}
}
`

var _ = Describe("Server Side Apply", func() {
	It("Apply a job with maestro", func() {
		// The kube-apiserver will set a default selector and label on the Pod of Job if the job does not have
		// spec.Selector, these fields are immutable, if we use update strategy to apply Job, it will report
		// AppliedManifestFailed. The maestro uses the server side strategy to apply a resource with ManifestWork
		// by default, this will avoid this.
		manifest := map[string]interface{}{}
		Expect(json.Unmarshal([]byte(sleepJob), &manifest)).ShouldNot(HaveOccurred())

		res := openapi.Resource{
			Manifest:   manifest,
			ConsumerId: &consumer_id,
		}

		created, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesPost(context.Background()).Resource(res).Execute()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))
		Expect(*created.Id).ShouldNot(BeEmpty())

		resourceID := *created.Id
		Eventually(func() error {
			found, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(context.Background(), resourceID).Execute()
			if err != nil {
				return err
			}

			if found.Status == nil {
				return fmt.Errorf("the resource %s status is nil", resourceID)
			}

			resourceStatus, err := api.JSONMapStatusToResourceStatus(found.Status)
			if err != nil {
				return err
			}

			conditions := resourceStatus.ReconcileStatus.Conditions

			if meta.IsStatusConditionFalse(conditions, workv1.WorkApplied) {
				return fmt.Errorf("unexpected condition %v for resource %s", conditions, resourceID)
			}

			if meta.IsStatusConditionFalse(conditions, workv1.WorkAvailable) {
				return fmt.Errorf("unexpected condition %v for resource %s", conditions, resourceID)
			}

			if meta.IsStatusConditionFalse(conditions, "StatusFeedbackSynced") {
				return fmt.Errorf("unexpected condition %v for resource %s", conditions, resourceID)
			}

			return nil
		}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
	})
})
