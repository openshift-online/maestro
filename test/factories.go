package test

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
)

func (helper *Helper) NewDinosaur(species string) *api.Dinosaur {
	dinoService := helper.Env().Services.Dinosaurs()

	dinosaur := &api.Dinosaur{
		Species: species,
	}

	dino, err := dinoService.Create(context.Background(), dinosaur)
	if err != nil {
		helper.T.Errorf("error creating dinosaur: %q", err)
	}

	return dino
}

func (helper *Helper) NewDinosaurList(namePrefix string, count int) (dinosaurs []*api.Dinosaur) {
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		dinosaurs = append(dinosaurs, helper.NewDinosaur(name))
	}
	return dinosaurs
}
