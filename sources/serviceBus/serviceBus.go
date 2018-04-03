package serviceBus

import (
	"fmt"
	"log"
)

type ServiceBusSource struct{}

func NewServiceBusSource(sourceConfig string) (*ServiceBusSource, error) {
	log.Println("Got config: ", sourceConfig)
	return &ServiceBusSource{}, nil
}

func (s *ServiceBusSource) GetWork() (string, error) {
	return "", fmt.Errorf("ServiceBus GetWork Not Implemented")
}
