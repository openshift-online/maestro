package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
)

type testDispatcher struct {
	dispatcher Dispatcher
	ctx        context.Context
	cancel     context.CancelFunc
}

func TestDispatcher(t *testing.T) {
	RegisterTestingT(t)

	cases := []struct {
		name              string
		existingConsumers []string
		existingInstances []string
		newConsumers      []string
		newInstances      []string
		stoppedInstances  []string
		expectedOwnship   map[string][]string
	}{
		{
			name:              "no existing consumers or instances",
			existingConsumers: []string{},
			existingInstances: []string{},
			expectedOwnship:   map[string][]string{},
		},
		{
			name:              "new consumers",
			existingConsumers: []string{},
			existingInstances: []string{},
			newConsumers:      []string{"foo", "bar"},
			expectedOwnship:   map[string][]string{},
		},
		{
			name:              "new instances",
			existingConsumers: []string{},
			existingInstances: []string{},
			newInstances:      []string{"fred", "plugh"},
			expectedOwnship:   map[string][]string{},
		},
		{
			name:              "existing consumers and instances",
			existingConsumers: []string{"foo", "bar"},
			existingInstances: []string{"fred", "thud"},
			expectedOwnship:   map[string][]string{"fred": {"foo"}, "thud": {"bar"}},
		},
		{
			name:              "existing consumers and instances, new consumers",
			existingConsumers: []string{"foo", "bar"},
			existingInstances: []string{"fred", "thud"},
			newConsumers:      []string{"baz", "qux"},
			expectedOwnship:   map[string][]string{"fred": {"foo", "baz"}, "thud": {"bar", "qux"}},
		},
		{
			name:              "existing consumers and instances, new instances",
			existingConsumers: []string{"foo", "bar"},
			existingInstances: []string{"fred", "thud"},
			newInstances:      []string{"plugh", "xyzzy"},
			expectedOwnship:   map[string][]string{"fred": {"foo"}, "thud": {"bar"}},
		},
		{
			name:              "existing consumers and instances, new consumers and instances",
			existingConsumers: []string{"foo", "bar"},
			existingInstances: []string{"fred", "thud"},
			newConsumers:      []string{"baz", "qux"},
			newInstances:      []string{"plugh", "xyzzy"},
			expectedOwnship:   map[string][]string{"fred": {"foo"}, "thud": {"bar"}, "plugh": {"baz"}, "xyzzy": {"qux"}},
		},
		{
			name:              "existing consumers and instances, new consumers and instances, delete instances",
			existingConsumers: []string{"foo", "bar"},
			existingInstances: []string{"fred", "thud"},
			newConsumers:      []string{"baz", "qux"},
			newInstances:      []string{"plugh", "xyzzy"},
			stoppedInstances:  []string{"fred"},
			expectedOwnship:   map[string][]string{"thud": {"bar"}, "plugh": {"baz"}, "xyzzy": {"qux", "foo"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootCtx, rootCtxCancel := context.WithCancel(context.Background())
			dispatcherMap := map[string]testDispatcher{}
			instanceDao := mocks.NewInstanceDao()
			consumerDao := mocks.NewConsumerDao()
			t.Logf(" 1. creating existing consumers: %v", tc.existingConsumers)
			for _, consumerID := range tc.existingConsumers {
				consumerDao.Create(context.Background(), &api.Consumer{Meta: api.Meta{ID: consumerID}})
			}
			t.Logf("2. initializing dispatchers with existing instances: %v", tc.existingInstances)
			for _, instanceID := range tc.existingInstances {
				existingDispatcher := NewDispatcher(instanceDao, consumerDao, instanceID, 2, 2)
				ctx, cancel := context.WithCancel(rootCtx)
				go existingDispatcher.Start(ctx)
				dispatcherMap[instanceID] = testDispatcher{
					dispatcher: existingDispatcher,
					ctx:        ctx,
					cancel:     cancel,
				}
			}

			Eventually(func() error {
				instances, err := instanceDao.All(rootCtx)
				if err != nil {
					return err
				}
				if len(instances) != len(tc.existingInstances) {
					return fmt.Errorf("expected %d instances, got %d", len(tc.existingInstances), len(instances))
				}
				return nil
			}, 10*time.Second, 1*time.Second).Should(Succeed())

			Eventually(func() error {
				consumerSum := 0
				for instanceID, dispatcher := range dispatcherMap {
					dispatcherImpl := dispatcher.dispatcher.(*DispatcherImpl)
					consistent := dispatcherImpl.consistent
					if consistent == nil {
						return fmt.Errorf("consistent hash ring not initialized for instance %s", instanceID)
					}
					if len(consistent.GetMembers()) != len(tc.existingInstances) {
						return fmt.Errorf("expected %d members, got %d for instance %s,", len(tc.existingInstances), len(consistent.GetMembers()), instanceID)
					}
					consumerSum += dispatcherImpl.consumerSet.Cardinality()
				}
				if len(tc.existingInstances) != 0 && consumerSum != len(tc.existingConsumers) {
					return fmt.Errorf("expected %d consumers, got %d", len(tc.existingConsumers), consumerSum)
				}
				return nil
			}, 10*time.Second, 1*time.Second).Should(Succeed())

			t.Logf("3. checking dispatcher members and consumer sets")
			for instanceID, dispatcher := range dispatcherMap {
				dispatcherImpl := dispatcher.dispatcher.(*DispatcherImpl)
				consistent := dispatcherImpl.consistent
				t.Logf("\tinstance: %s", instanceID)
				t.Logf("\t\tcurrent members: %v", consistent.GetMembers())
				t.Logf("\t\tconsumer set: %v", dispatcherImpl.consumerSet)
			}

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				t.Logf("4. creating new consumers: %v", tc.newConsumers)
				for _, newConsumer := range tc.newConsumers {
					_, err := consumerDao.Create(rootCtx, &api.Consumer{Meta: api.Meta{ID: newConsumer}})
					Expect(err).To(BeNil())
				}
				wg.Done()
			}()

			go func() {
				t.Logf("5. starting dispatchers for new instances: %v", tc.newInstances)
				for _, newInstance := range tc.newInstances {
					newDispatcher := NewDispatcher(instanceDao, consumerDao, newInstance, 2, 2)
					ctx, cancel := context.WithCancel(rootCtx)
					go newDispatcher.Start(ctx)
					dispatcherMap[newInstance] = testDispatcher{
						dispatcher: newDispatcher,
						ctx:        ctx,
						cancel:     cancel,
					}
				}
				wg.Done()
			}()

			wg.Wait()
			Eventually(func() error {
				consumerSum := 0
				for instanceID, dispatcher := range dispatcherMap {
					dispatcherImpl := dispatcher.dispatcher.(*DispatcherImpl)
					consistent := dispatcherImpl.consistent
					if consistent == nil {
						return fmt.Errorf("consistent hash ring not initialized for instance %s", instanceID)
					}
					if len(consistent.GetMembers()) != len(tc.existingInstances)+len(tc.newInstances) {
						return fmt.Errorf("expected %d members, got %d for instance %s,", len(tc.existingInstances)+len(tc.newInstances), len(consistent.GetMembers()), instanceID)
					}
					consumerSum += dispatcherImpl.consumerSet.Cardinality()
				}
				if len(tc.existingInstances) != 0 && consumerSum != len(tc.existingConsumers)+len(tc.newConsumers) {
					return fmt.Errorf("expected %d consumers, got %d", len(tc.existingConsumers)+len(tc.newConsumers), consumerSum)
				}
				return nil
			}, 10*time.Second, 1*time.Second).Should(Succeed())

			t.Logf("6. checking dispatcher members and consumer sets")
			for instanceID, dispatcher := range dispatcherMap {
				dispatcherImpl := dispatcher.dispatcher.(*DispatcherImpl)
				consistent := dispatcherImpl.consistent
				t.Logf("\tinstance: %s", instanceID)
				t.Logf("\t\tcurrent members: %v", consistent.GetMembers())
				t.Logf("\t\tconsumer set: %v", dispatcherImpl.consumerSet)
			}

			wg.Add(1)
			go func() {
				t.Logf("7. stopping dispatchers for instances: %v", tc.stoppedInstances)
				for _, stoppedInstance := range tc.stoppedInstances {
					dispatcherMap[stoppedInstance].cancel()
					delete(dispatcherMap, stoppedInstance)
				}
				wg.Done()
			}()
			wg.Wait()

			Eventually(func() error {
				consumerSum := 0
				for instanceID, dispatcher := range dispatcherMap {
					dispatcherImpl := dispatcher.dispatcher.(*DispatcherImpl)
					consistent := dispatcherImpl.consistent
					if len(consistent.GetMembers()) != len(tc.existingInstances)+len(tc.newInstances)-len(tc.stoppedInstances) {
						return fmt.Errorf("expected %d members, got %d for instance %s,", len(tc.existingInstances)+len(tc.newInstances)-len(tc.stoppedInstances), len(consistent.GetMembers()), instanceID)
					}
					consumerSum += dispatcherImpl.consumerSet.Cardinality()
				}
				if len(tc.existingInstances) != 0 && consumerSum != len(tc.existingConsumers)+len(tc.newConsumers) {
					return fmt.Errorf("expected %d consumers, got %d", len(tc.existingConsumers)+len(tc.newConsumers), consumerSum)
				}
				return nil
			}, 10*time.Second, 1*time.Second).Should(Succeed())

			t.Logf("8. checking dispatcher members and consumer sets")
			for instanceID, dispatcher := range dispatcherMap {
				dispatcherImpl := dispatcher.dispatcher.(*DispatcherImpl)
				consistent := dispatcherImpl.consistent
				t.Logf("\tinstance: %s", instanceID)
				t.Logf("\t\tcurrent members: %v", consistent.GetMembers())
				t.Logf("\t\tconsumer set: %v", dispatcherImpl.consumerSet)
			}

			for instanceID, consumerIDs := range tc.expectedOwnship {
				dispatcher, ok := dispatcherMap[instanceID]
				Expect(ok).To(BeTrue())
				dispatcherImpl, ok := dispatcher.dispatcher.(*DispatcherImpl)
				Expect(ok).To(BeTrue())
				gotConsumerIDs := []string{}
				queueLen := dispatcherImpl.workQueue.Len()
				for i := 0; i < queueLen; i++ {
					consumerID, _ := dispatcherImpl.workQueue.Get()
					dispatcherImpl.workQueue.Forget(consumerID)
					dispatcherImpl.workQueue.Done(consumerID)
					Expect(consumerID).ToNot(BeNil())
					consumerIDStr, ok := consumerID.(string)
					Expect(ok).To(BeTrue())
					gotConsumerIDs = append(gotConsumerIDs, consumerIDStr)
				}

				Expect(gotConsumerIDs).To(ContainElements(consumerIDs))

				for _, consumerID := range consumerIDs {
					Expect(dispatcherImpl.consumerSet.Contains(consumerID)).To(BeTrue())
				}
			}

			rootCtxCancel()
		})
	}
}
