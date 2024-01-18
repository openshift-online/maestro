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

func TestPulseServer(t *testing.T) {
	ctx := context.Background()
	h, _ := test.RegisterIntegration(t)

	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	instanceID := &h.Env().Config.MessageBroker.ClientID

	Eventually(func() error {
		instances, err := instanceDao.All(ctx)
		if err != nil {
			return err
		}

		if len(instances) != 1 {
			return fmt.Errorf("expected 1 instance, got %d", len(instances))
		}

		instance := instances[0]
		if instance.UpdatedAt.IsZero() {
			return fmt.Errorf("expected instance.UpdatedAt to be non-zero")
		}

		if instance.ID != *instanceID {
			return fmt.Errorf("expected instance.ID to be %s, got %s", *instanceID, instance.ID)
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	// insert two outdated instances
	_, err := instanceDao.UpSert(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID:        "outdated1",
			UpdatedAt: time.Now().Add(-2 * time.Minute),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	_, err = instanceDao.UpSert(ctx, &api.ServerInstance{
		Meta: api.Meta{
			ID:        "outdated2",
			UpdatedAt: time.Now().Add(-2 * time.Minute),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	// check that the outdated instances are deleted
	Eventually(func() error {
		instances, err := instanceDao.All(ctx)
		if err != nil {
			return err
		}

		if len(instances) != 1 {
			return fmt.Errorf("expected 1 instance, got %d", len(instances))
		}

		instance := instances[0]
		if instance.UpdatedAt.IsZero() {
			return fmt.Errorf("expected instance.UpdatedAt to be non-zero")
		}

		if instance.ID != *instanceID {
			return fmt.Errorf("expected instance.ID to be %s, got %s", *instanceID, instance.ID)
		}

		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())
}
