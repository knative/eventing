package testing

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockGet func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (bool, error)
type MockList func(innerClient client.Client, ctx context.Context, opts *client.ListOptions, list runtime.Object) (bool, error)
type MockCreate func(innerClient client.Client, ctx context.Context, obj runtime.Object) (bool, error)
type MockDelete func(innerClient client.Client, ctx context.Context, obj runtime.Object) (bool, error)
type MockUpdate func(innerClient client.Client, ctx context.Context, obj runtime.Object) (bool, error)

type mockClient struct {
	client client.Client
	mocks  Mocks
}

type Mocks struct {
	MockGets []MockGet
	MockLists []MockList
	MockCreates []MockCreate
	MockDeletes []MockDelete
	MockUpdates []MockUpdate
}

func NewMockClient(innerClient client.Client, mocks Mocks) client.Client {
	return &mockClient{
		client: innerClient,
		mocks: mocks,
	}
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	for _, mockGet := range m.mocks.MockGets {
		if complete, err := mockGet(m.client, ctx, key, obj); complete {
			return err
		}
	}
	return m.client.Get(ctx, key, obj)
}

func (m *mockClient) List(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
	for _, mockList := range m.mocks.MockLists {
		if complete, err := mockList(m.client, ctx, opts, list); complete {
			return err
		}
	}
	return m.client.List(ctx, opts, list)
}

func (m *mockClient) Create(ctx context.Context, obj runtime.Object) error {
	for _, mockCreate := range m.mocks.MockCreates {
		if complete, err := mockCreate(m.client, ctx, obj); complete {
			return err
		}
	}
	return m.client.Create(ctx, obj)
}

func (m *mockClient) Delete(ctx context.Context, obj runtime.Object) error {
	for _, mockDelete := range m.mocks.MockDeletes {
		if complete, err := mockDelete(m.client, ctx, obj); complete {
			return err
		}
	}
	return m.client.Delete(ctx, obj)
}

func (m *mockClient) Update(ctx context.Context, obj runtime.Object) error {
	for _, mockUpdate := range m.mocks.MockUpdates {
		if complete, err := mockUpdate(m.client, ctx, obj); complete {
			return err
		}
	}
	return m.client.Update(ctx, obj)
}
