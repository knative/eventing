package v1alpha1

//import "knative.dev/pkg/apis"
//
//var eventTypeDefCondSet = apis.NewLivingConditionSet(EventTypeDefinitionConditionReady)
//
//const (
//	EventTypeDefinitionConditionReady = apis.ConditionReady
//)
//
//// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
//func (*EventTypeDefinition) GetConditionSet() apis.ConditionSet {
//	return eventTypeDefCondSet
//}
//
//// GetCondition returns the condition currently associated with the given type, or nil.
//func (etds *EventTypeDefinitionStatus) GetCondition(t apis.ConditionType) *apis.Condition {
//	return eventTypeDefCondSet.Manage(etds).GetCondition(t)
//}
//
//// IsReady returns true if the resource is ready overall.
//func (etds *EventTypeDefinitionStatus) IsReady() bool {
//	return eventTypeDefCondSet.Manage(etds).IsHappy()
//}
//
//// GetTopLevelCondition returns the top level Condition.
//func (etds *EventTypeDefinitionStatus) GetTopLevelCondition() *apis.Condition {
//	return eventTypeDefCondSet.Manage(etds).GetTopLevelCondition()
//}
//
//// InitializeConditions sets relevant unset conditions to Unknown state.
//func (etds *EventTypeDefinitionStatus) InitializeConditions() {
//	eventTypeDefCondSet.Manage(etds).InitializeConditions()
//}
//
//func (etds *EventTypeDefinitionStatus) MarkReady() {
//	eventTypeDefCondSet.Manage(etds).MarkTrue(EventTypeDefinitionConditionReady)
//}
