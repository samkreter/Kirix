package aci

/* TODO:
- Add image pull secrets
*/

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	client "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
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

	var azAuth *client.Authentication

	if authFilepath := os.Getenv("AZURE_AUTH_LOCATION"); authFilepath != "" {
		auth, err := client.NewAuthenticationFromFile(authFilepath)
		if err != nil {
			return nil, err
		}

		azAuth = auth
	}

	p.aciClient, err = aci.NewClient(azAuth)
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

func (p *ACIProvider) GetACIFromK8sPod(pod *v1.Pod) error {
	var containerGroup aci.ContainerGroup
	containerGroup.Location = p.region
	containerGroup.RestartPolicy = aci.ContainerGroupRestartPolicy(pod.Spec.RestartPolicy)
	containerGroup.ContainerGroupProperties.OsType = aci.OperatingSystemTypes(p.operatingSystem)

	// get containers
	containers, err := p.getContainers(pod)
	if err != nil {
		return err
	}

	// get volumes
	volumes, err := p.getVolumes(pod)
	if err != nil {
		return err
	}
	// assign all the things
	containerGroup.ContainerGroupProperties.Containers = containers
	containerGroup.ContainerGroupProperties.Volumes = volumes

	// create ipaddress if containerPort is used
	count := 0
	for _, container := range containers {
		count = count + len(container.Ports)
	}
	ports := make([]aci.Port, 0, count)
	for _, container := range containers {
		for _, containerPort := range container.Ports {

			ports = append(ports, aci.Port{
				Port:     containerPort.Port,
				Protocol: aci.ContainerGroupNetworkProtocol("TCP"),
			})
		}
	}
	if len(ports) > 0 {
		containerGroup.ContainerGroupProperties.IPAddress = &aci.IPAddress{
			Ports: ports,
			Type:  "Public",
		}
	}

	podUID := string(pod.UID)
	podCreationTimestamp := pod.CreationTimestamp.String()
	containerGroup.Tags = map[string]string{
		"PodName":           pod.Name,
		"ClusterName":       pod.ClusterName,
		"NodeName":          pod.Spec.NodeName,
		"Namespace":         pod.Namespace,
		"UID":               podUID,
		"CreationTimestamp": podCreationTimestamp,
	}

	_, err = p.aciClient.CreateContainerGroup(
		p.resourceGroup,
		fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
		containerGroup,
	)

	return err
}

func (p *ACIProvider) getVolumes(pod *v1.Pod) ([]aci.Volume, error) {
	volumes := make([]aci.Volume, 0, len(pod.Spec.Volumes))
	for _, v := range pod.Spec.Volumes {
		// Handle the case for the EmptyDir.
		if v.EmptyDir != nil {
			volumes = append(volumes, aci.Volume{
				Name:     v.Name,
				EmptyDir: map[string]interface{}{},
			})
			continue
		}

		// Handle the case for GitRepo volume.
		if v.GitRepo != nil {
			volumes = append(volumes, aci.Volume{
				Name: v.Name,
				GitRepo: &aci.GitRepoVolume{
					Directory:  v.GitRepo.Directory,
					Repository: v.GitRepo.Repository,
					Revision:   v.GitRepo.Revision,
				},
			})
			continue
		}
	}

	return volumes, nil
}

func (p *ACIProvider) getContainers(pod *v1.Pod) ([]aci.Container, error) {
	containers := make([]aci.Container, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		c := aci.Container{
			Name: container.Name,
			ContainerProperties: aci.ContainerProperties{
				Image:   container.Image,
				Command: append(container.Command, container.Args...),
				Ports:   make([]aci.ContainerPort, 0, len(container.Ports)),
			},
		}

		for _, p := range container.Ports {
			c.Ports = append(c.Ports, aci.ContainerPort{
				Port:     p.ContainerPort,
				Protocol: getProtocol(p.Protocol),
			})
		}

		c.VolumeMounts = make([]aci.VolumeMount, 0, len(container.VolumeMounts))
		for _, v := range container.VolumeMounts {
			c.VolumeMounts = append(c.VolumeMounts, aci.VolumeMount{
				Name:      v.Name,
				MountPath: v.MountPath,
				ReadOnly:  v.ReadOnly,
			})
		}

		c.EnvironmentVariables = make([]aci.EnvironmentVariable, 0, len(container.Env))
		for _, e := range container.Env {
			c.EnvironmentVariables = append(c.EnvironmentVariables, aci.EnvironmentVariable{
				Name:  e.Name,
				Value: e.Value,
			})
		}

		cpuLimit := float64(container.Resources.Limits.Cpu().Value())
		memoryLimit := float64(container.Resources.Limits.Memory().Value()) / 1000000000.00
		cpuRequest := float64(container.Resources.Requests.Cpu().Value())
		memoryRequest := float64(container.Resources.Requests.Memory().Value()) / 1000000000.00

		c.Resources = aci.ResourceRequirements{
			Limits: aci.ResourceLimits{
				CPU:        cpuLimit,
				MemoryInGB: memoryLimit,
			},
			Requests: aci.ResourceRequests{
				CPU:        cpuRequest,
				MemoryInGB: memoryRequest,
			},
		}

		containers = append(containers, c)
	}
	return containers, nil
}

func getProtocol(pro v1.Protocol) aci.ContainerNetworkProtocol {
	switch pro {
	case v1.ProtocolUDP:
		return aci.ContainerNetworkProtocolUDP
	default:
		return aci.ContainerNetworkProtocolTCP
	}
}