package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/test"
)

func TestHealthCheckServer(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	// insert two existing instances, one is ready and the other is not
	_, err := instanceDao.Create(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: "instance1",
		},
		// last heartbeat is 3 seconds ago
		LastHeartbeat: time.Now().Add(-3 * time.Second),
		Ready:         true,
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = instanceDao.Create(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID: "instance2",
		},
		// last heartbeat is 3 seconds ago
		LastHeartbeat: time.Now().Add(-3 * time.Second),
		Ready:         false,
	})
	Expect(err).NotTo(HaveOccurred())

	instanceID := &h.Env().Config.MessageBroker.ClientID
	Eventually(func() error {
		instances, err := instanceDao.All(ctx)
		if err != nil {
			return err
		}

		if len(instances) != 3 {
			return fmt.Errorf("expected 3 instances, got %d", len(instances))
		}

		readyInstanceIDs, err := instanceDao.FindReadyIDs(ctx)
		if err != nil {
			return err
		}

		if len(readyInstanceIDs) != 1 {
			return fmt.Errorf("expected 1 ready instance, got %d", len(readyInstanceIDs))
		}

		if readyInstanceIDs[0] != *instanceID {
			return fmt.Errorf("expected instance %s to be ready, got %s", *instanceID, readyInstanceIDs[0])
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())
}
