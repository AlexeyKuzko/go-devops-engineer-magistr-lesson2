package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	absPath string
	relPath string
)

type Pod struct {
	APIVersion string   `yaml:"APIVersion"`
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
	CPU    interface{} `yaml:"cpu,omitempty"`
	Memory string      `yaml:"memory,omitempty"`
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
	err = validatePod(&pod, data)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%v\n", err)
		os.Exit(1)
	}
}

func validatePod(pod *Pod, data []byte) error {
	var validationErrors []string

	// Validate APIVersion
	if pod.APIVersion != "v1" {
		validationErrors = append(validationErrors, fmt.Sprintf("%s: APIVersion must be v1", relPath))
	}

	// Validate kind
	if pod.Kind != "Pod" {
		validationErrors = append(validationErrors, fmt.Sprintf("%s: kind must be Pod", relPath))
	}

	// Validate metadata.name
	if len(pod.Metadata.Name) == 0 {
		line := getLineNumber(data, "name")
		validationErrors = append(validationErrors, fmt.Sprintf("%s:%d: name is required", relPath, line))
	}

	// Validate spec.os
	validOSValues := map[string]bool{"linux": true, "windows": true}
	if !validOSValues[pod.Spec.OS] {
		line := getLineNumber(data, "os")
		validationErrors = append(validationErrors, fmt.Sprintf("%s:%d: os has unsupported value '%s'", relPath, line, pod.Spec.OS))
	}

	// Validate containers
	if len(pod.Spec.Containers) == 0 {
		validationErrors = append(validationErrors, fmt.Sprintf("%s: spec.containers is required", relPath))
	}

	for _, container := range pod.Spec.Containers {
		// Validate container name
		if strings.TrimSpace(container.Name) == "" {
			line := getLineNumber(data, "name")
			validationErrors = append(validationErrors, fmt.Sprintf("%s:%d: name is required", relPath, line))
		}

		// Validate container image
		if container.Image == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: container.image is required", relPath))
		}

		// Validate container ports
		if len(container.Ports) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: container must define at least one port", relPath))
		}

		for _, port := range container.Ports {
			if err := validatePort(port, data); err != nil {
				validationErrors = append(validationErrors, err.Error())
			}
		}

		// Validate readiness and liveness probes
		if err := validateProbe(container.ReadinessProbe, "readinessProbe", data); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
		if err := validateProbe(container.LivenessProbe, "livenessProbe", data); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}

		// Validate resources
		if err := validateResources(container.Resources, data); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
	}

	// Return all validation errors if any
	if len(validationErrors) > 0 {
		return errors.New(strings.Join(validationErrors, "\n"))
	}

	return nil
}

func validatePort(port Port, data []byte) error {
	// Validate container port range
	if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
		line := getLineNumber(data, "containerPort")
		return fmt.Errorf("%s:%d: containerPort value out of range", relPath, line)
	}

	return nil
}

func validateProbe(probe Probe, probeType string, data []byte) error {
	if probe.HTTPGet.Port <= 0 || probe.HTTPGet.Port > 65535 {
		line := getLineNumber(data, "port")
		return fmt.Errorf("%s:%d port value out of range", relPath, line)
	}
	return nil
}

func validateResources(resources Resource, data []byte) error {
	if resources.Requests.CPU != "" {
		if _, err := validateCPU(resources.Requests.CPU); err != nil {
			line := getLineNumber(data, "cpu")
			return fmt.Errorf("%s:%d: cpu %s", relPath, line, err.Error())
		}
	}
	return nil
}

func validateCPU(cpu interface{}) (int, error) {
	switch cpu := cpu.(type) {
	case int:
		return cpu, nil
	default:
		return 0, errors.New("must be int")
	}
}

func getLineNumber(data []byte, field string) int {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.Contains(line, field) {
			return i + 1 // +1 for 1-based index
		}
	}
	return -1 // Return -1 if field is
}
