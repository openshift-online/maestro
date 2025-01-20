package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	workv1 "open-cluster-management.io/api/work/v1"
)

const sleepJob = `
{
	"apiVersion": "batch/v1",
	"kind": "Job",
	"metadata": {
	  "name": "%s",
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

var _ = Describe("Server Side Apply", Ordered, Label("e2e-tests-serverside-apply"), func() {
	It("Apply a job with maestro", func() {
		// The kube-apiserver will set a default selector and label on the Pod of Job if the job does not have
		// spec.Selector, these fields are immutable, if we use update strategy to apply Job, it will report
		// AppliedManifestFailed. The maestro uses the server side strategy to apply a resource with ManifestWork
		// by default, this will avoid this.
		manifest := map[string]interface{}{}
		sleepJobName := fmt.Sprintf("sleep-%s", rand.String(5))
		err := json.Unmarshal([]byte(fmt.Sprintf(sleepJob, sleepJobName)), &manifest)
		Expect(err).ShouldNot(HaveOccurred())

		res := openapi.Resource{
			Manifest:     manifest,
			ConsumerName: &agentTestOpts.consumerName,
			ManifestConfig: map[string]interface{}{
				"resourceIdentifier": map[string]interface{}{
					"group":     "batch",
					"resource":  "jobs",
					"name":      sleepJobName,
					"namespace": "default",
				},
			},
		}

		created, resp, err := apiClient.DefaultApi.ApiMaestroV1ResourcesPost(ctx).Resource(res).Execute()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))
		Expect(*created.Id).ShouldNot(BeEmpty())

		resourceID := *created.Id
		Eventually(func() error {
			found, _, err := apiClient.DefaultApi.ApiMaestroV1ResourcesIdGet(ctx, resourceID).Execute()
			if err != nil {
				return err
			}

			if found.Status == nil {
				return fmt.Errorf("the resource %s status is nil", resourceID)
			}

			statusJSON, err := json.Marshal(found.Status)
			if err != nil {
				return fmt.Errorf("failed to marshal status to JSON: %v", err)
			}
			resourceStatus := &api.ResourceStatus{}
			if err := json.Unmarshal(statusJSON, resourceStatus); err != nil {
				return fmt.Errorf("failed to unmarshal status JSON to ResourceStatus: %v", err)
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

		// cleanup the job
		resp, err = apiClient.DefaultApi.ApiMaestroV1ResourcesIdDelete(ctx, resourceID).Execute()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
	})

	It("Apply a nested work with SSA", func() {
		workName := fmt.Sprintf("ssa-work-%s", rand.String(5))
		nestedWorkName := fmt.Sprintf("nested-work-%s", rand.String(5))
		nestedWorkNamespace := "default"

		work := NewNestedManifestWork(nestedWorkNamespace, workName, nestedWorkName)
		Eventually(func() error {
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			return err
		}, 5*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

		// make sure the nested work is created
		Eventually(func() error {
			_, err := agentTestOpts.workClientSet.WorkV1().ManifestWorks(nestedWorkNamespace).Get(ctx, nestedWorkName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			return nil
		}, 30*time.Second, time.Second).ShouldNot(HaveOccurred())

		// make sure the nested work is not updated
		Consistently(func() error {
			nestedWork, err := agentTestOpts.workClientSet.WorkV1().ManifestWorks(nestedWorkNamespace).Get(ctx, nestedWorkName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if nestedWork.Generation != 1 {
				return fmt.Errorf("nested work generation is changed to %d", nestedWork.Generation)
			}

			return nil
		}, 1*time.Minute, 1*time.Second).Should(BeNil())

		err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
		Expect(err).ShouldNot(HaveOccurred())
	})
})

func NewNestedManifestWork(nestedWorkNamespace, name, nestedWorkName string) *workv1.ManifestWork {
	nestedWork := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "work.open-cluster-management.io/v1",
			Kind:       "ManifestWork",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nestedWorkName,
			Namespace: nestedWorkNamespace,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{{
					RawExtension: runtime.RawExtension{
						Object: &corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								Kind:       "ConfigMap",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cm-test",
								Namespace: "default",
							},
							Data: map[string]string{
								"some": "data",
							},
						},
					},
				}},
			},
		},
	}

	manifest := workv1.Manifest{}
	manifest.Object = nestedWork

	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{manifest},
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     "work.open-cluster-management.io",
						Resource:  "manifestworks",
						Name:      nestedWorkName,
						Namespace: nestedWorkNamespace,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: workv1.UpdateStrategyTypeServerSideApply,
						ServerSideApply: &workv1.ServerSideApplyConfig{
							Force:        true,
							FieldManager: "maestro-agent",
						},
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
				},
			},
		},
	}
}
