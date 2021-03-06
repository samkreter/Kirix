package aci

/* TODO:
- Fix instance view state
- Add passing in command
- Add image pull secrets
*/

import (
	"errors"
	"fmt"
	"os"
	"strings"

	types "github.com/samkreter/Kirix/types"
	client "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
)

const (
	WorkEnvVarName = "KIRIX_WORK"
	AzureInfoFile  = "./azureInfo.json"
)

type ACIProvider struct {
	aciClient       *aci.Client
	workerInstance  *aci.ContainerGroup
	operatingSystem string
	region          string
	resourceGroup   string
	cpu             string
	memory          string
	cinstances      string
}

func isValidACIRegion(region string) bool {
	regionLower := strings.ToLower(region)
	regionTrimmed := strings.Replace(regionLower, " ", "", -1)

	for _, validRegion := range validAciRegions {
		if regionTrimmed == validRegion {
			return true
		}
	}

	return false
}

var validAciRegions = []string{
	"westeurope",
	"westus",
	"eastus",
	"southeastasia",
}

// NewACIProvider creates a new ACIProvider.
func NewACIProvider(config string, operatingSystem string, image string, deploymentFile string) (*ACIProvider, error) {
	var p ACIProvider
	var err error

	p.aciClient, err = CreateACIClient()
	if err != nil {
		return nil, err
	}

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
	if r := p.region; !isValidACIRegion(r) {
		unsupportedRegionMessage := fmt.Sprintf("Region %s is invalid. Current supported regions are: %s",
			r, strings.Join(validAciRegions, ", "))

		return nil, errors.New(unsupportedRegionMessage)
	}

	region := os.Getenv("ACI_REGION")
	if region == "" {
		return nil, errors.New("Region can not be empty please set ACI_REGION")
	}

	p.cpu = "20"
	p.memory = "100Gi"
	p.cinstances = "20"

	//If a single image is given, create a basic worker instance
	if image != "" {
		cg, err := GetSingleImageContainerGroup(image, region, operatingSystem)
		if err != nil {
			return nil, err
		}
		p.workerInstance = cg
	} else if deploymentFile != "" {
		return nil, errors.New("Currently do not support K8s deployment files.")
		// deployment, err := GetDeploymentFromFile(deploymentFile)
		// if err != nil{
		// 	return nil, err
		// }
		// cg, err := GetACIFromK8sPod(pod, region, operatingSystem)
		// p.workerInstance = cg

		// // Make sure the KIRIX_WORK env varible was added to one container
		// err = p.AddWork("")
		// if err != nil {
		// 	return nil, err
		// }
	} else {
		return nil, errors.New("Must supply either an image or Kubernetes deployment spec.")
	}

	return &p, err
}

func CreateACIClient() (*aci.Client, error) {
	var azAuth *client.Authentication

	if authFilepath := os.Getenv("AZURE_AUTH_LOCATION"); authFilepath != "" {
		auth, err := client.NewAuthenticationFromFile(authFilepath)
		if err != nil {
			return nil, err
		}

		azAuth = auth
	} else {
		azAuth = GetDefaultAzureAuthentication()
	}

	if clientID := os.Getenv("AZURE_CLIENT_ID"); clientID != "" {
		azAuth.ClientID = clientID
	}

	if clientSecret := os.Getenv("AZURE_CLIENT_SECRET"); clientSecret != "" {
		azAuth.ClientSecret = clientSecret
	}

	if tenantID := os.Getenv("AZURE_TENANT_ID"); tenantID != "" {
		azAuth.TenantID = tenantID
	}

	if subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID"); subscriptionID != "" {
		azAuth.SubscriptionID = subscriptionID
	}

	if azAuth.TenantID == "" ||
		azAuth.SubscriptionID == "" ||
		azAuth.ClientSecret == "" ||
		azAuth.ClientID == "" {
		return nil, errors.New("Must have AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID set.")
	}

	client, err := aci.NewClient(azAuth)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func GetDefaultAzureAuthentication() *client.Authentication {
	return &client.Authentication{
		ActiveDirectoryEndpoint: "https://login.microsoftonline.com",
		ResourceManagerEndpoint: "https://management.azure.com/",
		GraphResourceID:         "https://graph.windows.net/",
		SQLManagementEndpoint:   "https://management.core.windows.net:8443/",
		GalleryEndpoint:         "https://gallery.azure.com/",
		ManagementEndpoint:      "https://management.core.windows.net/",
	}
}

func (p *ACIProvider) CreateComputeInstance(name string, work string) error {
	p.AddWorkToWorkerInstance(work)

	fmt.Printf("Createing ACI: %s\n", name)
	_, err := p.aciClient.CreateContainerGroup(
		p.resourceGroup,
		name,
		*p.workerInstance,
	)

	return err
}

func (p *ACIProvider) DeleteComputeInstance(name string) error {
	fmt.Printf("Deleting ACI: %s\n", name)
	return p.aciClient.DeleteContainerGroup(p.resourceGroup, name)
}

func (p *ACIProvider) SendWork(name string) error {
	return fmt.Errorf("Not Implemented")
}

func (p *ACIProvider) GetComputeInstance(name string) (*types.ComputeInstance, error) {
	cg, err, _ := p.aciClient.GetContainerGroup(p.resourceGroup, name)
	if err != nil {
		return nil, err
	}

	return &types.ComputeInstance{
		Name:  cg.Name,
		State: ConvertState(cg.InstanceView.State),
	}, nil
}

func (p *ACIProvider) GetCurrentComputeInstances() ([]types.ComputeInstance, error) {
	cgs, err := p.aciClient.ListContainerGroups(p.resourceGroup)
	if err != nil {
		return nil, err
	}

	computeInstances := make([]types.ComputeInstance, len(cgs.Value))

	for idx, cg := range cgs.Value {

		computeInstances[idx] = types.ComputeInstance{
			Name:  cg.Name,
			State: ConvertState(cg.InstanceView.State),
		}
	}

	return computeInstances, nil
}

func ConvertState(state string) string {
	switch state {
	case "Running":
		return types.StateInProgress
	case "Succeeded":
		return types.StateComplete
	case "Failed":
		return types.StateComplete
	default:
		return types.StateInProgress
	}
}

func (p *ACIProvider) AddWorkToWorkerInstance(work string) error {
	// Kirix Work Env is already set up
	for idx, container := range p.workerInstance.Containers {
		for envIdx, envVar := range container.EnvironmentVariables {
			if envVar.Name == WorkEnvVarName {
				p.workerInstance.Containers[idx].EnvironmentVariables[envIdx].Value = work
				return nil
			}
		}
	}

	return fmt.Errorf("Could not find Env Variable: %s to add work.", WorkEnvVarName)
}

func GetSingleImageContainerGroup(image string, region string, operatingSystem string) (*aci.ContainerGroup, error) {
	var containerGroup aci.ContainerGroup
	containerGroup.Location = region
	containerGroup.RestartPolicy = aci.ContainerGroupRestartPolicy("Always")
	containerGroup.ContainerGroupProperties.OsType = aci.OperatingSystemTypes(operatingSystem)

	// TODO: Allow private repos

	container := aci.Container{
		Name: "worker-container",
		ContainerProperties: aci.ContainerProperties{
			Image: image,
			Ports: make([]aci.ContainerPort, 0),
			EnvironmentVariables: []aci.EnvironmentVariable{
				aci.EnvironmentVariable{
					Name:  WorkEnvVarName,
					Value: "",
				}},
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

func GetACIFromK8sPod(pod *v1.Pod, region string, operatingSystem string) (*aci.ContainerGroup, error) {
	var containerGroup aci.ContainerGroup
	containerGroup.Location = region
	containerGroup.Name = pod.Name
	containerGroup.Type = "Microsoft.ContainerInstance/containerGroups"
	containerGroup.RestartPolicy = aci.ContainerGroupRestartPolicy(pod.Spec.RestartPolicy)
	containerGroup.ContainerGroupProperties.OsType = aci.OperatingSystemTypes(operatingSystem)

	// get containers
	containers, err := getContainers(pod)
	if err != nil {
		return nil, err
	}

	// get volumes
	volumes, err := getVolumes(pod)
	if err != nil {
		return nil, err
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

	return &containerGroup, err
}

func getVolumes(pod *v1.Pod) ([]aci.Volume, error) {
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

func getContainers(pod *v1.Pod) ([]aci.Container, error) {
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

		if cpuLimit == 0 {
			cpuLimit = 1
		}

		if memoryLimit == 0 {
			memoryLimit = 1
		}

		if cpuRequest == 0 {
			cpuRequest = 1
		}

		if memoryRequest == 0 {
			memoryRequest = 1
		}

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
