package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Структуры для валидации полей YAML-файла
type Pod struct {
	APIVersion string     `yaml:"apiVersion"`
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
	HTTPGet HTTPGetAction `yaml:"httpGet"`
}

type HTTPGetAction struct {
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

type ResourceRequirements struct {
	Limits   ResourceLimits `yaml:"limits"`
	Requests ResourceLimits `yaml:"requests,omitempty"`
}

type ResourceLimits struct {
	CPU    interface{} `yaml:"cpu"`
	Memory string      `yaml:"memory"`
}

func main() {
	// Получаем путь к файлу через флаг
	filePath := flag.String("file", "", "Path to YAML file")
	flag.Parse()

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "YAML file path is required")
		os.Exit(1)
	}

	// Читаем содержимое файла
	content, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read file content: %v\n", err)
		os.Exit(1)
	}

	// Парсим YAML в структуру Pod
	var pod Pod
	err = yaml.Unmarshal(content, &pod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot unmarshal file content: %v\n", err)
		os.Exit(1)
	}

	// Выполняем валидацию полей
	if err := validatePod(pod); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", *filePath, err)
		os.Exit(1)
	}

	fmt.Println("YAML file is valid")
	os.Exit(0)
}

func validatePod(pod Pod) error {
	// Проверка apiVersion
	if pod.APIVersion != "v1" {
		return fmt.Errorf("apiVersion has unsupported value '%s'", pod.APIVersion)
	}

	// Проверка kind
	if pod.Kind != "Pod" {
		return fmt.Errorf("kind has unsupported value '%s'", pod.Kind)
	}

	// Проверка обязательных полей в metadata
	if pod.Metadata.Name == "" {
		return errors.New("metadata.name is required")
	}

	// Проверка os
	if pod.Spec.OS != "linux" && pod.Spec.OS != "windows" {
		return fmt.Errorf("spec.os has unsupported value '%s'", pod.Spec.OS)
	}

	// Проверка containers
	if len(pod.Spec.Containers) == 0 {
		return errors.New("spec.containers is required")
	}

	for _, container := range pod.Spec.Containers {
		if err := validateContainer(container); err != nil {
			return err
		}
	}

	return nil
}

func validateContainer(container Container) error {
	// Проверка имени контейнера
	snakeCaseRegex := `^[a-z0-9_]+$`
	matched, _ := regexp.MatchString(snakeCaseRegex, container.Name)
	if !matched {
		return fmt.Errorf("containers.name has invalid format '%s'", container.Name)
	}

	// Проверка образа контейнера
	imageRegex := `^registry\.bigbrother\.io\/.+:[a-zA-Z0-9_.-]+$`
	matched, _ = regexp.MatchString(imageRegex, container.Image)
	if !matched {
		return fmt.Errorf("containers.image has invalid format '%s'", container.Image)
	}

	// Проверка каждого порта в списке
	for _, port := range container.Ports {
		if err := validateContainerPort(port); err != nil {
			return err
		}
	}

	// Проверка ресурсов контейнера
	if err := validateResources(container.Resources); err != nil {
		return err
	}

	// Проверка readinessProbe
	if err := validateProbe(container.ReadinessProbe, "readinessProbe"); err != nil {
		return err
	}

	// Проверка livenessProbe
	if err := validateProbe(container.LivenessProbe, "livenessProbe"); err != nil {
		return err
	}

	return nil
}

func validateContainerPort(port ContainerPort) error {
	// Проверка диапазона порта (1-65535)
	if port.ContainerPort < 1 || port.ContainerPort > 65535 {
		return fmt.Errorf("containerPort value out of range: %d", port.ContainerPort)
	}

	if port.Protocol != "" && port.Protocol != "TCP" && port.Protocol != "UDP" {
		return fmt.Errorf("protocol has unsupported value '%s'", port.Protocol)
	}
	return nil
}

func validateProbe(probe Probe, probeName string) error {
	if probe.HTTPGet.Port <= 0 || probe.HTTPGet.Port >= 65536 {
		return fmt.Errorf("%s.httpGet.port value out of range", probeName)
	}
	return nil
}

func validateResources(resources ResourceRequirements) error {
	// Проверка лимитов CPU
	switch v := resources.Limits.CPU.(type) {
	case int:
		if v <= 0 {
			return errors.New("resources.limits.cpu is required and must be positive")
		}
	case string:
		// Если значение представлено как строка, проверяем, что это валидное целое число
		cpuRegex := `^\d+$`
		matched, _ := regexp.MatchString(cpuRegex, v)
		if !matched {
			return fmt.Errorf("resources.limits.cpu has invalid format '%s'", v)
		}
	default:
		return errors.New("resources.limits.cpu has invalid type")
	}

	// Проверка лимитов памяти
	if resources.Limits.Memory == "" {
		return errors.New("resources.limits.memory is required")
	}

	memoryRegex := `^\d+(Ki|Mi|Gi)$`
	matched, _ := regexp.MatchString(memoryRegex, resources.Limits.Memory)
	if !matched {
		return fmt.Errorf("resources.limits.memory has invalid format '%s'", resources.Limits.Memory)
	}

	return nil
}
