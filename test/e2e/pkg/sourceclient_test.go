package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bxcodec/faker/v3/support/slice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/utils/ptr"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

var _ = Describe("SourceWorkClient", Ordered, Label("e2e-tests-source-work-client"), func() {
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

			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create init work A (op-id: %s)", opID))
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			initWorkBName = "init-work-b-" + rand.String(5)
			work = NewManifestWorkWithLabels(initWorkBName, map[string]string{"app": "test"})
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create init work B (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete init work A (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, initWorkAName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete init work B (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, initWorkBName, metav1.DeleteOptions{})
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
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, NewManifestWork(workName), metav1.CreateOptions{})
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

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("patch work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(opIDCtx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the work status is updated by agent
			<-time.After(5 * time.Second)

			By("delete the work with source work client")
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete work (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
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

			By("start watching works from consumer " + agentTestOpts.consumerName)
			consumerWatcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			consumerWatcherResult := StartWatch(watcherCtx, consumerWatcher)

			By("start watching works from an other consumer")
			otherConsumerWatcher, err := watcherClient.ManifestWorks("other").Watch(watcherCtx, metav1.ListOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			otherConsumerWatcherResult := StartWatch(watcherCtx, otherConsumerWatcher)

			By("create a work with source work client")
			workName := "work-" + rand.String(5)
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, NewManifestWork(workName), metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work with source work client")
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete work (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
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
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)

			By("delete the work with source work client")
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete work (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
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
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
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
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete work (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
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

		It("should be able to watch the work when create and delete immediately", func() {
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

			watcher, err := watcherClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{
				LabelSelector: "app=test-create-delete",
			})
			Expect(err).ShouldNot(HaveOccurred())
			result := StartWatch(watcherCtx, watcher)

			By("create a work")
			workName := "work-" + rand.String(5)
			work := NewManifestWorkWithLabels(workName, map[string]string{"app": "test-create-delete"})
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			<-time.After(1 * time.Second)

			By("delete the work after 1s")
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete work (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				for _, watchedWork := range result.WatchedWorks {
					if meta.IsStatusConditionTrue(watchedWork.Status.Conditions, common.ResourceDeleted) {
						return nil
					}
				}

				return fmt.Errorf("no deleted work watched")
			}, 1*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
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
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("create work (op-id: %s)", opID))
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			prodWorkName = "work-production-" + rand.String(5)
			work = NewManifestWorkWithLabels(prodWorkName, map[string]string{"app": "test", "env": "production"})
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create prod work (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkAName = "work-integration-a-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkAName, map[string]string{"app": "test", "env": "integration", "val": "a"})
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create test work A (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkBName = "work-integration-b-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkBName, map[string]string{"app": "test", "env": "integration", "val": "b"})
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create test work B (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			testWorkCName = "work-integration-c-" + rand.String(5)
			work = NewManifestWorkWithLabels(testWorkCName, map[string]string{"app": "test", "env": "integration", "val": "c"})
			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("create test work C (op-id: %s)", opID))
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for few seconds to ensure the creation is finished
			<-time.After(5 * time.Second)
		})

		AfterEach(func() {
			opIDCtx, opID := newOpIDContext(ctx)
			By(fmt.Sprintf("delete work (op-id: %s)", opID))
			err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete prod work (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, prodWorkName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete test work A (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, testWorkAName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete test work B (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, testWorkBName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			opIDCtx, opID = newOpIDContext(ctx)
			By(fmt.Sprintf("delete test work C (op-id: %s)", opID))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, testWorkCName, metav1.DeleteOptions{})
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
				Expect(err).ShouldNot(HaveOccurred())
				var expectedWorks []workv1.ManifestWork
				for _, work := range works.Items {
					if work.DeletionTimestamp != nil {
						continue
					}
					expectedWorks = append(expectedWorks, work)
				}
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

	Context("Monitor a single workload with two source work clients", func() {
		var watchCtx context.Context
		var watchCancel context.CancelFunc

		var firstSourceID string
		var secondSourceID string

		var firstSourceWorkClient workv1client.WorkV1Interface
		var secondSourceWorkClient workv1client.WorkV1Interface

		var firstWatchedResult *WatchedResult
		var secondWatchedResult *WatchedResult

		var deployName = fmt.Sprintf("nginx-%s", rand.String(5))
		var deployUID string

		var work *workv1.ManifestWork

		BeforeEach(func() {
			firstSourceID = "sourceclient-1-test" + rand.String(5)
			secondSourceID = "sourceclient-2-test" + rand.String(5)

			By("add the gRPC auth rule", func() {
				err := helper.AddGRPCAuthRule(ctx, serverTestOpts.kubeClientSet, "grpc-pub-sub", "source", firstSourceID)
				Expect(err).To(Succeed())
				err = helper.AddGRPCAuthRule(ctx, serverTestOpts.kubeClientSet, "grpc-pub-sub", "source", secondSourceID)
				Expect(err).To(Succeed())
			})

			By("create a deployment on agent side", func() {
				deployment := &appsv1.Deployment{}
				deploymentJSON := helper.NewManifestJSON(deployName, "default", 0)
				err := json.Unmarshal([]byte(deploymentJSON), deployment)
				Expect(err).To(Succeed())
				_, err = agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
				Expect(err).To(Succeed())
				deployment, err = agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				Expect(err).To(Succeed())
				Expect(*deployment.Spec.Replicas).Should(Equal(int32(0)))
				deployUID = string(deployment.UID)
			})

			work = NewReadonlyWork(deployName)

			watchCtx, watchCancel = context.WithCancel(ctx)

			logger, err := logging.NewStdLoggerBuilder().Build()
			Expect(err).ShouldNot(HaveOccurred())

			By("create first work client", func() {
				var err error
				firstSourceWorkClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
					ctx,
					logger,
					apiClient,
					grpcOptions,
					firstSourceID,
				)
				Expect(err).ShouldNot(HaveOccurred())
				firstWatcher, err := firstSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Watch(watchCtx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				firstWatchedResult = StartWatch(watchCtx, firstWatcher)
			})

			By("create second work client", func() {
				var err error
				secondSourceWorkClient, err = grpcsource.NewMaestroGRPCSourceWorkClient(
					ctx,
					logger,
					apiClient,
					grpcOptions,
					secondSourceID,
				)
				Expect(err).ShouldNot(HaveOccurred())
				secondWatcher, err := secondSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Watch(watchCtx, metav1.ListOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				secondWatchedResult = StartWatch(watchCtx, secondWatcher)
			})
		})

		AfterEach(func() {
			// Attempt to delete from both clients, ignoring NotFound errors
			err := firstSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, work.Name, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				Expect(err).ShouldNot(HaveOccurred())
			}

			err = secondSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, work.Name, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				Expect(err).ShouldNot(HaveOccurred())
			}

			err = agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Delete(ctx, deployName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				Expect(err).ShouldNot(HaveOccurred())
			}

			watchCancel()
		})

		It("Should have two independent manifestworks applied", func() {
			var firstWorkUID string
			var secondWorkUID string

			By("create two bundles for a single workload using manifestwork", func() {
				opIDCtx, opID := newOpIDContext(ctx)
				By(fmt.Sprintf("create work with first source client (op-id: %s)", opID))
				firstCreated, err := firstSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				opIDCtx, opID = newOpIDContext(ctx)
				By(fmt.Sprintf("create work with second source client (op-id: %s)", opID))
				secondCreated, err := secondSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(opIDCtx, work, metav1.CreateOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				firstWorkUID = string(firstCreated.UID)
				secondWorkUID = string(secondCreated.UID)

				Expect(firstWorkUID).ShouldNot(Equal(secondWorkUID))
			})

			By("check the appliedmanifestworks", func() {
				Eventually(func() error {
					// there are two appliedmanifestworks
					appliedWorks, err := agentTestOpts.workClientSet.WorkV1().AppliedManifestWorks().List(ctx, metav1.ListOptions{
						LabelSelector: "maestro.e2e.test.name=monitor",
					})
					if err != nil {
						return err
					}

					if len(appliedWorks.Items) != 2 {
						return fmt.Errorf("unexpected applied works %d", len(appliedWorks.Items))
					}

					appliedWorkNames := []string{}
					for _, appliedWork := range appliedWorks.Items {
						parts := strings.SplitN(appliedWork.Name, "-", 2)
						if len(parts) != 2 {
							return fmt.Errorf("unexpected applied work name: %s", appliedWork.Name)
						}

						appliedWorkNames = append(appliedWorkNames, parts[1])
					}

					if !slice.Contains(appliedWorkNames, firstWorkUID) {
						return fmt.Errorf("the first work %s is not found in %v", firstWorkUID, appliedWorkNames)
					}

					if !slice.Contains(appliedWorkNames, secondWorkUID) {
						return fmt.Errorf("the second work %s is not found in %v", secondWorkUID, appliedWorkNames)
					}

					return nil
				}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			})

			By("update deploy replicas", func() {
				deployment, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				updatedDeploy := deployment.DeepCopy()
				updatedDeploy.Spec.Replicas = ptr.To(int32(1))
				_, err = agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Update(ctx, updatedDeploy, metav1.UpdateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			})

			By("check the bundle status", func() {
				// the status should be synced
				Eventually(func() error {
					if err := AssertReplicas(firstWatchedResult.WatchedWorks, work.Name, int32(1)); err != nil {
						return fmt.Errorf("failed to check in first watcher: %v", err)
					}

					if err := AssertReplicas(secondWatchedResult.WatchedWorks, work.Name, int32(1)); err != nil {
						return fmt.Errorf("failed to check in second watcher: %v", err)
					}

					return nil
				}, 5*time.Minute, 10*time.Second).ShouldNot(HaveOccurred())
			})

			By("delete the first work", func() {
				Eventually(func() error {
					// delete one, the other should be not be changed
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("delete work with first source client (op-id: %s)", opID))
					if err := firstSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(opIDCtx, work.Name, metav1.DeleteOptions{}); err != nil {
						return err
					}

					if _, err := secondSourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, work.Name, metav1.GetOptions{}); err != nil {
						return err
					}

					appliedWorks, err := agentTestOpts.workClientSet.WorkV1().AppliedManifestWorks().List(ctx, metav1.ListOptions{
						LabelSelector: "maestro.e2e.test.name=monitor",
					})
					if err != nil {
						return err
					}

					if len(appliedWorks.Items) != 1 {
						return fmt.Errorf("unexpected applied works %d", len(appliedWorks.Items))
					}

					if !strings.Contains(appliedWorks.Items[0].Name, secondWorkUID) {
						return fmt.Errorf("applied work is recreated")
					}

					// deploy should not be recreated
					deploy, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
					if err != nil {
						return err
					}

					if deployUID != string(deploy.UID) {
						return fmt.Errorf("deploy is recreated")
					}

					return nil
				}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			})
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

func AssertReplicas(watchedWorks []*workv1.ManifestWork, name string, replicas int32) error {
	var latestWatchedWork *workv1.ManifestWork
	for i := len(watchedWorks) - 1; i >= 0; i-- {
		if watchedWorks[i].Name == name {
			latestWatchedWork = watchedWorks[i]
			break
		}
	}

	if latestWatchedWork == nil {
		return fmt.Errorf("the work %s not watched", name)
	}

	for _, manifest := range latestWatchedWork.Status.ResourceStatus.Manifests {
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

func NewReadonlyWork(deployName string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("work-%s", rand.String(5)),
			Labels: map[string]string{
				"maestro.e2e.test.name": "monitor",
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Object: &appsv1.Deployment{
								TypeMeta: metav1.TypeMeta{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
								},
								ObjectMeta: metav1.ObjectMeta{
									Name:      deployName,
									Namespace: "default",
								},
							},
						},
					},
				},
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     "apps",
						Resource:  "deployments",
						Name:      deployName,
						Namespace: "default",
					},
					FeedbackRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: "resource",
									Path: "@",
								},
							},
						},
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: workv1.UpdateStrategyTypeReadOnly,
					},
				},
			},
		},
	}
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
