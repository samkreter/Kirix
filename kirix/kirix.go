package kirix

import (
	"fmt"

	"github.com/samkreter/Kirix/sources/serviceBus"
)

func New(source, sourceConfig string) error {

	var s Source
	var err error

	switch source {
	case "serviceBus":
		s, err = serviceBus.NewServiceBusSource(sourceConfig)
		if err != nil {
			return err
		}
	default:
		fmt.Printf("Source '%s' is not supported\n", source)
	}

	s.GetItem()

	return fmt.Errorf("Not Implemented")
}
