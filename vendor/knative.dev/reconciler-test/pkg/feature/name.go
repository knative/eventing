/*
Copyright 2020 The Knative Authors

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

package feature

import (
	"math/rand"
	"strings"
	"time"
	"unicode"

	"knative.dev/pkg/kmeta"
)

const (
	letterBytes    = "abcdefghijklmnopqrstuvwxyz"
	randSuffixLen  = 8
	sep            = '-'
	testNamePrefix = "Test"
)

func init() {
	// Properly seed the random number generator so RandomString() is actually random.
	// Otherwise, rerunning tests will generate the same names for the test resources, causing conflicts with
	// already existing resources.
	seed := time.Now().UTC().UnixNano()
	rand.Seed(seed)
}

type Namer interface {
	Name() string
}

// ObjectPrefixForTest returns the Name prefix for this test's random names.
func ObjectPrefixForTest(t Namer) string {
	return MakeK8sNamePrefix(strings.TrimPrefix(t.Name(), testNamePrefix))
}

// ObjectNameForTest generates a random object Name based on the test Name.
func ObjectNameForTest(t Namer) string {
	return kmeta.ChildName(ObjectPrefixForTest(t), string(sep)+RandomString())
}

// AppendRandomString will generate a random string that begins with prefix.
// This is useful if you want to make sure that your tests can run at the same
// time against the same environment without conflicting.
// This method will use "-" as the separator between the prefix and
// the random suffix.
func AppendRandomString(prefix string) string {
	return strings.Join([]string{prefix, RandomString()}, string(sep))
}

// RandomString will generate a random string.
func RandomString() string {
	suffix := make([]byte, randSuffixLen)

	for i := range suffix {
		suffix[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(suffix)
}

// MakeRandomK8sName
func MakeRandomK8sName(prefix string) string {
	return MakeK8sNamePrefix(AppendRandomString(prefix))
}

// MakeK8sNamePrefix converts each chunk of non-alphanumeric character into a single dash
// and also convert camelcase tokens into dash-delimited lowercase tokens.
func MakeK8sNamePrefix(s string) string {
	var sb strings.Builder
	newToken := false
	for _, c := range s {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c)) {
			newToken = true
			continue
		}
		if sb.Len() > 0 && (newToken || unicode.IsUpper(c)) {
			sb.WriteRune(sep)
		}
		sb.WriteRune(unicode.ToLower(c))
		newToken = false
	}
	return sb.String()
}

// GetBaseFuncName returns the baseFuncName parsed from the fullFuncName.
// eg. test/e2e.TestMain will return TestMain.
func GetBaseFuncName(fullFuncName string) string {
	name := fullFuncName
	// Possibly there is no parent package, so only remove it from the Name if '/' exists
	if strings.ContainsRune(name, '/') {
		name = name[strings.LastIndex(name, "/")+1:]
	}
	name = name[strings.LastIndex(name, ".")+1:]
	return name
}
