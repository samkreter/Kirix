package runner

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/samkreter/Kirix/providers/aci"
	"github.com/samkreter/Kirix/sources/serviceBus"
	types "github.com/samkreter/Kirix/types"
)

var (
	WorkChanBufferSize = 100
)

type Source interface {
	GetWork() (string, error)
}

type Compute interface {
	GetState() string
	GetName() string
}

type Provider interface {
	CreateComputeInstance(name string, work string) error

	SendWork(name string) error

	GetCurrentComputeInstances() ([]types.ComputeInstance, error)

	GetComputeInstance(name string) (*types.ComputeInstance, error)

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
		p, err := aci.NewACIProvider("", "Linux", "pskreter/test", "")
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

func SourceWatcher(source Source, workChan chan string) {
	for {
		work, err := source.GetWork()
		if err != nil {
			log.Println(err)
			continue
		}

		workChan <- work
	}
}

func (r *Runner) Run() error {
	t, _ := r.Provider.GetComputeInstance("test2")
	fmt.Println(t)
	return nil
	workChan := make(chan string, WorkChanBufferSize)

	// Delete unneeded Compute Instnaces
	go r.GarbageCollector()

	// Create a watcher for each source
	for _, source := range r.Sources {
		go SourceWatcher(source, workChan)
	}

	for {
		work := <-workChan
		freeComputes, err := r.GetFreeComputeInstances()
		if err != nil {
			return fmt.Errorf("Get Free Compute Error: %s", err)
		}

		// Get the first availble worker otherwise create a new worker
		if len(freeComputes) > 0 {
			r.Provider.CreateComputeInstance(freeComputes[0].Name, work)
		} else {
			r.Provider.CreateComputeInstance(getUniqueWorkerName(), work)
		}
	}

	return nil
}

func (r *Runner) GarbageCollector() {

	computeStaleTime := time.Duration(time.Minute * 5)

	lastChangeTime := time.Time{}
	currCount := math.MaxInt64

	for {
		freeCompute, err := r.GetFreeComputeInstances()
		if err != nil {
			log.Printf("Get Free Compute Error: %s", err)
		}

		numFree := len(freeCompute)

		if numFree == 0 || currCount == 0 {
			currCount = math.MaxInt64
		} else if numFree >= currCount {
			if time.Since(lastChangeTime) > computeStaleTime {
				err := r.Provider.DeleteComputeInstance(freeCompute[len(freeCompute)-1].Name)
				if err != nil {
					log.Printf("Delete compute instance error: %s", err)
				}
			}
		} else {
			currCount = numFree
			lastChangeTime = time.Now()
		}
	}
}

func getUniqueWorkerName() string {
	randChars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789")
	b := make([]rune, 7)
	for i := range b {
		b[i] = randChars[rand.Intn(len(randChars))]
	}
	return "kirix-worker-" + string(b)
}

func (r *Runner) GetFreeComputeInstances() ([]types.ComputeInstance, error) {
	currComputeInstances, err := r.Provider.GetCurrentComputeInstances()
	if err != nil {
		return nil, err
	}

	var freeComputeInstances []types.ComputeInstance
	for _, computeInstance := range currComputeInstances {
		if computeInstance.State == types.StateComplete {
			freeComputeInstances = append(freeComputeInstances, computeInstance)
		}
	}

	return freeComputeInstances, nil
}
