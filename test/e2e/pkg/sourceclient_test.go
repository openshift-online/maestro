package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"
)

var _ = Describe("gRPC Source ManifestWork Client Test", func() {
	Context("Watch work status with gRPC source ManifestWork client", func() {
		var ctx context.Context
		var cancel context.CancelFunc

		var sourceID string
		var grpcOptions *grpc.GRPCOptions

		var watchedWorks []*workv1.ManifestWork

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			sourceID = "sourceclient-test" + rand.String(5)

			watchedWorks = []*workv1.ManifestWork{}

			grpcOptions = grpc.NewGRPCOptions()
			grpcOptions.URL = grpcServerAddress

			workClient, err := work.NewClientHolderBuilder(grpcOptions).
				WithClientID(fmt.Sprintf("%s-watcher", sourceID)).
				WithSourceID(sourceID).
				WithCodecs(codec.NewManifestBundleCodec()).
				WithWorkClientWatcherStore(grpcsource.NewRESTFullAPIWatcherStore(apiClient, sourceID)).
				WithResyncEnabled(false).
				NewSourceClientHolder(ctx)
			Expect(err).ShouldNot(HaveOccurred())

			watcher, err := workClient.ManifestWorks(consumer_name).Watch(ctx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			go func() {
				ch := watcher.ResultChan()
				for {
					select {
					case <-ctx.Done():
						return
					case event, ok := <-ch:
						if !ok {
							return
						}
						switch event.Type {
						case watch.Modified:
							if work, ok := event.Object.(*workv1.ManifestWork); ok {
								watchedWorks = append(watchedWorks, work)
							}
						case watch.Deleted:
							if work, ok := event.Object.(*workv1.ManifestWork); ok {
								watchedWorks = append(watchedWorks, work)
							}
						}
					}
				}
			}()
		})

		AfterEach(func() {
			cancel()
		})

		It("The work status should be watched", func() {
			workClient, err := work.NewClientHolderBuilder(grpcOptions).
				WithClientID(fmt.Sprintf("%s-client", sourceID)).
				WithSourceID(sourceID).
				WithCodecs(codec.NewManifestBundleCodec()).
				WithWorkClientWatcherStore(grpcsource.NewRESTFullAPIWatcherStore(apiClient, sourceID)).
				WithResyncEnabled(false).
				NewSourceClientHolder(ctx)
			Expect(err).ShouldNot(HaveOccurred())

			By("create a work")
			workName := "work-" + rand.String(5)
			_, err = workClient.ManifestWorks(consumer_name).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("list the works")
			works, err := workClient.ManifestWorks(consumer_name).List(ctx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(works.Items) == 1).To(BeTrue())

			// wait for few seconds to ensure the work status is updated by agent
			<-time.After(5 * time.Second)

			By("update a work")
			work, err := workClient.ManifestWorks(consumer_name).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err := ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = workClient.ManifestWorks(consumer_name).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the work status is updated by agent
			<-time.After(5 * time.Second)

			By("delete a work")
			err = workClient.ManifestWorks(consumer_name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if len(watchedWorks) < 2 {
					return fmt.Errorf("unexpected watched works %v", watchedWorks)
				}

				hasDeletedWork := false
				for _, watchedWork := range watchedWorks {
					if meta.IsStatusConditionTrue(watchedWork.Status.Conditions, common.ManifestsDeleted) {
						hasDeletedWork = true
						break
					}
				}

				if !hasDeletedWork {
					return fmt.Errorf("expected the deleted works is watched, but failed")
				}

				return nil
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

		})
	})
})

func NewManifestWork(name string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					NewManifest(name),
				},
			},
		},
	}
}

func ToWorkPatch(old, new *workv1.ManifestWork) ([]byte, error) {
	oldData, err := json.Marshal(old)
	if err != nil {
		return nil, err
	}

	newData, err := json.Marshal(new)
	if err != nil {
		return nil, err
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, err
	}

	return patchBytes, nil
}

func NewManifest(name string) workv1.Manifest {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      name,
			},
			"data": map[string]string{
				"test": rand.String(5),
			},
		},
	}
	objectStr, _ := obj.MarshalJSON()
	manifest := workv1.Manifest{}
	manifest.Raw = objectStr
	return manifest
}
