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

package environment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/images/ko"
)

var (
	// defaultImageProducer is the function that will be used to produce the
	// container images by default.
	//
	// To use a different image producer pass a different ImageProducer
	// when creating an Environment through GlobalEnvironment
	// see WithImageProducer.
	defaultImageProducer = ImageProducer(ko.Publish)

	// produceImagesLock is used to ensure that ProduceImages is only called
	// once at the time.
	produceImagesLock = sync.Mutex{}
)

// parallelQueueSize is the max number of packages at one time in queue to be
// consumed by image producer.
const parallelQueueSize = 1_000

// ImageProducer is a function that will be used to produce the container images.
//
// pack is a Go main package reference like `knative.dev/reconciler-test/cmd/eventshub`.
type ImageProducer func(ctx context.Context, pack string) (string, error)

// RegisterPackage registers an interest in producing an image based on the
// provided package.
// Can be called multiple times with the same package.
// A package will be used to produce the image and used
// like `image: ko://<package>` inside test yaml.
func RegisterPackage(pack ...string) EnvOpts {
	return func(ctx context.Context, _ Environment) (context.Context, error) {
		rk := registeredPackagesKey{}
		rk.register(ctx, pack)
		store := rk.get(ctx)
		return context.WithValue(ctx, rk, store), nil
	}
}

// WithImages will bypass ProduceImages() and use the provided image set
// instead. Should be called before ProduceImages(), if used, likely in an
// init() method. An images value should be a container registry image. The
// images map is presented to the templates on the field `images`, and used
// like `image: <key>` inside test yaml.
func WithImages(given map[string]string) EnvOpts {
	return func(ctx context.Context, _ Environment) (context.Context, error) {
		ik := imageStoreKey{}
		store := ik.new()
		store.refs = given
		return context.WithValue(ctx, ik, store), nil
	}
}

// ProduceImages returns back the packages that have been added.
// Will produce images once, can be called many times.
func ProduceImages(ctx context.Context) (map[string]string, error) {
	produceImagesLock.Lock()
	defer produceImagesLock.Unlock()
	rk := registeredPackagesKey{}
	ik := imageStoreKey{}
	store := ik.get(ctx)

	ip := GetImageProducer(ctx)

	for _, pack := range rk.packages(ctx) {
		koPack := fmt.Sprintf("ko://%s", pack)
		if store.refs[koPack] != "" {
			continue
		}
		image, err := ip(ctx, pack)
		if errors.Is(err, ko.ErrKoPublishFailed) {
			logging.FromContext(ctx).Warnw("Ko publish failed, using image directly", "error", err, "image", pack)
			image = pack
			err = nil
		}
		if err != nil {
			return nil, err
		}
		store.refs[koPack] = strings.TrimSpace(image)
	}
	return store.copyRefs(), nil
}

func initializeImageStores(ctx context.Context) context.Context {
	var emptyPkgs []string
	emptyImgs := make(map[string]string)
	mctx, err := UnionOpts(
		RegisterPackage(emptyPkgs...),
		WithImages(emptyImgs),
	)(ctx, nil)
	if err != nil {
		logging.FromContext(ctx).
			Fatal("Failed to initialize image stores: ", err)
	}
	return mctx
}

type registeredPackagesKey struct{}

type packagesStore struct {
	refs chan string
}

func (k registeredPackagesKey) get(ctx context.Context) *packagesStore {
	if registered, ok := ctx.Value(k).(*packagesStore); ok {
		return registered
	}
	return &packagesStore{
		refs: make(chan string, parallelQueueSize),
	}
}

func (k registeredPackagesKey) packages(ctx context.Context) []string {
	store := k.get(ctx)
	refs := make([]string, 0)
	for {
		select {
		case ref := <-store.refs:
			refs = append(refs, ref)
		default:
			return refs
		}
	}
}

func (k registeredPackagesKey) register(ctx context.Context, packs []string) {
	store := k.get(ctx)
	for _, pack := range packs {
		pack = strings.TrimPrefix(pack, "ko://")
		store.refs <- pack
	}
}

type imageStoreKey struct{}

func (k imageStoreKey) new() *imageStore {
	return &imageStore{
		refs: make(map[string]string),
	}
}

func (k imageStoreKey) get(ctx context.Context) *imageStore {
	if i, ok := ctx.Value(k).(*imageStore); ok {
		return i
	}
	return k.new()
}

type imageStore struct {
	refs map[string]string
}

func (is imageStore) copyRefs() map[string]string {
	refs := make(map[string]string, len(is.refs))
	for k, v := range is.refs {
		refs[k] = v
	}
	return refs
}

// imageProducerKey is the key for the ImageProducer context value.
type imageProducerKey struct{}

// WithImageProducer allows using a different ImageProducer
// when creating an Environment through GlobalEnvironment.
// Example usage:
// GlobalEnvironment.Environment(WithImageProducer(file.ImageProducer("images.yaml")))
func WithImageProducer(producer ImageProducer) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		return withImageProducer(ctx, producer), nil
	}
}

func withImageProducer(ctx context.Context, producer ImageProducer) context.Context {
	return context.WithValue(ctx, imageProducerKey{}, producer)
}

// GetImageProducer extracts an ImageProducer from the given context.
func GetImageProducer(ctx context.Context) ImageProducer {
	p := ctx.Value(imageProducerKey{})
	if p == nil {
		return defaultImageProducer
	}
	return p.(ImageProducer)
}
