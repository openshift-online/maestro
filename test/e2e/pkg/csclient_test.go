package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"
)

var _ = Describe("CSClient", Ordered, Label("e2e-tests-csclient"), func() {
	Context("Manifestwork CRUD Tests", func() {
		workName := fmt.Sprintf("work-%s", rand.String(5))
		deployName := fmt.Sprintf("nginx-%s", rand.String(5))
		It("create a manifestwork with source work client", func() {
			err := createWork(workName, deployName)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 1 {
					return fmt.Errorf("unexpected replicas, expected 1, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("get the manifestwork via cs api", func() {
			Eventually(func() error {
				got, err := getWork(workName)
				if err != nil {
					return err
				}
				if got.Name != workName {
					return fmt.Errorf("unexpected work name, expected %s, got %s", workName, got.Name)
				}
				if len(got.Spec.Workload.Manifests) != 1 {
					return fmt.Errorf("unexpected number of manifests, expected 1, got %d", len(got.Spec.Workload.Manifests))
				}
				if got.Status.Conditions == nil {
					return fmt.Errorf("expected conditions to be set")
				}
				if !meta.IsStatusConditionTrue(got.Status.Conditions, workv1.WorkApplied) {
					return fmt.Errorf("unexpected condition %v", got.Status.Conditions)
				}
				if !meta.IsStatusConditionTrue(got.Status.Conditions, workv1.WorkAvailable) {
					return fmt.Errorf("unexpected condition %v", got.Status.Conditions)
				}
				if len(got.Status.ResourceStatus.Manifests) != 1 {
					return fmt.Errorf("unexpected number of resource status manifests, expected 1, got %d", len(got.Status.ResourceStatus.Manifests))
				}
				if !meta.IsStatusConditionTrue(got.Status.ResourceStatus.Manifests[0].Conditions, "StatusFeedbackSynced") {
					return fmt.Errorf("unexpected manifest condition %v", got.Status.ResourceStatus.Manifests[0].Conditions)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("update the manifestwork via cs api", func() {
			updating := helper.NewManifestWork(workName, deployName, "default", 2)
			err := updateWork(workName, updating)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas != 2 {
					return fmt.Errorf("unexpected replicas, expected 2, got %d", *deploy.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("delete the manifestwork via cs api", func() {
			err := deleteWork(workName)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("deployment %q still exists", deployName)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("check the manifestwork deletion via cs api", func() {
			Eventually(func() error {
				_, err := getWork(workName)
				if err == nil {
					return fmt.Errorf("expected work to be deleted")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})

func createWork(workName, deployName string) error {
	work := helper.NewManifestWork(workName, deployName, "default", 1)
	data, err := json.Marshal(work)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("%s/works", csServerAddress), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return err
	}

	var created workv1.ManifestWork
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return err
	}
	log.Infof("Created manifestwork %q", created.Name)
	return nil
}

func getWork(name string) (*workv1.ManifestWork, error) {
	url := fmt.Sprintf("%s/works/%s", csServerAddress, name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	var got workv1.ManifestWork
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		return nil, err
	}
	return &got, nil
}

func updateWork(name string, newWork *workv1.ManifestWork) error {
	data, err := json.Marshal(newWork)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/works/%s", csServerAddress, name), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return err
	}

	var updated workv1.ManifestWork
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return err
	}
	log.Infof("Updated manifestwork %q", updated.Name)
	return nil
}

func deleteWork(name string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/works/%s", csServerAddress, name), nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return err
	}
	return nil
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("status: %d, body: %s", resp.StatusCode, string(body))
}
