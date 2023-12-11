package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

var _ = Describe("Resources", Ordered, Label("e2e-tests-resources"), func() {

	It("is CRUD tests", func() {
	})

	var data = `
{
	"consumer_id": "%s",
	"version": %d,
	"manifest": {
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "nginx",
			"namespace": "default"
		},
		"spec": {
			"replicas": %d,
			"selector": {
				"matchLabels": {
					"app": "nginx"
				}
			},
			"template": {
				"metadata": {
					"labels": {
						"app": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"image": "nginxinc/nginx-unprivileged",
							"name": "nginx"
						}
					]
				}
			}
		}
	}
}`
	var resource_id string

	Context("Create Resource", func() {

		It("post the nginx resource to the maestro api", func() {

			responseBody, err := sendHTTPRequest(http.MethodPost, apiServerAddress+"/api/maestro/v1/resources",
				bytes.NewBuffer([]byte(fmt.Sprintf(data, consumer_id, 0, 1))))
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Println(string(responseBody))

			var resource openapi.Resource
			err = json.Unmarshal(responseBody, &resource)
			Ω(err).ShouldNot(HaveOccurred())

			resource_id = *resource.Id
			Ω(resource_id).ShouldNot(BeEmpty())

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})

	Context("Patch Resource", func() {

		It("patch the nginx resource", func() {

			responseBody, err := sendHTTPRequest(http.MethodPatch, apiServerAddress+"/api/maestro/v1/resources/"+resource_id,
				bytes.NewBuffer([]byte(fmt.Sprintf(data, consumer_id, 0, 2))))
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Println(string(responseBody))

			Eventually(func() error {
				deploy, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}
				if *deploy.Spec.Replicas == 2 {
					return nil
				}
				return fmt.Errorf("replicas is not 2")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})

	Context("Delete Resource", func() {

		It("delete the nginx resource", func() {

			responseBody, err := sendHTTPRequest(http.MethodDelete, apiServerAddress+"/api/maestro/v1/resources/"+resource_id, nil)
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Println(string(responseBody))

			Eventually(func() error {
				_, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})

})
