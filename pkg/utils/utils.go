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
)

const (
	resolverFileName  = "/etc/resolv.conf"
	defaultDomainName = "cluster.local"
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

func ToDNS1123Subdomain(name string) string {
	// If it is not a valid DNS1123 subdomain, make it a valid one.
	if msgs := validation.IsDNS1123Subdomain(name); len(msgs) != 0 {
		// If the length exceeds the max, cut it and leave some room for the generated UUID.
		if len(name) > validation.DNS1123SubdomainMaxLength {
			name = name[:validation.DNS1123SubdomainMaxLength-10]
		}
		name = strings.ToLower(name)
		name = validChars.ReplaceAllString(name, "")
		// Only start/end with alphanumeric.
		name = strings.Trim(name, "-.")
	}
	return name
}
