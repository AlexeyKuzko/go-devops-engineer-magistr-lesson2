package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Pod struct {
	APIVersion string     `yaml:"APIVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   ObjectMeta `yaml:"metadata"`
	Spec       PodSpec    `yaml:"spec"`
}

type ObjectMeta struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type PodSpec struct {
	OS         string      `yaml:"os"`
	Containers []Container `yaml:"containers"`
}

type Container struct {
	Name           string               `yaml:"name"`
	Image          string               `yaml:"image"`
	Ports          []ContainerPort      `yaml:"ports,omitempty"`
	ReadinessProbe Probe                `yaml:"readinessProbe,omitempty"`
	LivenessProbe  Probe                `yaml:"livenessProbe,omitempty"`
	Resources      ResourceRequirements `yaml:"resources"`
}

type ContainerPort struct {
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty"`
}

type Probe struct {
	HTTPGet HTTPGetAction `yaml:"HTTPGet"`
}

type HTTPGetAction struct {
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

type ResourceRequirements struct {
	Limits   map[string]string `yaml:"limits"`
	Requests map[string]string `yaml:"requests,omitempty"`
}

func validatePod(pod Pod, fileName string) []string {
	var errors []string

	if pod.APIVersion != "v1" {
		errors = append(errors, fmt.Sprintf("%s: apiVersion has unsupported value '%s'", fileName, pod.APIVersion))
	}

	if pod.Kind != "Pod" {
		errors = append(errors, fmt.Sprintf("%s: kind must be 'Pod', but got '%s'", fileName, pod.Kind))
	}

	if pod.Metadata.Name == "" {
		errors = append(errors, fmt.Sprintf("%s: metadata.name is required", fileName))
	}

	validOS := []string{"linux", "windows", "darwin"}
	if !contains(validOS, pod.Spec.OS) {
		errors = append(errors, fmt.Sprintf("%s: os has unsupported value '%s'", fileName, pod.Spec.OS))
	}

	for _, container := range pod.Spec.Containers {
		if container.Name == "" {
			errors = append(errors, fmt.Sprintf("%s: container.name is required", fileName))
		}
		if !strings.HasPrefix(container.Image, "registry.bigbrother.io") {
			errors = append(errors, fmt.Sprintf("%s: container.image must be from domain 'registry.bigbrother.io', but got '%s'", fileName, container.Image))
		}
		if cpuLimit, ok := container.Resources.Limits["cpu"]; ok {
			if _, err := strconv.Atoi(cpuLimit); err != nil {
				errors = append(errors, fmt.Sprintf("%s: cpu must be int", fileName))
			}
		}
	}

	return errors
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: yamlvalid <file>")
		os.Exit(1)
	}

	fileName := os.Args[1]
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read file %s: %v\n", fileName, err)
		os.Exit(1)
	}

	var pod Pod
	if err := yaml.Unmarshal(content, &pod); err != nil {
		fmt.Fprintf(os.Stderr, "cannot unmarshal file %s: %v\n", fileName, err)
		os.Exit(1)
	}

	errors := validatePod(pod, fileName)
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	fmt.Println("YAML validation passed.")
	os.Exit(0)
}
