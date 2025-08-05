/*
Copyright 2025.

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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestParseGVK(t *testing.T) {
	testCases := []struct {
		name          string
		gvkString     string
		expectedGVK   schema.GroupVersionKind
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid deployment gvk",
			gvkString:   "apps/v1, Kind=Deployment",
			expectedGVK: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expectError: false,
		},
		{
			name:        "valid service gvk",
			gvkString:   "v1, Kind=Service",
			expectedGVK: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			expectError: false,
		},
		{
			name:          "invalid format - missing kind",
			gvkString:     "apps/v1",
			expectError:   true,
			errorContains: "invalid GVK string format",
		},
		{
			name:          "invalid format - extra parts",
			gvkString:     "apps/v1, Kind=Deployment, Other=Value",
			expectError:   true,
			errorContains: "invalid GVK string format",
		},
		{
			name:          "invalid groupversion",
			gvkString:     "apps/v1/extra, Kind=Deployment",
			expectError:   true,
			errorContains: "invalid GroupVersion string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gvk, err := parseGVK(tc.gvkString)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedGVK, gvk)
			}
		})
	}
}
