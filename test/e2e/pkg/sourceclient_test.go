package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
)

var _ = Describe("gRPC Source ManifestWork Client Test", func() {
	Context("Watch work status with gRPC source ManifestWork client", func() {
		var watcherCtx context.Context
		var watcherCancel context.CancelFunc

		var firstInitWorkName string
		var secondInitWorkName string

		BeforeEach(func() {
			watcherCtx, watcherCancel = context.WithCancel(context.Background())

			// prepare two works firstly
			firstInitWorkName = "first-init-work-" + rand.String(5)
			secondInitWorkName = "second-init-work-" + rand.String(5)

			_, err := workClient.ManifestWorks(consumer_name).Create(ctx, NewManifestWork(firstInitWorkName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = workClient.ManifestWorks(consumer_name).Create(ctx, NewManifestWork(secondInitWorkName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := workClient.ManifestWorks(consumer_name).Delete(ctx, firstInitWorkName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = workClient.ManifestWorks(consumer_name).Delete(ctx, secondInitWorkName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := AssertWorkNotFound(firstInitWorkName); err != nil {
					return err
				}

				return AssertWorkNotFound(secondInitWorkName)
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			watcherCancel()
		})

		It("The work status should be watched", func() {
			By("create a work client for watch")
			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				watcherCtx,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start watching")
			watcher, err := watcherClient.ManifestWorks(consumer_name).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			result := StartWatch(watcherCtx, watcher)

			By("create a work by work client")
			workName := "work-" + rand.String(5)
			_, err = workClient.ManifestWorks(consumer_name).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("update a work by work client")
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

			By("delete the work by work client")
			err = workClient.ManifestWorks(consumer_name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(result)
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("The watchers with different namespace", func() {
			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				watcherCtx,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start watching works from all consumers")
			allConsumerWatcher, err := watcherClient.ManifestWorks(metav1.NamespaceAll).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			allConsumerWatcherResult := StartWatch(watcherCtx, allConsumerWatcher)

			By("start watching works from consumer" + consumer_name)
			consumerWatcher, err := watcherClient.ManifestWorks(consumer_name).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			consumerWatcherResult := StartWatch(watcherCtx, consumerWatcher)

			By("start watching works from an other consumer")
			otherConsumerWatcher, err := watcherClient.ManifestWorks("other").Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			otherConsumerWatcherResult := StartWatch(watcherCtx, otherConsumerWatcher)

			By("create a work by work client")
			workName := "work-" + rand.String(5)
			_, err = workClient.ManifestWorks(consumer_name).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work by work client")
			err = workClient.ManifestWorks(consumer_name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(allConsumerWatcherResult)
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(consumerWatcherResult)
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			Consistently(func() error {
				if len(otherConsumerWatcherResult.WatchedWorks) != 0 {
					return fmt.Errorf("unexpected watched works")
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})

type WatchedResult struct {
	WatchedWorks []*workv1.ManifestWork
}

func StartWatch(ctx context.Context, watcher watch.Interface) *WatchedResult {
	result := &WatchedResult{WatchedWorks: []*workv1.ManifestWork{}}
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
						result.WatchedWorks = append(result.WatchedWorks, work)
					}
				case watch.Deleted:
					if work, ok := event.Object.(*workv1.ManifestWork); ok {
						result.WatchedWorks = append(result.WatchedWorks, work)
					}
				}
			}
		}
	}()

	return result
}

func AssertWatchResult(result *WatchedResult) error {
	hasFirstInitWork := false
	hasSecondInitWork := false
	hasWork := false
	hasDeletedWork := false

	for _, watchedWork := range result.WatchedWorks {
		if strings.HasPrefix(watchedWork.Name, "first-init-work-") {
			hasFirstInitWork = true
		}

		if strings.HasPrefix(watchedWork.Name, "second-init-work-") {
			hasSecondInitWork = true
		}

		if strings.HasPrefix(watchedWork.Name, "work-") {
			hasWork = true
		}

		if meta.IsStatusConditionTrue(watchedWork.Status.Conditions, common.ManifestsDeleted) {
			hasDeletedWork = true
		}
	}

	if !hasFirstInitWork {
		return fmt.Errorf("expected the first init works is watched, but failed")
	}

	if !hasSecondInitWork {
		return fmt.Errorf("expected the second init works is watched, but failed")
	}

	if !hasWork {
		return fmt.Errorf("expected the works is watched, but failed")
	}

	if !hasDeletedWork {
		return fmt.Errorf("expected the deleted works is watched, but failed")
	}

	return nil
}

func AssertWorkNotFound(name string) error {
	_, err := workClient.ManifestWorks(consumer_name).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	return fmt.Errorf("the work %s still exists", name)
}

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
