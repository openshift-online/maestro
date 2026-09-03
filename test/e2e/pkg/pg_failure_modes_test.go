package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

const (
	resultWaitDuration = 30 * time.Second
)

// These tests assert maestro recovers from certain postgres related failure modes gracefully.
// The pgfailure tool is deployed as a pod to execute commands against postgres to induce failures,
// and to lock maestro at certain points in its execution flow so the failures will have the intended
// effect.
//
// Each scenario uses the following workflow to reproduce the failure mode under test:
// 1. Setup (e.g. create resources, scale the agent to "turn off" status updates)
// 2. Deploy "failure" pgfailure pod (e.g. lock the events table, deploy a failure trigger)
// 3. Service commands (e.g. create/update/delete a resource)
// 4. Deploy "recovery" pgfailure pod (e.g. terminate backends, unlock table, remove failure trigger)
// 5. Assert the state of the resource under test, including watched status events
//
// Both spec event and status event flows are tested, each requiring their own combination of pgfailure
// commands to induce the failure mode.
//
// The intention is for these tests to be as deterministic as possible.
var _ = Describe("Postgres failure modes", Ordered, ContinueOnFailure, Label("e2e-tests-pg-failure-modes"), func() {

	BeforeAll(func() {
		// shrinking the max_notify_queue_pages in postgres requires a pod restart, we do it once for all
		// tests to reduce instability
		shrinkNotifyQueue()
	})

	AfterAll(func() {
		resetNotifyQueue()
	})

	// These tests add a trigger that fails any write operations to the events table, simulating a database failure
	// "mid-transaction" in the spec event create/update/delete workflows
	var _ = Describe("Spec receive event insertion failure modes", Ordered, Label("e2e-tests-spec-receive-event-insert-failure-modes"), func() {
		workNamePrefix := "eventinsertfail"

		AfterAll(func() {
			cleanupFailurePods()
			cleanupResources(ctx)
		})

		run := func(setupOps func(), serviceOps func(), checkResults func() error) {
			runScenario(pgfailureCommandArgs("block events"), pgfailureCommandArgs("unblock events"), 1, setupOps, serviceOps, checkResults, false)
		}

		It("events insertion failure on resource create", func() {
			workName := fmt.Sprintf("%s-create-%s", workNamePrefix, rand.String(5))

			run(
				nil,
				func() {
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, NewManifestWork(workName), metav1.CreateOptions{})
					Expect(err).To(MatchError(ContainSubstring("pgfailure: writes blocked on events")))
				},
				func() error {
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return nil
					}
					if err != nil {
						return err
					}
					return fmt.Errorf("resource %s should not exist after failed create", workName)
				})
		})

		It("events insertion failure on resource update", func() {
			workName := fmt.Sprintf("%s-update-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			Expect(watchResult).ShouldNot(BeNil())
			defer watcherCancel()

			run(
				func() {
					work := NewManifestWork(workName)
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, work, metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())
					By(fmt.Sprintf("resource %s:%s created", workName, work.UID))

					By("waiting for resource status")
					Eventually(func() error {
						return assertWorkStatus(watchResult, workName, 1)
					}, 1*time.Minute, 2*time.Second).Should(Succeed())
				},
				func() {
					_, err := updateResource(workName)
					Expect(err).To(MatchError(ContainSubstring("pgfailure: writes blocked on events")))
				},
				func() error {
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if len(work.Spec.Workload.Manifests) == 0 {
						return fmt.Errorf("resource %s has no manifests", workName)
					}
					name, err := manifestName(work.Spec.Workload.Manifests[0])
					if err != nil {
						return err
					}
					if name != workName {
						return fmt.Errorf("resource was updated: manifest name=%s, expected %s", name, workName)
					}
					return nil
				},
			)
		})

		It("events insertion failure on resource delete", func() {
			workName := fmt.Sprintf("%s-delete-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()

			run(
				func() {
					work := NewManifestWork(workName)
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, work, metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())
					By(fmt.Sprintf("resource %s:%s created", workName, work.UID))

					By("waiting for resource status")
					Eventually(func() error {
						return assertWorkStatus(watchResult, workName, 1)
					}, 1*time.Minute, 2*time.Second).Should(Succeed())
				},
				func() {
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("deleting resource %s (op-id: %s)", workName, opID))
					err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(
						opIDCtx, workName, metav1.DeleteOptions{})
					Expect(err).To(MatchError(ContainSubstring("pgfailure: writes blocked on events")))
				},
				func() error {
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("resource %s should still exist: %v", workName, err)
					}
					if work.DeletionTimestamp != nil {
						return fmt.Errorf("resource %s should not have deletion timestamp", workName)
					}
					return nil
				},
			)
		})
	})

	// These tests simulate a "dropped" NOTIFY message for the events queue by locking the events table,
	// performing the create/update/delete operation (thus blocking maestro at thot point), and then
	// terminating listeners before immediately unlocking the events table.
	// NOTE: the listener will sometimes reconnect before the pg_notify call, so tihs test can produce
	// false positives.
	var _ = Describe("Spec receive NOTIFY drop failure modes", Ordered, Label("e2e-tests-spec-receive-notify-drop-modes"), func() {
		workNamePrefix := "notifydrop"

		AfterAll(func() {
			cleanupFailurePods()
			cleanupResources(ctx)
		})

		run := func(setupOps func(), serviceOps func(), checkResults func() error) {
			runScenario(pgfailureCommandArgs("lock events"), pgfailureCommandArgs("terminate listeners events", "unlock events"), 1, setupOps, serviceOps, checkResults, true)
		}

		It("events NOTIFY drop on resource create", func() {
			workName := fmt.Sprintf("%s-create-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()

			run(
				nil,
				func() {
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, NewManifestWork(workName), metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())
					By(fmt.Sprintf("resource %s:%s created", workName, work.UID))
				},
				func() error {
					return assertWorkStatus(watchResult, workName, 1)
				})
		})

		It("events NOTIFY drop on resource update", func() {
			workName := fmt.Sprintf("%s-update-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()

			run(
				func() {
					work := NewManifestWork(workName)
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, work, metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())
					By(fmt.Sprintf("resource %s:%s created", workName, work.UID))

					By("waiting for resource status")
					Eventually(func() error {
						return assertWorkStatus(watchResult, workName, 1)
					}, 1*time.Minute, 2*time.Second).Should(Succeed())
				},
				func() {
					_, err := updateResource(workName)
					Expect(err).ShouldNot(HaveOccurred())
				},
				func() error {
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if len(work.Spec.Workload.Manifests) == 0 {
						return fmt.Errorf("resource %s has no manifests", workName)
					}
					name, err := manifestName(work.Spec.Workload.Manifests[0])
					if err != nil {
						return err
					}
					if name == workName {
						return fmt.Errorf("resource was not updated")
					}
					return assertWorkStatus(watchResult, workName, 2)
				})
		})

		It("events NOTIFY drop on resource delete", func() {
			workName := fmt.Sprintf("%s-delete-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()

			run(
				func() {
					work := NewManifestWork(workName)
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, work, metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())

					By(fmt.Sprintf("waiting for status of resource %s", work.UID))
					Eventually(func() error {
						return assertWorkStatus(watchResult, workName, 1)
					}, 1*time.Minute, 2*time.Second).Should(Succeed())
				},
				func() {
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("deleting resource %s (op-id: %s)", workName, opID))
					err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(
						opIDCtx, workName, metav1.DeleteOptions{})
					Expect(err).ShouldNot(HaveOccurred())
				},
				func() error {
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if err == nil || !errors.IsNotFound(err) {
						return fmt.Errorf("resource %s should not exist: %v", workName, err)
					}
					return nil
				},
			)
		})
	})

	// These tests simulate the unlikely, but possible, scenario where the NOTIFY queue is full in the spec event flows.
	var _ = Describe("Spec receive NOTIFY queue full failure modes", Ordered, Label("e2e-tests-spec-receive-notify-queue-failure-modes"), func() {
		workNamePrefix := "notifyfill"

		AfterAll(func() {
			cleanupFailurePods()
			cleanupResources(ctx)
		})

		run := func(setupOps func(), serviceOps func(), checkResults func() error) {
			runScenario(pgfailureCommandArgs("notify-queue fill"), pgfailureCommandArgs("notify-queue drain"), 1, setupOps, serviceOps, checkResults, false)
		}

		It("pg_notify queue full on resource create", func() {
			workName := fmt.Sprintf("%s-create-%s", workNamePrefix, rand.String(5))

			run(
				nil,
				func() {
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, NewManifestWork(workName), metav1.CreateOptions{})
					Expect(err).To(MatchError(ContainSubstring("too many notifications in the NOTIFY queue")))
				},
				func() error {
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return nil
					}
					if err != nil {
						return err
					}
					return fmt.Errorf("resource %s should not exist after failed create", workName)
				},
			)
		})

		It("pg_notify queue full on resource update", func() {
			workName := fmt.Sprintf("%s-update-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()

			run(
				func() {
					work := NewManifestWork(workName)
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, work, metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())

					By("waiting for resource status")
					Eventually(func() error {
						return assertWorkStatus(watchResult, workName, 1)
					}, 1*time.Minute, 2*time.Second).Should(Succeed())
				},
				func() {
					_, err := updateResource(workName)
					Expect(err).To(MatchError(ContainSubstring("too many notifications in the NOTIFY queue")))
				},
				func() error {
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if len(work.Spec.Workload.Manifests) == 0 {
						return fmt.Errorf("resource %s has no manifests", workName)
					}
					name, err := manifestName(work.Spec.Workload.Manifests[0])
					if err != nil {
						return err
					}
					if name != workName {
						return fmt.Errorf("resource was updated: manifest name=%s, expected %s", name, workName)
					}
					return nil
				})
		})

		It("events NOTIFY queue full on resource delete", func() {
			workName := fmt.Sprintf("%s-delete-%s", workNamePrefix, rand.String(5))
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()

			run(
				func() {
					work := NewManifestWork(workName)
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, work, metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())

					By("waiting for resource status")
					Eventually(func() error {
						return assertWorkStatus(watchResult, workName, 1)
					}, 1*time.Minute, 2*time.Second).Should(Succeed())
				},
				func() {
					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("deleting resource %s (op-id: %s)", workName, opID))
					err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(
						opIDCtx, workName, metav1.DeleteOptions{})
					Expect(err).To(MatchError(ContainSubstring("too many notifications in the NOTIFY queue")))
				},
				func() error {
					work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
						ctx, workName, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("resource %s should still exist: %v", workName, err)
					}
					if work.DeletionTimestamp != nil {
						return fmt.Errorf("resource %s should not have deletion timestamp", workName)
					}
					return nil
				},
			)
		})
	})

	// These tests focus on the status event flows.
	// All use the same setup and verification but use different pgfailure commands.
	//
	// The high level flow is:
	// 1. Scale the maestro agent to 0 (thus disabling status event delivery)
	// 2. Create a resource for which we will watch status events
	// 3. Deploy the "failure" pgfailure pod
	// 4. Scale the maestro agent to 1 (thus enabling status event delivery)
	// 5. Deploy the "recovery" pgfailure pod
	// 6. Assert the current and watched status of the resource
	var _ = Describe("Status receive failure modes", Ordered, Label("e2e-tests-status-receive-failure-modes"), func() {

		AfterEach(func() {
			scaleDeployment("maestro-agent", agentTestOpts.agentNamespace, 1)
			cleanupFailurePods()
			cleanupResources(ctx)
		})

		run := func(workName string, failureArgs []string, recoveryArgs []string) {
			watchResult, watcherCancel := watchStatus()
			defer watcherCancel()
			runScenario(failureArgs, recoveryArgs, 5,
				func() {
					By("scaling agent to 0 to prevent delivery of status events")
					scaleDeployment("maestro-agent", agentTestOpts.agentNamespace, 0)

					opIDCtx, opID := newOpIDContext(ctx)
					By(fmt.Sprintf("creating resource %s (op-id: %s)", workName, opID))
					_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(
						opIDCtx, NewManifestWork(workName), metav1.CreateOptions{})
					Expect(err).ShouldNot(HaveOccurred())
				}, func() {
					By("scaling agent back to 1 to re-enable delivery of status events")
					scaleDeployment("maestro-agent", agentTestOpts.agentNamespace, 1)
				}, func() error {
					return assertWorkStatus(watchResult, workName, 1)
				}, true)
		}

		It("resource status update failure", func() {
			workName := fmt.Sprintf("status-resource-update-%s", rand.String(5))
			run(workName, pgfailureCommandArgs("block resources"), pgfailureCommandArgs("unblock resources"))
		})

		It("status_events insertion failure", func() {
			workName := fmt.Sprintf("status-event-insert-%s", rand.String(5))
			run(workName, pgfailureCommandArgs("block status_events"), pgfailureCommandArgs("unblock status_events"))
		})

		// TODO: This test will often succeed, but it is a false positive, since there is no mitigation
		// in the status event processing flow for this failure mode.  Once a mitigation is applied this
		// test should be re-enabled.
		// It("status_events NOTIFY drop", func() {
		// 		workName := fmt.Sprintf("status-notify-drop-%s", rand.String(5))
		// 		run(workName, pgfailureCommandArgs("lock status_events"), pgfailureCommandArgs("terminate listeners status_events", "unlock status_events"))
		// })

		It("status_events NOTIFY queue full", func() {
			workName := fmt.Sprintf("status-queue-fill-%s", rand.String(5))
			run(workName, pgfailureCommandArgs("notify-queue fill"), pgfailureCommandArgs("notify-queue drain"))
		})
	})
})

func runScenario(failureArgs, recoveryArgs []string, recoveryWaitSeconds int, setupOps func(), serviceOps func(), checkResults func() error, waitForResults bool) {
	if setupOps != nil {
		setupOps()
	}

	By(fmt.Sprintf("creating failure pod with args %v", failureArgs))
	failurePod := createPGFailurePod("fail", failureArgs)

	By(fmt.Sprintf("waiting for failure pod %s to be ready", failurePod))
	Eventually(func() error {
		return checkPodReady(failurePod)
	}, 1*time.Minute, 2*time.Second).Should(Succeed())

	// the failurePod may cause serviceOps to block, so it must be run asynchronously, and the scenario
	// may require a sleep interval to ensure that serviceOps starts before creating the recoveryPod
	By("calling serviceOps (async)")
	done := make(chan struct{})
	go func() {
		defer GinkgoRecover()
		defer close(done)
		serviceOps()
	}()

	time.Sleep(time.Duration(recoveryWaitSeconds) * time.Second)

	By(fmt.Sprintf("running recovery command with args %v", recoveryArgs))
	recoveryPod := createPGFailurePod("recover", recoveryArgs)
	Eventually(func() error {
		return checkPodReady(recoveryPod)
	}, 1*time.Minute, 2*time.Second).Should(Succeed())

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		Fail("service_ops did not complete within 30 seconds after recovery")
	}

	if waitForResults {
		By("waiting for results")
		Eventually(checkResults, resultWaitDuration, 2*time.Second).Should(Succeed())
	} else {
		By("checking results")
		Expect(checkResults()).ShouldNot(HaveOccurred())
	}

}

func cleanupFailurePods() {
	By("cleaning up pgfailure pods")
	pods, err := serverTestOpts.kubeClientSet.CoreV1().Pods(serverTestOpts.serverNamespace).List(
		ctx, metav1.ListOptions{LabelSelector: "app=pgfailure"})
	if err == nil {
		for _, p := range pods.Items {
			_ = serverTestOpts.kubeClientSet.CoreV1().Pods(serverTestOpts.serverNamespace).Delete(
				ctx, p.Name, metav1.DeleteOptions{})
		}
	}
}

func shrinkNotifyQueue() {
	By("setting max_notify_queue_pages to 64 via pgfailure pod")
	podName := createPGFailurePod("set-nq-size", pgfailureCommandArgs("notify-queue set-max-size"))
	Eventually(func() error {
		return checkPodReady(podName)
	}).ShouldNot(HaveOccurred())

	By("restarting maestro-db so the new queue size takes effect")
	Expect(restartAndWaitForOldPodGone(ctx, "maestro-db", serverTestOpts.serverNamespace)).Should(Succeed())
	By("waiting for maestro gRPC to be ready")
	Eventually(func() error {
		_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{})
		return err
	}, 60*time.Second, 2*time.Second).Should(Succeed())

	// give time for NOTIFY listeners to reconnect
	time.Sleep(10 * time.Second)
}

func resetNotifyQueue() {
	By("resetting max_notify_queue_pages to default via pgfailure pod")
	podName := createPGFailurePod("reset-nq-size", pgfailureCommandArgs("notify-queue reset-max-size"))
	Eventually(func() error {
		return checkPodReady(podName)
	}).ShouldNot(HaveOccurred())

	By("restarting maestro-db so the new queue size takes effect")
	Expect(restartAndWaitForOldPodGone(ctx, "maestro-db", serverTestOpts.serverNamespace)).Should(Succeed())
	By("waiting for maestro gRPC to be ready")
	Eventually(func() error {
		_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).List(ctx, metav1.ListOptions{})
		return err
	}, 60*time.Second, 2*time.Second).Should(Succeed())

	// give time for NOTIFY listeners to reconnect
	time.Sleep(10 * time.Second)
}

func restartAndWaitForOldPodGone(ctx context.Context, deploymentName, namespace string) error {
	pods, err := serverTestOpts.kubeClientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s", deploymentName),
	})
	if err != nil {
		return err
	}
	oldPodNames := map[string]bool{}
	for _, p := range pods.Items {
		oldPodNames[p.Name] = true
	}
	if err := restartDeployment(ctx, serverTestOpts.kubeClientSet, deploymentName, namespace); err != nil {
		return err
	}
	By(fmt.Sprintf("waiting for old pods to be gone:  %v", oldPodNames))
	Eventually(func() error {
		for name := range oldPodNames {
			_, err := serverTestOpts.kubeClientSet.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
			if err == nil {
				return fmt.Errorf("old pod %s still exists", name)
			}
			if !errors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}, 2*time.Minute, 2*time.Second).Should(Succeed())
	return nil
}

func updateResource(workName string) (*workv1.ManifestWork, error) {
	work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
		ctx, workName, metav1.GetOptions{})
	Expect(err).ShouldNot(HaveOccurred())
	newWork := work.DeepCopy()
	newWork.Spec.Workload.Manifests = []workv1.Manifest{
		NewManifest(workName + "-updated"),
	}
	patchData, err := grpcsource.ToWorkPatch(work, newWork)
	Expect(err).ShouldNot(HaveOccurred())
	opIDCtx, opID := newOpIDContext(ctx)
	By(fmt.Sprintf("patching resource %s (op-id: %s)", workName, opID))
	work, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(
		opIDCtx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
	return work, err
}

func watchStatus() (*WatchedResult, context.CancelFunc) {
	watcherCtx, watcherOpID := newOpIDContext(ctx)
	By(fmt.Sprintf("watching for status updates (op-id: %s)", watcherOpID))
	watcher, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Watch(watcherCtx, metav1.ListOptions{})
	Expect(err).ShouldNot(HaveOccurred())
	cancel := func() {
		watcher.Stop()
	}
	return StartWatch(watcherCtx, watcher), cancel
}

func assertWorkStatus(watchResult *WatchedResult, workName string, expectedGeneration int64) error {
	check := func(work *workv1.ManifestWork) error {
		if len(work.Status.Conditions) == 0 {
			return fmt.Errorf("work %s has no status conditions yet", work.Name)
		}
		applied := false
		available := false
		for _, c := range work.Status.Conditions {
			if c.ObservedGeneration == work.Generation {
				if c.Type == "Applied" {
					applied = c.Status == "True"
				}
				if c.Type == "Available" {
					available = c.Status == "True"
				}
			}
		}

		if !applied {
			return fmt.Errorf("work %s for generation %d has not been applied.", workName, work.Generation)
		}
		if !available {
			return fmt.Errorf("work %s for generation %d is not available.", workName, work.Generation)
		}

		return nil
	}

	work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
		ctx, workName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if work.Generation != int64(expectedGeneration) {
		return fmt.Errorf("work %s is not the expectedGeneration %d, actual generation: %d", workName, expectedGeneration, work.Generation)
	}
	err = check(work)
	if err != nil {
		return fmt.Errorf("resource does not have expected status: %s", err)
	}
	for _, w := range watchResult.WatchedWorks {
		if w.Name == workName {
			for _, c := range w.Status.Conditions {
				if c.ObservedGeneration == work.Generation {
					err = check(w)
					if err == nil {
						return nil
					}
				}
			}
		}
	}
	if err != nil {
		return fmt.Errorf("watcher received status for work %s and generation %d, but resource was not available: %s", workName, work.Generation, err)
	}
	return fmt.Errorf("watcher did not receive status for work %s and generation %d", workName, work.Generation)
}

func pgfailureCommandArgs(commands ...string) []string {
	var args []string
	for _, cmd := range commands {
		args = append(args, "-c", cmd)
	}
	return args
}

func manifestName(m workv1.Manifest) (string, error) {
	var obj unstructured.Unstructured
	if err := json.Unmarshal(m.Raw, &obj); err != nil {
		return "", fmt.Errorf("unmarshal manifest: %v", err)
	}
	return obj.GetName(), nil
}

func pgFailureDeployInfo() (secretName string, image string) {
	deploy, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(serverTestOpts.serverNamespace).Get(
		ctx, "maestro", metav1.GetOptions{})
	Expect(err).ShouldNot(HaveOccurred())

	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == "rds" && v.Secret != nil {
			secretName = v.Secret.SecretName
			break
		}
	}
	Expect(secretName).ShouldNot(BeEmpty(), "maestro deployment has no 'rds' volume with a secret")

	image = e2eImage()
	return
}

func createPGFailurePod(name string, args []string) string {
	podName := fmt.Sprintf("pgfailure-%s-%s", name, rand.String(5))
	secretName, image := pgFailureDeployInfo()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: serverTestOpts.serverNamespace,
			Labels:    map[string]string{"app": "pgfailure"},
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "true",
				"proxy.istio.io/config":   "holdApplicationUntilProxyStarts: true\nterminationDrainDuration: 10s",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "pgfailure",
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/e2e/pgfailure"},
					Args:            args,
					VolumeMounts: []corev1.VolumeMount{
						{Name: "rds", MountPath: "/secrets/rds", ReadOnly: true},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(8081)},
						},
						PeriodSeconds: 1,
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "rds",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secretName,
						},
					},
				},
			},
		},
	}

	_, err := serverTestOpts.kubeClientSet.CoreV1().Pods(serverTestOpts.serverNamespace).Create(
		ctx, pod, metav1.CreateOptions{})
	Expect(err).ShouldNot(HaveOccurred())
	return podName
}

func scaleDeployment(name, namespace string, replicas int32) {
	deploy, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).ShouldNot(HaveOccurred())

	deploy.Spec.Replicas = &replicas
	_, err = serverTestOpts.kubeClientSet.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	Expect(err).ShouldNot(HaveOccurred())

	By("waiting for replicas...")
	Eventually(func() error {
		d, err := serverTestOpts.kubeClientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		By(fmt.Sprintf("ready replicas: %d, available replicas: %d", d.Status.ReadyReplicas, d.Status.AvailableReplicas))
		if d.Status.ReadyReplicas != replicas {
			return fmt.Errorf("waiting for scale: ready replicas %d/%d", d.Status.ReadyReplicas, replicas)
		}
		if d.Status.AvailableReplicas != replicas {
			return fmt.Errorf("waiting for scale: available replicas %d/%d", d.Status.AvailableReplicas, replicas)
		}
		return nil
	}, 2*time.Minute, 2*time.Second).Should(Succeed())
	By("replicas ready")
}

func checkPodReady(name string) error {
	pod, err := serverTestOpts.kubeClientSet.CoreV1().Pods(serverTestOpts.serverNamespace).Get(
		ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return nil
		}
	}
	return fmt.Errorf("pod %s not ready", name)
}
