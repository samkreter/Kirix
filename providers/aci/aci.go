package aci

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
)

type ACIProvider struct {
	aciClient       *aci.Client
	image           string
	command         []string
	resourceGroup   string
	region          string
	operatingSystem string
	cpu             string
	memory          string
	cinstances      string
}

// NewACIProvider creates a new ACIProvider.
func NewACIProvider(config string, operatingSystem string, image string) (*ACIProvider, error) {
	var p ACIProvider
	var err error

	p.aciClient, err = aci.NewClient()
	if err != nil {
		return nil, err
	}

	p.image = image

	if config != "" {
		f, err := os.Open(config)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if err := p.loadConfig(f); err != nil {
			return nil, err
		}
	}

	if rg := os.Getenv("ACI_RESOURCE_GROUP"); rg != "" {
		p.resourceGroup = rg
	}
	if p.resourceGroup == "" {
		return nil, errors.New("Resource group can not be empty please set ACI_RESOURCE_GROUP")
	}

	if r := os.Getenv("ACI_REGION"); r != "" {
		p.region = r
	}
	if p.region == "" {
		return nil, errors.New("Region can not be empty please set ACI_REGION")
	}

	p.cpu = "20"
	p.memory = "100Gi"
	p.cinstances = "20"

	p.operatingSystem = operatingSystem

	return &p, err
}

func (p *ACIProvider) CreateComputeInstance(name string, work string) error {
	//TODO: Get default container group, set work as ENV
	containerGroup, err := p.GetSingleImageContainerGroup(work)
	if err != nil {
		return err
	}

	_, err = p.aciClient.CreateContainerGroup(
		p.resourceGroup,
		name,
		*containerGroup,
	)

	return err
}

func (p *ACIProvider) DeleteComputeInstance(name string) error {
	return p.aciClient.DeleteContainerGroup(p.resourceGroup, name)
}

func (p *ACIProvider) SendWork(name string) error {
	return fmt.Errorf("Not Implemented")
}

func (p *ACIProvider) GetSingleImageContainerGroup(work string) (*aci.ContainerGroup, error) {
	var containerGroup aci.ContainerGroup
	containerGroup.Location = p.region
	containerGroup.RestartPolicy = aci.ContainerGroupRestartPolicy("Always")
	containerGroup.ContainerGroupProperties.OsType = aci.OperatingSystemTypes(p.operatingSystem)

	// TODO: Allow private repos

	container := aci.Container{
		Name: "worker-container",
		ContainerProperties: aci.ContainerProperties{
			Image:   p.image,
			Command: p.command,
			Ports:   make([]aci.ContainerPort, 0),
			EnvironmentVariables: []aci.EnvironmentVariable{
				aci.EnvironmentVariable{
					Name:  "KIRIX_WORK",
					Value: work,
				},
			},
			Resources: aci.ResourceRequirements{
				Limits: aci.ResourceLimits{
					CPU:        1,
					MemoryInGB: 1,
				},
				Requests: aci.ResourceRequests{
					CPU:        1,
					MemoryInGB: 1,
				},
			},
		},
	}

	containerGroup.ContainerGroupProperties.Containers = []aci.Container{container}

	// ports := []aci.Port{
	// 	aci.Port{
	// 		Port:     80,
	// 		Protocol: aci.ContainerGroupNetworkProtocol("TCP"),
	// 	},
	// }

	// containerGroup.ContainerGroupProperties.IPAddress = &aci.IPAddress{
	// 	Ports: ports,
	// 	Type:  "Public",
	// }

	return &containerGroup, nil
}

func (p *ACIProvider) GetComputeInstance(namespace, name string) (*aci.ContainerGroup, error) {
	cg, err, status := p.aciClient.GetContainerGroup(p.resourceGroup, fmt.Sprintf("%s-%s", namespace, name))
	if err != nil {
		if *status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return cg, nil
}

func (p *ACIProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	logContent := ""
	cg, err, _ := p.aciClient.GetContainerGroup(p.resourceGroup, fmt.Sprintf("%s-%s", namespace, podName))
	if err != nil {
		return logContent, err
	}

	// get logs from cg
	retry := 10
	for i := 0; i < retry; i++ {
		cLogs, err := p.aciClient.GetContainerLogs(p.resourceGroup, cg.Name, containerName, tail)
		if err != nil {
			log.Println(err)
			time.Sleep(5000 * time.Millisecond)
		} else {
			logContent = cLogs.Content
			break
		}
	}

	return logContent, err
}

func (p *ACIProvider) GetCurrentComputeInstances() ([]aci.ContainerGroup, error) {
	cgs, err := p.aciClient.ListContainerGroups(p.resourceGroup)
	if err != nil {
		return nil, err
	}

	return cgs.Value, nil
}
