package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/component-base/metrics/testutil"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
)

var _ = Describe("SourceWorkClient", Ordered, Label("e2e-tests-source-work-client"), func() {
	Context("Update an obsolete work", func() {
		var workName string

		BeforeEach(func() {
			workName = "work-" + rand.String(5)
			work := NewManifestWork(workName)
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)
		})

		AfterEach(func() {
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

		})

		It("should return error when updating an obsolete work", func() {
			By("update a work by work client")
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("update the work by work client again")
			obsoleteWork := work.DeepCopy()
			obsoleteWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err = grpcsource.ToWorkPatch(work, obsoleteWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).Should(HaveOccurred())
			Expect(strings.Contains(err.Error(), "the resource version is not the latest")).Should(BeTrue())

			// wait for few seconds to ensure the update is finished
			<-time.After(5 * time.Second)
		})
	})

	Context("Watch work status with source work client", func() {
		var watcherCtx context.Context
		var watcherCancel context.CancelFunc

		var initWorkAName string
		var initWorkBName string

		BeforeEach(func() {
			// reset the metrics firstly
			grpcsource.ResetsourceClientRegisteredWatchersGaugeMetric()

			watcherCtx, watcherCancel = context.WithCancel(ctx)

			// prepare two works firstly
			initWorkAName = "init-work-a-" + rand.String(5)
			work := NewManifestWorkWithLabels(initWorkAName, map[string]string{"app": "test"})

			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			initWorkBName = "init-work-b-" + rand.String(5)
			work = NewManifestWorkWithLabels(initWorkBName, map[string]string{"app": "test"})
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, initWorkAName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, initWorkBName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := AssertWorkNotFound(initWorkAName); err != nil {
					return err
				}

				return AssertWorkNotFound(initWorkBName)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			watcherCancel()
		})

		It("the work status should be watched", func() {
			By("create a work watcher client")
			logger, err := logging.NewStdLoggerBuilder().Build()
			Expect(err).ShouldNot(HaveOccurred())

			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				ctx,
				logger,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start status watching")
			watcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			result := StartWatch(watcherCtx, watcher)

			By("create a work with source work client")
			workName := "work-" + rand.String(5)
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("update a work with source work client")
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the work status is updated by agent
			<-time.After(5 * time.Second)

			By("delete the work with source work client")
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(result)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			expectedMetrics := fmt.Sprintf(`
			# HELP source_client_registered_watchers Number of registered watchers for a source client.
			# TYPE source_client_registered_watchers gauge
			source_client_registered_watchers{namespace="%s",source="%s"} 1
			`, agentTestOpts.consumerName, sourceID)
			err = testutil.GatherAndCompare(prometheus.DefaultGatherer,
				strings.NewReader(expectedMetrics), "source_client_registered_watchers")
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("the watchers for different namespace", func() {
			logger, err := logging.NewStdLoggerBuilder().Build()
			Expect(err).ShouldNot(HaveOccurred())

			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				ctx,
				logger,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start watching works from all consumers")
			allConsumerWatcher, err := watcherClient.ManifestWorks(metav1.NamespaceAll).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			allConsumerWatcherResult := StartWatch(watcherCtx, allConsumerWatcher)

			By("start watching works from consumer" + agentTestOpts.consumerName)
			consumerWatcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			consumerWatcherResult := StartWatch(watcherCtx, consumerWatcher)

			By("start watching works from an other consumer")
			otherConsumerWatcher, err := watcherClient.ManifestWorks("other").Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			otherConsumerWatcherResult := StartWatch(watcherCtx, otherConsumerWatcher)

			By("create a work with source work client")
			workName := "work-" + rand.String(5)
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work with source work client")
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(allConsumerWatcherResult)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(consumerWatcherResult)
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			Consistently(func() error {
				if len(otherConsumerWatcherResult.WatchedWorks) != 0 {
					return fmt.Errorf("unexpected watched works")
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			expectedMetrics := fmt.Sprintf(`
			# HELP source_client_registered_watchers Number of registered watchers for a source client.
			# TYPE source_client_registered_watchers gauge
			source_client_registered_watchers{namespace="",source="%s"} 1
			source_client_registered_watchers{namespace="%s",source="%s"} 1
			source_client_registered_watchers{namespace="other",source="%s"} 1
			`, sourceID, agentTestOpts.consumerName, sourceID, sourceID)
			err = testutil.GatherAndCompare(prometheus.DefaultGatherer,
				strings.NewReader(expectedMetrics), "source_client_registered_watchers")
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("the watchers with label selector", func() {
			logger, err := logging.NewStdLoggerBuilder().Build()
			Expect(err).ShouldNot(HaveOccurred())

			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				ctx,
				logger,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("start watching with label")
			watcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{
				LabelSelector: "app=test",
			})
			Expect(err).ShouldNot(HaveOccurred())
			result := StartWatch(watcherCtx, watcher)

			By("create a work with source work client")
			workName := "work-" + rand.String(5)
			work := NewManifestWorkWithLabels(workName, map[string]string{"app": "test"})
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work with source work client")
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				return AssertWatchResult(result)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			expectedMetrics := fmt.Sprintf(`
			# HELP source_client_registered_watchers Number of registered watchers for a source client.
			# TYPE source_client_registered_watchers gauge
			source_client_registered_watchers{namespace="%s",source="%s"} 1
			`, agentTestOpts.consumerName, sourceID)
			err = testutil.GatherAndCompare(prometheus.DefaultGatherer,
				strings.NewReader(expectedMetrics), "source_client_registered_watchers")
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should recreate stopped watchers", func() {
			logger, err := logging.NewStdLoggerBuilder().Build()
			Expect(err).ShouldNot(HaveOccurred())

			watcherClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
				ctx,
				logger,
				apiClient,
				grpcOptions,
				sourceID,
			)
			Expect(err).ShouldNot(HaveOccurred())

			By("create first watcher for namespace")
			firstWatcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			firstResult := StartWatch(watcherCtx, firstWatcher)

			By("create and process a work with first watcher")
			workName := "watcher-test-work-" + rand.String(5)
			work := NewManifestWork(workName)
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for work to be processed
			<-time.After(3 * time.Second)

			expectedMetrics := fmt.Sprintf(`
			# HELP source_client_registered_watchers Number of registered watchers for a source client.
			# TYPE source_client_registered_watchers gauge
			source_client_registered_watchers{namespace="%s",source="%s"} 1
			`, agentTestOpts.consumerName, sourceID)
			err = testutil.GatherAndCompare(prometheus.DefaultGatherer,
				strings.NewReader(expectedMetrics), "source_client_registered_watchers")
			Expect(err).ShouldNot(HaveOccurred())

			By("stop the first watcher")
			firstWatcher.Stop()

			// wait for watcher to be fully stopped
			<-time.After(2 * time.Second)

			By("create second watcher for same namespace after first watcher stopped")
			secondWatcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			secondResult := StartWatch(watcherCtx, secondWatcher)

			By("verify second watcher can process events independently")
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("verify second watcher received events")
			Eventually(func() error {
				if len(secondResult.WatchedWorks) == 0 {
					return fmt.Errorf("second watcher should have received events")
				}
				return nil
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			By("verify watchers are independent - first watcher stopped, second continues")
			Expect(len(firstResult.WatchedWorks)).Should(BeNumerically(">", 0), "first watcher should have processed initial events")
			Expect(len(secondResult.WatchedWorks)).Should(BeNumerically(">", 0), "second watcher should have processed events after restart")

			secondWatcher.Stop()

			expectedMetrics = fmt.Sprintf(`
			# HELP source_client_registered_watchers Number of registered watchers for a source client.
			# TYPE source_client_registered_watchers gauge
			source_client_registered_watchers{namespace="%s",source="%s"} 0
			`, agentTestOpts.consumerName, sourceID)
			err = testutil.GatherAndCompare(prometheus.DefaultGatherer,
				strings.NewReader(expectedMetrics), "source_client_registered_watchers")
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("List works with source work client", func() {
		var workName string
		var prodWorkName string
		var testWorkAName string
		var testWorkBName string
		var testWorkCName string

		BeforeEach(func() {
			// prepare works firstly
			workName = "work-" + rand.String(5)
			work := NewManifestWork(workName)
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			prodWorkName = "work-production" + rand.String(5)
			work = NewManifestWorkWithLabels(prodWorkName, map[string]string{"app": "test", "env": "production"})
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkAName = "work-integration-a-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkAName, map[string]string{"app": "test", "env": "integration", "val": "a"})
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkBName = "work-integration-b-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkBName, map[string]string{"app": "test", "env": "integration", "val": "b"})
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkCName = "work-integration-c-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkCName, map[string]string{"app": "test", "env": "integration", "val": "c"})
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)
		})

		AfterEach(func() {
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, prodWorkName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, testWorkAName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, testWorkBName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, testWorkCName, metav1.DeleteOptions{})
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
			}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		})

		It("list works with options", func() {
			By("list all works", func() {
				works, err := sourceWorkClient.ManifestWorks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(err).ShouldNot(HaveOccurred())
				Expect(AssertWorks(expectedWorks, workName, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())
			})

			By("list works by consumer name", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(err).ShouldNot(HaveOccurred())
				Expect(AssertWorks(expectedWorks, workName, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())
			})

			By("list works by nonexistent consumer", func() {
				works, err := sourceWorkClient.ManifestWorks("nonexistent").List(ctx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks)).ShouldNot(HaveOccurred())

			})

			By("list works with nonexistent labels", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
					LabelSelector: "nonexistent=true",
				})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks)).ShouldNot(HaveOccurred())
			})

			By("list works with app label", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
					LabelSelector: "app=test",
				})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())
			})

			By("list works without test env", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
					LabelSelector: "app=test,env!=integration",
				})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks, prodWorkName)).ShouldNot(HaveOccurred())
			})

			By("list works in prod and test env", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
					LabelSelector: "env in (production, integration)",
				})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks, prodWorkName, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())
			})

			By("list works in test env and val not in a and b", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
					LabelSelector: "env=integration,val notin (a,b)",
				})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks, testWorkCName)).ShouldNot(HaveOccurred())
			})

			By("list works with val label", func() {
				works, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
					LabelSelector: "val",
				})
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
				Expect(AssertWorks(expectedWorks, testWorkAName, testWorkBName, testWorkCName)).ShouldNot(HaveOccurred())
			})

			// TODO support does not exist
			// By("list works without val label")
			// works, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{
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
			if err := ensureObservedGeneration(watchedWork); err != nil {
				return err
			}
		}

		if strings.HasPrefix(watchedWork.Name, "init-work-b") && !watchedWork.CreationTimestamp.IsZero() {
			hasSecondInitWork = true
			if err := ensureObservedGeneration(watchedWork); err != nil {
				return err
			}
		}

		if strings.HasPrefix(watchedWork.Name, "work-") && !watchedWork.CreationTimestamp.IsZero() {
			hasWork = true
			if err := ensureObservedGeneration(watchedWork); err != nil {
				return err
			}
		}

		if meta.IsStatusConditionTrue(watchedWork.Status.Conditions, common.ResourceDeleted) && !watchedWork.DeletionTimestamp.IsZero() {
			if len(watchedWork.Spec.Workload.Manifests) == 0 {
				return fmt.Errorf("expected the deleted work has spec, but failed %v", watchedWork.Spec)
			}

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
	_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, name, metav1.GetOptions{})
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

func NewManifestWorkWithLabels(name string, labels map[string]string) *workv1.ManifestWork {
	work := NewManifestWork(name)
	work.Labels = labels
	return work
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

func ensureObservedGeneration(work *workv1.ManifestWork) error {
	if meta.IsStatusConditionTrue(work.Status.Conditions, common.ResourceDeleted) {
		return nil
	}

	for _, cond := range work.Status.Conditions {
		if cond.ObservedGeneration == 0 {
			return fmt.Errorf("unexpected observed generation %d for work %s",
				cond.ObservedGeneration, work.Name)
		}
	}

	return nil
}
