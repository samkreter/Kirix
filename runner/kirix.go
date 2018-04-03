package runner

import (
	"fmt"

	"github.com/samkreter/Kirix/sources/serviceBus"
)

type Source interface {
	GetWork() (string, error)
}

type Runner struct {
	Sources []Source
}

func New(sources []string, sourceConfig string) (*Runner, error) {

	var runner Runner

	// Set up the sources
	for _, source := range sources {
		switch source {
		case "serviceBus":
			s, err := serviceBus.NewServiceBusSource(sourceConfig)
			if err != nil {
				return nil, err
			}

			runner.Sources = append(runner.Sources, s)

		default:
			fmt.Printf("Source '%s' is not supported\n", source)
		}
	}

	return &runner, nil
}

func (r *Runner) Run() error {
	return fmt.Errorf("Not implemneted.")
}
