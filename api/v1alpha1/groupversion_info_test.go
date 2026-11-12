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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersOperariusTypes(t *testing.T) {
	sch := runtime.NewScheme()
	if err := AddToScheme(sch); err != nil {
		t.Fatalf("AddToScheme failed: %v", err)
	}

	for _, kind := range []string{"Operarius", "OperariusList"} {
		gvk := GroupVersion.WithKind(kind)
		if !sch.Recognizes(gvk) {
			t.Errorf("scheme does not recognize %v", gvk)
		}
		if _, err := sch.New(gvk); err != nil {
			t.Errorf("scheme.New(%v) failed: %v", gvk, err)
		}
	}
}
