package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

const (
	numResources     = 500
	numStatusWorkers = 2
	timeout          = 5 * time.Minute
)

type phase int32

const (
	phaseCreating phase = iota
	phaseCreated
	phaseUpdating
	phaseUpdated
	phaseDeleting
	phaseDeleted
	phaseVerifying
	phaseVerified
)

var phaseNames = [...]string{
	phaseCreating:  "Creating",
	phaseCreated:   "Created",
	phaseUpdating:  "Updating",
	phaseUpdated:   "Updated",
	phaseDeleting:  "Deleting",
	phaseDeleted:   "Deleted",
	phaseVerifying: "Verifying",
	phaseVerified:  "Verified",
}

func (p phase) String() string {
	if int(p) < len(phaseNames) {
		return phaseNames[p]
	}
	return fmt.Sprintf("Unknown(%d)", p)
}

type result struct {
	mu          sync.Mutex
	name        string
	phase       phase
	phaseErrors map[phase][]error
}

func (r *result) retry(err error) {
	klog.V(4).Infof("recording error for retry: %s: %s", r.name, err)
	r.phaseErrors[r.phase] = append(r.phaseErrors[r.phase], err)
}

// This test is intended to assert maestro recovers from different failure modes while under load.
// Unlike the scenarios in pg_failure_modes_test, this test cannot be considered deterministic.
//
// This test attempts to send N resources through a create/update/delete/verify lifecycle, where
// update/delete/verification is triggered by the reception of a watched status event for a
// resource.  By asserting that all N events successfully complete the entire lifecycle, we show
// that maestro can recover gracefully from any of the failure modes induced.
//
// This is accomplished by the orchestration of two sets of goroutines:
// 1. Status watchers, that receive status events from a source work client watch channel
// 2. A single "action" goroutine that reads from a k8s workqueue to process the initial creation
// requests for each resource, and to retry failed operations performed by the status watchers.
// 3. A single "chaos" goroutine that uses the pgfailure tool to induce failure modes.
//
// These goroutines keep track of the current phase of each resource and any errors observered at
// that phase via a map of result structs.
//
// Once all resources have completed the full lifecycle, or a timer has been exceeded, the results
// are summarized.  A successful run means all resources completed the full lifecycle within the
// alotted time.
var _ = Describe("Injecting known failure modes while under load", Ordered, Label("e2e-tests-stress"), func() {
	var oldCtx context.Context

	BeforeAll(func() {
		// This test produces excessive "publish event" logs with the default logger, this will
		// remove those logs.
		oldCtx = ctx
		ctx = klog.NewContext(ctx, logr.Discard())
	})

	AfterAll(func() {
		cleanupFailurePods()
		cleanupResources(ctx)
		ctx = oldCtx
	})

	It("should process a burst of resources through create/update/delete lifecycle", func() {

		By("starting status watcher")
		watcher, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Watch(
			ctx, metav1.ListOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		defer watcher.Stop()
		statusCh := watcher.ResultChan()

		By(fmt.Sprintf("creating work queue for %d resources", numResources))
		queue := workqueue.NewTypedRateLimitingQueue(workqueue.NewTypedItemFastSlowRateLimiter[*result](10*time.Millisecond, time.Second, 5))

		var results sync.Map
		var completedCount atomic.Int32
		done := make(chan struct{})
		closeDone := sync.OnceFunc(func() { close(done) })

		By(fmt.Sprintf("starting %d status worker goroutines", numStatusWorkers))
		var statusWg sync.WaitGroup
		for i := range numStatusWorkers {
			statusWg.Add(1)
			go func(workerID int) {
				defer GinkgoRecover()
				defer statusWg.Done()
				statusWatcherLoop(statusCh, done, &results, &completedCount, queue, closeDone)
			}(i)
		}

		By("starting queue worker goroutine")
		var queueWg sync.WaitGroup
		queueWg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer queueWg.Done()
			createAndRetryLoop(queue, &completedCount, closeDone)
		}()

		By("starting chaos goroutine")
		chaosStop := make(chan struct{})
		var chaosWg sync.WaitGroup
		chaosWg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer chaosWg.Done()
			chaosLoop(chaosStop, &completedCount)
		}()

		start := time.Now()
		By(fmt.Sprintf("sending %d work names", numResources))
		for i := range numResources {
			name := fmt.Sprintf("stress-work-%04d-%s", i, rand.String(5))
			r := &result{
				name:        name,
				phase:       phaseCreating,
				phaseErrors: make(map[phase][]error),
			}
			results.Store(r.name, r)
			queue.Add(r)
		}

		By("waiting for status workers to finish")
		statusWg.Wait()
		queue.ShutDown()

		By("waiting for queue worker to finish")
		queueWg.Wait()

		By("stopping chaos goroutine")
		close(chaosStop)
		chaosWg.Wait()

		By(fmt.Sprintf("workers completed in %s", time.Since(start)))

		By("checking results")
		var totalCount, successCount, failCount int
		var failures []string
		results.Range(func(key, value any) bool {
			name := key.(string)
			r := value.(*result)
			totalCount++

			if r.phase == phaseVerified {
				successCount++
			} else {
				failCount++
				failures = append(failures, fmt.Sprintf("%s: stuck at phase %s", name, r.phase))
				for phase, errs := range r.phaseErrors {
					for _, e := range errs {
						failures = append(failures, fmt.Sprintf("  phase %s: %v", phase, e))
					}
				}
			}
			return true
		})

		By(fmt.Sprintf("results: %d total, %d succeeded, %d failed", totalCount, successCount, failCount))
		for _, f := range failures {
			By(fmt.Sprintf("FAILURE: %s", f))
		}
		Expect(totalCount).To(Equal(numResources))
		Expect(failCount).To(Equal(0))
	})
})

func statusWatcherLoop(
	statusCh <-chan watch.Event,
	done <-chan struct{},
	results *sync.Map,
	completedCount *atomic.Int32,
	queue workqueue.TypedRateLimitingInterface[*result],
	closeDone func(),
) {
	timer := time.After(timeout)

	for {
		select {
		case <-done:
			return
		case <-timer:
			return
		case event, ok := <-statusCh:
			if !ok {
				By("status channel is closed")
				return
			}
			if event.Type != watch.Modified && event.Type != watch.Deleted {
				klog.V(4).Infof("ignoring event type %s", event.Type)
			}
			work, ok := event.Object.(*workv1.ManifestWork)
			if !ok {
				klog.V(4).Infof("could not deserialize manifest work from event")
			}
			val, exists := results.Load(work.Name)
			if !exists {
				continue
			}
			r := val.(*result)
			klog.V(4).Infof("received status event for work %s", r.name)
			func() {
				r.mu.Lock()
				defer r.mu.Unlock()
				switch {
				case meta.IsStatusConditionTrue(work.Status.Conditions, common.ResourceDeleted) &&
					r.phase == phaseDeleted:
					r.phase = phaseVerifying

					_, getErr := sourceWorkClient.ManifestWorks(
						agentTestOpts.consumerName).Get(
						ctx, work.Name, metav1.GetOptions{})
					var verifyErr error
					if getErr == nil {
						verifyErr = fmt.Errorf("resource %s still exists after delete", work.Name)
					} else if !errors.IsNotFound(getErr) {
						verifyErr = getErr
					}

					if verifyErr != nil {
						r.retry(verifyErr)
						queue.AddRateLimited(r)
						return
					}

					r.phase = phaseVerified
					completed := completedCount.Add(1)
					if completed%50 == 0 || completed == int32(numResources) {
						By(fmt.Sprintf("progress: %d/%d resources completed", completed, numResources))
					}
					if completed >= int32(numResources) {
						closeDone()
					}

				case isAvailableAtGen(work, 2, r.name+"-updated") && r.phase == phaseUpdated:
					r.phase = phaseDeleting

					opIDCtx, _ := newOpIDContext(ctx)
					deleteErr := sourceWorkClient.ManifestWorks(
						agentTestOpts.consumerName).Delete(
						opIDCtx, work.Name, metav1.DeleteOptions{})
					if deleteErr != nil {
						r.retry(deleteErr)
						queue.AddRateLimited(r)
						return
					}
					r.phase = phaseDeleted

					// The maestro agent has been observed to delay delivery of deleted status for resources, resulting
					// in test flakiness.  ARO-HCP has implemented a mitigation to re-send delete requests to maestro
					// on a timer to improve resiliency.  We apply the same mitigation here by premptively adding the
					// resource to the work queue where the deletion can be verified and, if not yet fully deleted,
					// resend the delete request.
					queue.AddAfter(r, 10*time.Second)

				case isAvailableAtGen(work, 1, r.name) && r.phase == phaseCreated:
					r.phase = phaseUpdating

					updateErr := stressUpdateResource(ctx, work.Name)
					if updateErr != nil {
						r.retry(updateErr)
						queue.AddRateLimited(r)
						return
					}
					r.phase = phaseUpdated
				}
			}()
		}
	}
}

func createAndRetryLoop(
	queue workqueue.TypedRateLimitingInterface[*result],
	completedCount *atomic.Int32,
	closeDone func(),
) {
	for {
		r, shutdown := queue.Get()
		if shutdown {
			return
		}
		klog.V(4).Infof("work %s:%s received", r.name, r.phase)
		func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			switch {
			case r.phase == phaseCreating:
				klog.V(4).Infof("creating work %s", r.name)
				opIDCtx, _ := newOpIDContext(ctx)
				_, createErr := sourceWorkClient.ManifestWorks(
					agentTestOpts.consumerName).Create(
					opIDCtx, NewManifestWork(r.name), metav1.CreateOptions{})
				if createErr != nil && !errors.IsAlreadyExists(createErr) && !errors.IsConflict(createErr) {
					r.retry(createErr)
					queue.AddRateLimited(r)
					return
				}
				r.phase = phaseCreated

			case r.phase == phaseUpdating:
				updateErr := stressUpdateResource(ctx, r.name)
				if updateErr != nil {
					r.retry(updateErr)
					queue.AddRateLimited(r)
					return
				}
				r.phase = phaseUpdated

			case r.phase == phaseDeleting:
				opIDCtx, _ := newOpIDContext(ctx)
				deleteErr := sourceWorkClient.ManifestWorks(
					agentTestOpts.consumerName).Delete(
					opIDCtx, r.name, metav1.DeleteOptions{})
				if deleteErr != nil && !errors.IsNotFound(deleteErr) {
					r.retry(deleteErr)
					queue.AddRateLimited(r)
					return
				}
				r.phase = phaseDeleted

			case r.phase == phaseDeleted:
				_, getErr := sourceWorkClient.ManifestWorks(
					agentTestOpts.consumerName).Get(
					ctx, r.name, metav1.GetOptions{})
				if errors.IsNotFound(getErr) {
					return
				}

				r.retry(fmt.Errorf("deletion was delayed for resource %s, resending", r.name))
				opIDCtx, _ := newOpIDContext(ctx)
				deleteErr := sourceWorkClient.ManifestWorks(
					agentTestOpts.consumerName).Delete(
					opIDCtx, r.name, metav1.DeleteOptions{})
				if deleteErr != nil {
					r.retry(deleteErr)
					queue.AddRateLimited(r)
					return
				}

				// check again in 10 seconds
				queue.AddAfter(r, 10*time.Second)

			case r.phase == phaseVerifying:
				_, getErr := sourceWorkClient.ManifestWorks(
					agentTestOpts.consumerName).Get(
					ctx, r.name, metav1.GetOptions{})
				if errors.IsNotFound(getErr) {
					r.phase = phaseVerified
					completed := completedCount.Add(1)
					if completed%50 == 0 || completed == int32(numResources) {
						By(fmt.Sprintf("progress: %d/%d resources completed", completed, numResources))
					}
					if completed >= int32(numResources) {
						closeDone()
					}
					return
				}
				var verifyErr error
				if getErr == nil {
					verifyErr = fmt.Errorf("resource %s still exists after delete", r.name)
				} else {
					verifyErr = getErr
				}
				r.retry(verifyErr)
				queue.AddRateLimited(r)
			}
			queue.Forget(r)
		}()
		queue.Done(r)
	}
}

func isAvailableAtGen(work *workv1.ManifestWork, gen int64, expectedManifestName string) bool {
	appliedAtGen := false
	for _, c := range work.Status.Conditions {
		if c.Type == "Applied" && c.Status == metav1.ConditionTrue && c.ObservedGeneration == gen {
			appliedAtGen = true
			break
		}
	}
	if !appliedAtGen {
		return false
	}

	manifests := work.Status.ResourceStatus.Manifests
	if len(manifests) != 1 {
		return false
	}

	m := manifests[0]
	if m.ResourceMeta.Name != expectedManifestName {
		return false
	}

	return meta.IsStatusConditionTrue(m.Conditions, "Applied") &&
		meta.IsStatusConditionTrue(m.Conditions, "Available")
}

func chaosLoop(stop <-chan struct{}, completedCount *atomic.Int32) {
	if chaosSleep(1*time.Second, stop) {
		return
	}

	type chaosAction struct {
		name         string
		failureArgs  []string
		recoveryArgs []string
		holdDuration time.Duration
	}

	pgActions := []chaosAction{
		{
			name:         "block event writes",
			failureArgs:  pgfailureCommandArgs("block events"),
			recoveryArgs: pgfailureCommandArgs("unblock events"),
			holdDuration: 5 * time.Second,
		},
		{
			name:         "NOTIFY drop on events",
			failureArgs:  pgfailureCommandArgs("lock events"),
			recoveryArgs: pgfailureCommandArgs("terminate listeners events", "unlock events"),
			holdDuration: 5 * time.Second,
		},
		{
			name:         "block status_events writes",
			failureArgs:  pgfailureCommandArgs("block status_events"),
			recoveryArgs: pgfailureCommandArgs("unblock status_events"),
			holdDuration: 5 * time.Second,
		},
	}

	for {
		for _, action := range pgActions {
			if completedCount.Load() >= int32(numResources) {
				return
			}

			By(fmt.Sprintf("chaos: injecting %s", action.name))
			failPod := createPGFailurePod("chaos-fail", action.failureArgs)
			Eventually(func() error { return checkPodReady(failPod) }, 1*time.Minute, 2*time.Second).Should(Succeed())

			chaosSleep(action.holdDuration, stop)

			By(fmt.Sprintf("chaos: recovering %s", action.name))
			recoverPod := createPGFailurePod("chaos-recover", action.recoveryArgs)
			Eventually(func() error { return checkPodReady(recoverPod) }, 1*time.Minute, 2*time.Second).Should(Succeed())
			cleanupFailurePods()

			if chaosSleep(10*time.Second, stop) {
				return
			}
		}

		if completedCount.Load() >= int32(numResources) {
			return
		}

		if chaosSleep(15*time.Second, stop) {
			return
		}
	}
}

func chaosSleep(d time.Duration, stop <-chan struct{}) bool {
	select {
	case <-time.After(d):
		return false
	case <-stop:
		return true
	}
}

func stressUpdateResource(ctx context.Context, workName string) error {
	work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(
		ctx, workName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	newWork := work.DeepCopy()
	newWork.Spec.Workload.Manifests = []workv1.Manifest{NewManifest(workName + "-updated")}
	patchData, err := grpcsource.ToWorkPatch(work, newWork)
	if err != nil {
		return err
	}
	opIDCtx, _ := newOpIDContext(ctx)
	_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(
		opIDCtx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
	return err
}
