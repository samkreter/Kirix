package runner

import (
	"fmt"
	"log"

	"github.com/samkreter/Kirix/providers/aci"
	"github.com/samkreter/Kirix/sources/serviceBus"
)

type Source interface {
	GetWork() (string, error)
}

type Provider interface {
	CreateComputeInstance(name string, work string) error

	SendWork(name string) error

	DeleteComputeInstance(name string) error
}

type Runner struct {
	Sources  []Source
	Provider Provider
}

func New(sources []string, sourceConfig string, provider string) (*Runner, error) {

	var runner Runner

	// Set up the sources
	for _, source := range sources {
		switch source {
		case "serviceBus":
			s, err := serviceBus.NewServiceBusSource(sourceConfig)
			if err != nil {
				return nil, err
			}

			log.Println("Adding source: ", source)
			runner.Sources = append(runner.Sources, s)

		default:
			fmt.Printf("Source '%s' is not supported\n", source)
		}
	}

	// Set up Provider
	switch provider {
	case "aci":
		// TODO: set up config
		p, err := aci.NewACIProvider("", "Linux", "nginx")
		if err != nil {
			return &Runner{}, err
		}
		runner.Provider = p

		fmt.Println("Added provider: ", provider)
	default:
		return nil, fmt.Errorf("No providers available named: %s", provider)
	}

	return &runner, nil
}

func (r *Runner) Run() error {
	fmt.Println("running")

	// Created
	// err := r.Provider.CreateComputeInstance("sam-test-1", "testwork")
	// if err != nil {
	// 	log.Fatal("Error in creation: ", err)
	// }

	// // Delete
	// err = r.Provider.DeleteComputeInstance("sam-test-1")
	// if err != nil {
	// 	log.Fatal("Error in creation: ", err)
	// }
	return fmt.Errorf("Runner not implemneted.")
}
