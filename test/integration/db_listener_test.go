package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/maestro/test"
)

func TestWaitForNotification(t *testing.T) {
	// it is used to check the result of the notification
	result := make(chan string)

	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))
	defer func() {
		cancel()
	}()

	g2 := h.Env().Database.SessionFactory.New(ctx)

	listener := h.Env().Database.SessionFactory.NewListener(ctx, "events", func(id string) {
		result <- id
	})
	var originalListenerId string
	// find the original listener id in the pg_stat_activity table
	g2.Raw("SELECT pid FROM pg_stat_activity WHERE query LIKE 'LISTEN%'").Scan(&originalListenerId)
	if originalListenerId == "" {
		t.Errorf("the original Listener was not recreated")
	}

	// Simulate an errListenerClosed and wait for the listener to be recreated
	listener.Close()

	Eventually(func() error {
		var newListenerId string
		g2.Raw("SELECT pid FROM pg_stat_activity WHERE query LIKE 'LISTEN%'").Scan(&newListenerId)
		if newListenerId == "" {
			return fmt.Errorf("the new Listener was not created")
		}
		// Validate the listener was recreated
		if originalListenerId == newListenerId {
			return fmt.Errorf("Listener was not recreated")
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	// send a notification to the new listener
	g2.Exec("NOTIFY events, 'test'")

	// wait for the notification to be received
	time.Sleep(1 * time.Second)

	if <-result != "test" {
		t.Errorf("the notification was not received")
	}
}
