package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	absPath string
	relPath string
)

type Pod struct {
	ApiVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type Spec struct {
	OS         string      `yaml:"os"`
	Containers []Container `yaml:"containers"`
}

type Container struct {
	Name           string   `yaml:"name"`
	Image          string   `yaml:"image"`
	Ports          []Port   `yaml:"ports,omitempty"`
	ReadinessProbe Probe    `yaml:"readinessProbe,omitempty"`
	LivenessProbe  Probe    `yaml:"livenessProbe,omitempty"`
	Resources      Resource `yaml:"resources,omitempty"`
}

type Port struct {
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty"`
}

type Probe struct {
	HTTPGet HTTPGet `yaml:"httpGet,omitempty"`
}

type HTTPGet struct {
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

type Resource struct {
	Limits   ResourceLimits `yaml:"limits,omitempty"`
	Requests ResourceLimits `yaml:"requests,omitempty"`
}

type ResourceLimits struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

func init() {
	if len(os.Args[1:]) != 1 {
		panic("path to yaml is not provided")
	}
	filePath := os.Args[1]
	_, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		panic(fmt.Sprintf("%s does not exist", filePath))
	}
	absPath, _ = filepath.Abs(filePath)
	parentDir := filepath.Dir(filePath)
	relPath, _ = filepath.Rel(parentDir, filePath)
}

func main() {
	// Read the YAML file content
	data, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read file: %v\n", err)
		os.Exit(1)
	}

	// Unmarshal the YAML content into the Pod struct
	var pod Pod
	err = yaml.Unmarshal(data, &pod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot unmarshal file content: %v\n", err)
		os.Exit(1)
	}

	// Validate the Pod struct
	err = validatePod(&pod)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%v\n", err)
		os.Exit(1)
	}

	// If everything is valid
	fmt.Println("YAML file is valid")
}

func validatePod(pod *Pod) error {
	var validationErrors []string

	// Validate apiVersion
	if pod.ApiVersion != "v1" {
		validationErrors = append(validationErrors, fmt.Sprintf("%s apiVersion must be v1", relPath))
	}

	// Validate kind
	if pod.Kind != "Pod" {
		validationErrors = append(validationErrors, fmt.Sprintf("%s kind must be Pod", relPath))
	}

	// Validate metadata.name
	if len(pod.Metadata.Name) == 0 {
		validationErrors = append(validationErrors, fmt.Sprintf("%s name is required", relPath))
	}

	// Validate spec.os
	validOSValues := map[string]bool{"linux": true, "windows": true}
	if !validOSValues[pod.Spec.OS] {
		validationErrors = append(validationErrors, fmt.Sprintf("%s os has unsupported value '%s'", relPath, pod.Spec.OS))
	}

	// Validate containers
	if len(pod.Spec.Containers) == 0 {
		validationErrors = append(validationErrors, fmt.Sprintf("%s: containers is required", relPath))
	}

	for _, container := range pod.Spec.Containers {
		// Validate container name
		if strings.TrimSpace(container.Name) == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("%s name is required", relPath))
		}

		// Validate container image
		if container.Image == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("%s container.image is required", relPath))
		}

		// Validate container ports
		if len(container.Ports) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("%s container must define at least one port", relPath))
		}

		for _, port := range container.Ports {
			if err := validatePort(port); err != nil {
				validationErrors = append(validationErrors, err.Error())
			}
		}

		// Validate readiness and liveness probes
		if err := validateProbe(container.ReadinessProbe, "readinessProbe"); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
		if err := validateProbe(container.LivenessProbe, "livenessProbe"); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}

		// Validate resources
		if err := validateResources(container.Resources); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
	}

	// Return all validation errors if any
	if len(validationErrors) > 0 {
		return errors.New(strings.Join(validationErrors, "\n"))
	}

	return nil
}

func validatePort(port Port) error {
	// Validate container port range
	if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
		return fmt.Errorf("%s: containerPort must be in the range (0, 65535], found %d", relPath, port.ContainerPort)
	}

	// Validate protocol
	if port.Protocol != "" && port.Protocol != "TCP" && port.Protocol != "UDP" {
		return fmt.Errorf("%s: unsupported protocol '%s', must be TCP or UDP", relPath, port.Protocol)
	}

	return nil
}

func validateProbe(probe Probe, probeType string) error {
	if probe.HTTPGet.Port <= 0 || probe.HTTPGet.Port > 65535 {
		return fmt.Errorf("%s: port value out of range", relPath)
	}
	return nil
}

func validateResources(resources Resource) error {
	if resources.Limits.CPU != "" {
		if _, err := validateCPU(resources.Limits.CPU); err != nil {
			return fmt.Errorf("%s: cpu %s", relPath, err.Error())
		}
	}
	if resources.Requests.CPU != "" {
		if _, err := validateCPU(resources.Requests.CPU); err != nil {
			return fmt.Errorf("%s: cpu %s", relPath, err.Error())
		}
	}
	return nil
}

func validateCPU(cpu interface{}) (int, error) {
	var i int
	switch cpu := cpu.(type) {
	case int:
		return cpu, nil
	case string:
		var err error
		i, err = strconv.Atoi(cpu)
		if err != nil {
			return 0, errors.New("must be int")
		}
		return i, nil
	default:
		return 0, errors.New("must be int")
	}
}
