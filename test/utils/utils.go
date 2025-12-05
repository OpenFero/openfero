/*
Copyright 2025 OpenFero.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,staticcheck
)

const (
	kindBinary  = "kind"
	kindCluster = "openfero-e2e"
)

// Run executes a command and returns its output
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = GinkgoWriter.Write([]byte("chdir dir: " + err.Error()))
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = GinkgoWriter.Write([]byte("running: " + command + "\n"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

// GetProjectDir returns the project root directory
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	wd = strings.Replace(wd, "/test/utils", "", -1)
	return wd, nil
}

// GetNonEmptyLines returns non-empty lines from a string
func GetNonEmptyLines(output string) []string {
	var result []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// LoadImageToKindClusterWithName loads an image to the Kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := kindCluster
	if c := os.Getenv("KIND_CLUSTER"); c != "" {
		cluster = c
	}
	kindPath := kindBinary
	if k := os.Getenv("KIND"); k != "" {
		kindPath = k
	}
	cmd := exec.Command(kindPath, "load", "docker-image", name, "--name", cluster)
	_, err := Run(cmd)
	return err
}

// KindClusterExists checks if the Kind cluster exists
func KindClusterExists() bool {
	kindPath := kindBinary
	if k := os.Getenv("KIND"); k != "" {
		kindPath = k
	}
	cmd := exec.Command(kindPath, "get", "clusters")
	output, err := Run(cmd)
	if err != nil {
		return false
	}
	cluster := kindCluster
	if c := os.Getenv("KIND_CLUSTER"); c != "" {
		cluster = c
	}
	return strings.Contains(output, cluster)
}

// CreateKindCluster creates a Kind cluster
func CreateKindCluster() error {
	kindPath := kindBinary
	if k := os.Getenv("KIND"); k != "" {
		kindPath = k
	}
	cluster := kindCluster
	if c := os.Getenv("KIND_CLUSTER"); c != "" {
		cluster = c
	}
	cmd := exec.Command(kindPath, "create", "cluster", "--name", cluster)
	_, err := Run(cmd)
	return err
}

// DeleteKindCluster deletes the Kind cluster
func DeleteKindCluster() error {
	kindPath := kindBinary
	if k := os.Getenv("KIND"); k != "" {
		kindPath = k
	}
	cluster := kindCluster
	if c := os.Getenv("KIND_CLUSTER"); c != "" {
		cluster = c
	}
	cmd := exec.Command(kindPath, "delete", "cluster", "--name", cluster)
	_, err := Run(cmd)
	return err
}

// ApplyYAML applies YAML content via kubectl
func ApplyYAML(yaml string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(yaml)
	_, err := Run(cmd)
	return err
}

// DeleteYAML deletes resources from YAML content via kubectl
func DeleteYAML(yaml string) error {
	cmd := exec.Command("kubectl", "delete", "-f", "-", "--ignore-not-found")
	cmd.Stdin = bytes.NewBufferString(yaml)
	_, err := Run(cmd)
	return err
}
