package v1alpha1

import "context"

func (et *EventTypeDefinition) SetDefaults(ctx context.Context) {
	et.Spec.SetDefaults(ctx)
}

func (ets *EventTypeDefinitionSpec) SetDefaults(ctx context.Context) {
}
