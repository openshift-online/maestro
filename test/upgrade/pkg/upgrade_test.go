package upgrade_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/maestro/test/upgrade/pkg/utils"
)

const (
	namespace          = "default"
	deployName         = "maestro-e2e-upgrade-test"
	deployReadonlyName = "maestro-e2e-upgrade-test-readonly"
	nestedWorkName     = "maestro-e2e-upgrade-test-work"
)

const (
	timeout = 2 * time.Minute
	polling = 10 * time.Second
)

var _ = Describe("Upgrade Test", Ordered, Label("e2e-tests-upgrade"), func() {

	var deployWorkName string
	var deployReadonlyWorkName string
	var nestedWorkWorkName string

	BeforeAll(func() {
		deployClient := kubeClientSet.AppsV1().Deployments(namespace)

		By("create a deployment for readonly work", func() {
			Eventually(func() error {
				_, err := deployClient.Get(ctx, deployReadonlyName, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					deploy := utils.NewDeployment(namespace, deployReadonlyName, 0)
					_, newErr := deployClient.Create(ctx, deploy, metav1.CreateOptions{})
					return newErr
				}
				if err != nil {
					return err
				}
				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("create a work to apply a deployment", func() {
			deploy := utils.NewDeployment(namespace, deployName, 0)
			deployWorkName = utils.WorkName(utils.DeploymentGVK, deploy)

			Eventually(func() error {
				_, err := workServerClient.Get(ctx, deployWorkName)
				if errors.IsNotFound(err) {
					work, newErr := utils.NewManifestWork(utils.DeploymentGVK, utils.DeploymentGVR, deploy)
					if newErr != nil {
						return newErr
					}

					work.Labels["maestro.e2e.test.name"] = "upgrade"
					_, newErr = workServerClient.Create(ctx, work)
					return newErr
				}
				if err != nil {
					return err
				}
				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("create a readonly work to retrieve a deployment status", func() {
			deployReadonly := utils.NewDeploymentReadonly(namespace, deployReadonlyName)
			deployReadonlyWorkName = utils.WorkName(utils.DeploymentGVK, deployReadonly)
			Eventually(func() error {
				_, err := workServerClient.Get(ctx, deployReadonlyWorkName)
				if errors.IsNotFound(err) {
					work, newErr := utils.NewManifestWork(utils.DeploymentGVK, utils.DeploymentGVR, deployReadonly)
					if newErr != nil {
						return newErr
					}

					work.Labels["maestro.e2e.test.name"] = "upgrade"
					_, newErr = workServerClient.Create(ctx, work)
					return newErr
				}
				if err != nil {
					return err
				}
				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("create a work to apply a nested manifestwork", func() {
			nestedWork := utils.NewDeploymentManifestWork(namespace, nestedWorkName)
			nestedWorkWorkName = utils.WorkName(utils.ManifestWorkGVK, nestedWork)
			Eventually(func() error {
				_, err := workServerClient.Get(ctx, nestedWorkWorkName)
				if errors.IsNotFound(err) {
					work, newErr := utils.NewManifestWork(utils.ManifestWorkGVK, utils.ManifestWorkGVR, nestedWork)
					if newErr != nil {
						return newErr
					}

					work.Labels["maestro.e2e.test.name"] = "upgrade"
					_, newErr = workServerClient.Create(ctx, work)
					return newErr
				}
				if err != nil {
					return err
				}
				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})
	})

	It("update deployment via a work", func() {
		var lastGeneration int64
		var lastReplicas int32
		var expectedReplicas int32

		deployClient := kubeClientSet.AppsV1().Deployments(namespace)

		By("ensure the deployment is applied", func() {
			Eventually(func() error {
				deploy, err := deployClient.Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				lastGeneration = deploy.Generation
				lastReplicas = *deploy.Spec.Replicas
				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		expectedReplicas = utils.UpdateReplicas(lastReplicas)
		By("updated deployment via a work", func() {
			Eventually(func() error {
				work, err := workServerClient.Get(ctx, deployWorkName)
				if err != nil {
					return fmt.Errorf("failed to get work: %v", err)
				}

				newWork := work.DeepCopy()
				newWork.Spec.Workload.Manifests = []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Object: utils.NewDeployment(namespace, deployName, expectedReplicas),
						},
					},
				}
				patchData, err := grpcsource.ToWorkPatch(work, newWork)
				if err != nil {
					return fmt.Errorf("failed to generate work patch: %v", err)
				}

				_, err = workServerClient.Patch(ctx, deployWorkName, patchData)
				if err != nil {
					return fmt.Errorf("failed to patch work: %v", err)
				}

				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("ensure the deployment is updated", func() {
			Eventually(func() error {
				deploy, err := deployClient.Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if deploy.Generation == lastGeneration {
					return fmt.Errorf("expected deployment %s updated, but failed", deployName)
				}

				if *deploy.Spec.Replicas != expectedReplicas {
					return fmt.Errorf("expected replicas %d, but got %d", expectedReplicas, *deploy.Spec.Replicas)
				}

				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("ensure the deployment new status is watched", func() {
			Eventually(func() error {
				watchedWork, err := workServerClient.Get(ctx, deployWorkName)
				if err != nil {
					return err
				}
				return AssertReplicas(watchedWork, expectedReplicas)
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})
	})

	It("watch deployment status via a readonly work", func() {
		var expectedReplicas int32

		By("ensure the deployment status is watched", func() {
			Eventually(func() error {
				watchedWork, err := workServerClient.Get(ctx, deployReadonlyWorkName)
				if err != nil {
					return err
				}
				return AssertStatusFeedbackSynced(watchedWork)
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("update the deployment", func() {
			Eventually(func() error {
				deployClient := kubeClientSet.AppsV1().Deployments(namespace)
				deploy, err := deployClient.Get(ctx, deployReadonlyName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				expectedReplicas = utils.UpdateReplicas(*deploy.Spec.Replicas)
				newDeploy := deploy.DeepCopy()
				newDeploy.Spec.Replicas = ptr.To(expectedReplicas)

				_, err = deployClient.Update(ctx, newDeploy, metav1.UpdateOptions{})
				if err != nil {
					return err
				}

				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())

		})

		By("ensure the deployment new status is watched", func() {
			Eventually(func() error {
				watchedWork, err := workServerClient.Get(ctx, deployReadonlyWorkName)
				if err != nil {
					return err
				}
				return AssertReplicas(watchedWork, expectedReplicas)
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})
	})

	It("update nested work via a work", func() {
		workClient := workClientSet.WorkV1().ManifestWorks(namespace)

		By("ensure the nested work is applied", func() {
			Eventually(func() error {
				_, err := workClient.Get(ctx, nestedWorkName, metav1.GetOptions{})
				return err
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("update the nested work via work", func() {
			Eventually(func() error {
				work, err := workServerClient.Get(ctx, nestedWorkWorkName)
				if err != nil {
					return err
				}

				newWork := work.DeepCopy()
				newWork.Spec.Workload.Manifests = []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Object: utils.NewDeploymentManifestWork(namespace, nestedWorkName),
						},
					},
				}
				patchData, err := grpcsource.ToWorkPatch(work, newWork)
				if err != nil {
					return fmt.Errorf("failed to generate work patch: %v", err)
				}

				_, err = workServerClient.Patch(ctx, nestedWorkWorkName, patchData)
				if err != nil {
					return fmt.Errorf("failed to patch work: %v", err)
				}

				return nil
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})

		By("ensure the manifestwork new status is watched", func() {
			Eventually(func() error {
				_, err := workClient.Get(ctx, nestedWorkName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				watchedWork, err := workServerClient.Get(ctx, nestedWorkWorkName)
				if err != nil {
					return err
				}
				return AssertNestedWorkAvailable(watchedWork)
			}).WithTimeout(timeout).WithPolling(polling).ShouldNot(HaveOccurred())
		})
	})
})

func AssertStatusFeedbackSynced(watchedWork *workv1.ManifestWork) error {
	for _, manifest := range watchedWork.Status.ResourceStatus.Manifests {
		if meta.IsStatusConditionTrue(manifest.Conditions, "StatusFeedbackSynced") {
			return nil
		}
	}

	return fmt.Errorf("the work %s status feedback is not synced", watchedWork.Name)
}

func AssertReplicas(watchedWork *workv1.ManifestWork, replicas int32) error {
	for _, manifest := range watchedWork.Status.ResourceStatus.Manifests {
		if meta.IsStatusConditionTrue(manifest.Conditions, "StatusFeedbackSynced") {
			feedbackJson, err := json.Marshal(manifest.StatusFeedbacks)
			if err != nil {
				return err
			}

			if strings.Contains(string(feedbackJson), fmt.Sprintf(`readyReplicas\":%d`, replicas)) {
				return nil
			}
		}
	}

	return fmt.Errorf("the expected replicas %d is not found from feedback", replicas)
}

func AssertNestedWorkAvailable(watchedWork *workv1.ManifestWork) error {
	if meta.IsStatusConditionTrue(watchedWork.Status.Conditions, "Applied") &&
		meta.IsStatusConditionTrue(watchedWork.Status.Conditions, "Available") {
		return nil
	}

	return fmt.Errorf("the work %s is not available", watchedWork.Name)
}
