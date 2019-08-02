/*
Copyright 2018 The Knative Authors

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
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/validation"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	resolverFileName  = "/etc/resolv.conf"
	defaultDomainName = "cluster.local"

	// Number of characters to keep available just in case the prefix used in generateName
	// exceeds the maximum allowed for k8s names.
	generateNameSafety = 10
)

var (
	domainName string
	once       sync.Once

	// Only allow alphanumeric, '-' or '.'.
	validChars = regexp.MustCompile(`[^-\.a-z0-9]+`)
)

// GetClusterDomainName returns cluster's domain name or an error
// Closes issue: https://github.com/knative/eventing/issues/714
func GetClusterDomainName() string {
	once.Do(func() {
		f, err := os.Open(resolverFileName)
		if err == nil {
			defer f.Close()
			domainName = getClusterDomainName(f)

		} else {
			domainName = defaultDomainName
		}
	})

	return domainName
}

func getClusterDomainName(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		elements := strings.Split(scanner.Text(), " ")
		if elements[0] != "search" {
			continue
		}
		for i := 1; i < len(elements)-1; i++ {
			if strings.HasPrefix(elements[i], "svc.") {
				return elements[i][4:]
			}
		}
	}
	// For all abnormal cases return default domain name
	return defaultDomainName
}

func ObjectRef(obj metav1.Object, gvk schema.GroupVersionKind) corev1.ObjectReference {
	// We can't always rely on the TypeMeta being populated.
	// See: https://github.com/knative/serving/issues/2372
	// Also: https://github.com/kubernetes/apiextensions-apiserver/issues/29
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	return corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
	}
}

// ToDNS1123Subdomain converts 'name' to a valid DNS1123 subdomain, required for object names in
// K8s.
func ToDNS1123Subdomain(name string) string {
	// If it is not a valid DNS1123 subdomain, make it a valid one.
	if msgs := validation.IsDNS1123Subdomain(name); len(msgs) != 0 {
		// If the length exceeds the max, cut it and leave some room for a potential generated UUID.
		if len(name) > validation.DNS1123SubdomainMaxLength {
			name = name[:validation.DNS1123SubdomainMaxLength-generateNameSafety]
		}
		name = strings.ToLower(name)
		name = validChars.ReplaceAllString(name, "")
		// Only start/end with alphanumeric.
		name = strings.Trim(name, "-.")
	}
	return name
}

// GenerateFixedName generates a fixed name for the given owning resource and human readable prefix.
// The name's length will be short enough to be valid for K8s Services.
func GenerateFixedName(owner metav1.Object, prefix string) string {
	uid := string(owner.GetUID())
	// Make sure the UID is separated from the prefix by a leading dash.
	if !strings.HasPrefix(uid, "-") {
		uid = "-" + uid
	}
	// Trim any trailing dash from the prefix as the UID is now prepended with the dash.
	prefix = strings.TrimSuffix(prefix, "-")
	pl := validation.DNS1123LabelMaxLength - len(uid)
	if pl > len(prefix) {
		pl = len(prefix)
	}
	return prefix[:pl] + uid
}
