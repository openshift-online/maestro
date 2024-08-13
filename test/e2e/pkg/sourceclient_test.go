package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/common"
)

var _ = Describe("gRPC Source ManifestWork Client Test", func() {
	Context("Update an obsolete work", func() {
		var workName string

		BeforeEach(func() {
			workName = "work-" + rand.String(5)
			work := NewManifestWork(workName)
			Eventually(func() error {
				_, err := workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
				return err
			}, 5*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)
		})

		AfterEach(func() {
			err := workClient.ManifestWorks(consumer.Name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}, 60*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

		})

		It("Should return an error when updating an obsolete work", func() {
			By("update a work by work client")
			work, err := workClient.ManifestWorks(consumer.Name).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = workClient.ManifestWorks(consumer.Name).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("update the work by work client again")
			obsoleteWork := work.DeepCopy()
			obsoleteWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err = grpcsource.ToWorkPatch(work, obsoleteWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = workClient.ManifestWorks(consumer.Name).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).Should(HaveOccurred())
			Expect(strings.Contains(err.Error(), "the resource version is not the latest")).Should(BeTrue())
		})
	})

	Context("Watch work status with gRPC source ManifestWork client", func() {
		var watcherCtx context.Context
		var watcherCancel context.CancelFunc

		var initWorkAName string
		var initWorkBName string

		BeforeEach(func() {
			watcherCtx, watcherCancel = context.WithCancel(ctx)

			// prepare two works firstly
			initWorkAName = "init-work-a-" + rand.String(5)
			work := NewManifestWorkWithLabels(initWorkAName, map[string]string{"app": "test"})

			_, err := workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			initWorkBName = "init-work-b-" + rand.String(5)
			work = NewManifestWorkWithLabels(initWorkBName, map[string]string{"app": "test"})
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := workClient.ManifestWorks(consumer.Name).Delete(ctx, initWorkAName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, initWorkBName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := AssertWorkNotFound(initWorkAName); err != nil {
					return err
				}

				return AssertWorkNotFound(initWorkBName)
			}, 60*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

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
			watcher, err := watcherClient.ManifestWorks(consumer.Name).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			result := StartWatch(watcherCtx, watcher)

			By("create a work by work client")
			workName := "work-" + rand.String(5)
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("update a work by work client")
			work, err := workClient.ManifestWorks(consumer.Name).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = workClient.ManifestWorks(consumer.Name).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the work status is updated by agent
			<-time.After(5 * time.Second)

			By("delete the work by work client")
			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(result)
			}, 60*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
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

			By("start watching works from consumer" + consumer.Name)
			consumerWatcher, err := watcherClient.ManifestWorks(consumer.Name).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			consumerWatcherResult := StartWatch(watcherCtx, consumerWatcher)

			By("start watching works from an other consumer")
			otherConsumerWatcher, err := watcherClient.ManifestWorks("other").Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			otherConsumerWatcherResult := StartWatch(watcherCtx, otherConsumerWatcher)

			By("create a work by work client")
			workName := "work-" + rand.String(5)
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work by work client")
			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(allConsumerWatcherResult)
			}, 60*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

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

		It("The watchers with label selector", func() {
			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				watcherCtx,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start watching with label app=test")
			watcher, err := watcherClient.ManifestWorks(consumer.Name).Watch(watcherCtx, metav1.ListOptions{
				LabelSelector: "app=test",
			})
			Expect(err).ShouldNot(HaveOccurred())
			result := StartWatch(watcherCtx, watcher)

			By("create a work by work client")
			workName := "work-" + rand.String(5)
			work := NewManifestWorkWithLabels(workName, map[string]string{"app": "test"})
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work by work client")
			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(result)
			}, 60*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})

	Context("List works with gRPC source ManifestWork client", func() {
		var workName string
		var prodWorkName string
		var testWorkAName string
		var testWorkBName string
		var testWorkCName string

		BeforeEach(func() {
			// prepare works firstly
			workName = "work-" + rand.String(5)
			work := NewManifestWork(workName)
			_, err := workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			prodWorkName = "work-prod" + rand.String(5)
			work = NewManifestWorkWithLabels(prodWorkName, map[string]string{"app": "nginx", "env": "prod"})
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkAName = "work-test-a-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkAName, map[string]string{"app": "nginx", "env": "test", "val": "a"})
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkBName = "work-test-b-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkBName, map[string]string{"app": "nginx", "env": "test", "val": "b"})
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkCName = "work-test-c-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkCName, map[string]string{"app": "nginx", "env": "test", "val": "c"})
			_, err = workClient.ManifestWorks(consumer.Name).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := workClient.ManifestWorks(consumer.Name).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, prodWorkName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, testWorkAName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, testWorkBName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = workClient.ManifestWorks(consumer.Name).Delete(ctx, testWorkCName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := AssertWorkNotFound(workName); err != nil {
					return err
				}

				if err := AssertWorkNotFound(prodWorkName); err != nil {
					return err
				}

				if err := AssertWorkNotFound(testWorkAName); err != nil {
					return err
				}

				if err := AssertWorkNotFound(testWorkBName); err != nil {
					return err
				}

				return AssertWorkNotFound(testWorkCName)
			}, 60*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("List works with options", func() {
			By("list all works")
			works, err := workClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, workName, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())

			By("list works by consumer name")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, workName, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())

			By("list works by nonexistent consumer")
			works, err = workClient.ManifestWorks("nonexistent").List(ctx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items)).ShouldNot(HaveOccurred())

			By("list works with nonexistent labels")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
				LabelSelector: "nonexistent=true",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items)).ShouldNot(HaveOccurred())

			By("list works with app label")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
				LabelSelector: "app=nginx",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())

			By("list works without test env")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
				LabelSelector: "app=nginx,env!=test",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, prodWorkName)).ShouldNot(HaveOccurred())

			By("list works in prod and test env")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
				LabelSelector: "env in (prod, test)",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())

			By("list works in test env and val not in a and b")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
				LabelSelector: "env=test,val notin (a,b)",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, testWorkCName)).ShouldNot(HaveOccurred())

			By("list works with val label")
			works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
				LabelSelector: "val",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(AssertWorks(works.Items, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())

			// TODO support does not exist
			// By("list works without val label")
			// works, err = workClient.ManifestWorks(consumer.Name).List(ctx, metav1.ListOptions{
			// 	LabelSelector: "!val",
			// })
			// Expect(err).ShouldNot(HaveOccurred())
			// Expect(AssertWorks(works.Items, workName, prodWorkName)).ShouldNot(HaveOccurred())
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
		if strings.HasPrefix(watchedWork.Name, "init-work-a") && !watchedWork.CreationTimestamp.IsZero() {
			hasFirstInitWork = true
		}

		if strings.HasPrefix(watchedWork.Name, "init-work-b") && !watchedWork.CreationTimestamp.IsZero() {
			hasSecondInitWork = true
		}

		if strings.HasPrefix(watchedWork.Name, "work-") && !watchedWork.CreationTimestamp.IsZero() {
			hasWork = true
		}

		if meta.IsStatusConditionTrue(watchedWork.Status.Conditions, common.ManifestsDeleted) && !watchedWork.DeletionTimestamp.IsZero() {
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
	_, err := workClient.ManifestWorks(consumer.Name).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	return fmt.Errorf("the work %s still exists", name)
}

func AssertWorks(works []workv1.ManifestWork, expected ...string) error {
	workNames := sets.Set[string]{}
	expectedNames := sets.Set[string]{}.Insert(expected...)

	for _, work := range works {
		workNames.Insert(work.Name)
	}

	if len(expectedNames) != len(workNames) {
		return fmt.Errorf("expected %v, but got %v", expectedNames, workNames)
	}

	if !equality.Semantic.DeepEqual(expectedNames, workNames) {
		return fmt.Errorf("expected %v, but got %v", expectedNames, workNames)
	}

	return nil
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

func NewManifestWorkWithLabels(name string, labels map[string]string) *workv1.ManifestWork {
	work := NewManifestWork(name)
	work.Labels = labels
	return work
}

func NewManifest(name string) workv1.Manifest {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      name,
				"labels": map[string]string{
					"test": "true",
				},
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
