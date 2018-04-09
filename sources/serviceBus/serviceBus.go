package serviceBus

import (
	"fmt"
	"log"
	//queue "github.com/g-rad/go-azurequeue"
)

type ServiceBusSource struct{}

func NewServiceBusSource(sourceConfig string) (*ServiceBusSource, error) {
	log.Println("Got config: ", sourceConfig)
	return &ServiceBusSource{}, nil

	// cli := queue.QueueClient{
	// 	Namespace:  "my-test",
	// 	KeyName:    "RootManageSharedAccessKey",
	// 	KeyValue:   "ErCWbtgArb55Tqqu9tXgdCtopbZ44pMH01sjpMrYGrE=",
	// 	QueueName:  "my-queue",
	// 	Timeout:    60,
	// }
}

func (s *ServiceBusSource) GetWork() (string, error) {
	return "", fmt.Errorf("ServiceBus GetWork Not Implemented")
}
