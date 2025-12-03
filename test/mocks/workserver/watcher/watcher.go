package watcher

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
	workv1 "open-cluster-management.io/api/work/v1"
)

type WorkStore struct {
	sync.RWMutex
	works map[string]*workv1.ManifestWork
}

func (s *WorkStore) CreateOrUpdate(work *workv1.ManifestWork) {
	s.Lock()
	defer s.Unlock()

	if work == nil {
		return
	}
	s.works[work.Name] = work
}

func (s *WorkStore) Get(name string) *workv1.ManifestWork {
	s.RLock()
	defer s.RUnlock()

	return s.works[name]
}

func StartWatch(ctx context.Context, watcher watch.Interface) *WorkStore {
	works := &WorkStore{works: make(map[string]*workv1.ManifestWork)}
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
						works.CreateOrUpdate(work)
					}
				case watch.Deleted:
					if work, ok := event.Object.(*workv1.ManifestWork); ok {
						works.CreateOrUpdate(work)
					}
				}
			}
		}
	}()

	return works
}
