/*
Copyright 2021 The Knative Authors

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

package state

import (
	"context"
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

const (
	PodFitsResources                   = "PodFitsResources"
	NoMaxResourceCount                 = "NoMaxResourceCount"
	EvenPodSpread                      = "EvenPodSpread"
	AvailabilityNodePriority           = "AvailabilityNodePriority"
	AvailabilityZonePriority           = "AvailabilityZonePriority"
	LowestOrdinalPriority              = "LowestOrdinalPriority"
	RemoveWithEvenPodSpreadPriority    = "RemoveWithEvenPodSpreadPriority"
	RemoveWithAvailabilityNodePriority = "RemoveWithAvailabilityNodePriority"
	RemoveWithAvailabilityZonePriority = "RemoveWithAvailabilityZonePriority"
	RemoveWithHighestOrdinalPriority   = "RemoveWithHighestOrdinalPriority"
)

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

type FilterPlugin interface {
	Plugin
	// Filter is called by the scheduler.
	// All FilterPlugins should return "Success" to declare that
	// the given pod fits the vreplica.
	Filter(ctx context.Context, args interface{}, state *State, key types.NamespacedName, podID int32) *Status
}

// ScoreExtensions is an interface for Score extended functionality.
type ScoreExtensions interface {
	// NormalizeScore is called for all pod scores produced by the same plugin's "Score"
	// method. A successful run of NormalizeScore will update the scores list and return
	// a success status.
	NormalizeScore(ctx context.Context, state *State, scores PodScoreList) *Status
}

type ScorePlugin interface {
	Plugin
	// Score is called by the scheduler.
	// All ScorePlugins should return "Success" unless the args are invalid.
	Score(ctx context.Context, args interface{}, state *State, feasiblePods []int32, key types.NamespacedName, podID int32) (uint64, *Status)

	// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if does not
	ScoreExtensions() ScoreExtensions
}

// NoMaxResourceCountArgs holds arguments used to configure the NoMaxResourceCount plugin.
type NoMaxResourceCountArgs struct {
	NumPartitions int
}

// EvenPodSpreadArgs holds arguments used to configure the EvenPodSpread plugin.
type EvenPodSpreadArgs struct {
	MaxSkew int32
}

// AvailabilityZonePriorityArgs holds arguments used to configure the AvailabilityZonePriority plugin.
type AvailabilityZonePriorityArgs struct {
	MaxSkew int32
}

// AvailabilityNodePriorityArgs holds arguments used to configure the AvailabilityNodePriority plugin.
type AvailabilityNodePriorityArgs struct {
	MaxSkew int32
}

// Code is the Status code/type which is returned from plugins.
type Code int

// These are predefined codes used in a Status.
const (
	// Success means that plugin ran correctly and found pod schedulable.
	Success Code = iota
	// Unschedulable is used when a plugin finds a pod unschedulable due to not satisying the predicate.
	Unschedulable
	// Error is used for internal plugin errors, unexpected input, etc.
	Error
)

// Status indicates the result of running a plugin.
type Status struct {
	code    Code
	reasons []string
	err     error
}

// Code returns code of the Status.
func (s *Status) Code() Code {
	if s == nil {
		return Success
	}
	return s.code
}

// Message returns a concatenated message on reasons of the Status.
func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return strings.Join(s.reasons, ", ")
}

// NewStatus makes a Status out of the given arguments and returns its pointer.
func NewStatus(code Code, reasons ...string) *Status {
	s := &Status{
		code:    code,
		reasons: reasons,
	}
	if code == Error {
		s.err = errors.New(s.Message())
	}
	return s
}

// AsStatus wraps an error in a Status.
func AsStatus(err error) *Status {
	return &Status{
		code:    Error,
		reasons: []string{err.Error()},
		err:     err,
	}
}

// AsError returns nil if the status is a success; otherwise returns an "error" object
// with a concatenated message on reasons of the Status.
func (s *Status) AsError() error {
	if s.IsSuccess() {
		return nil
	}
	if s.err != nil {
		return s.err
	}
	return errors.New(s.Message())
}

// IsSuccess returns true if and only if "Status" is nil or Code is "Success".
func (s *Status) IsSuccess() bool {
	return s.Code() == Success
}

// IsError returns true if and only if "Status" is "Error".
func (s *Status) IsError() bool {
	return s.Code() == Error
}

// IsUnschedulable returns true if "Status" is Unschedulable
func (s *Status) IsUnschedulable() bool {
	return s.Code() == Unschedulable
}

type PodScore struct {
	ID    int32
	Score uint64
}

type PodScoreList []PodScore

// PluginToPodScores declares a map from plugin name to its PodScoreList.
type PluginToPodScores map[string]PodScoreList

// PluginToStatus maps plugin name to status. Currently used to identify which Filter plugin
// returned which status.
type PluginToStatus map[string]*Status

// Merge merges the statuses in the map into one. The resulting status code have the following
// precedence: Error, Unschedulable, Success
func (p PluginToStatus) Merge() *Status {
	if len(p) == 0 {
		return nil
	}

	finalStatus := NewStatus(Success)
	for _, s := range p {
		if s.Code() == Error {
			finalStatus.err = s.AsError()
		}
		if s.Code() > finalStatus.code {
			finalStatus.code = s.Code()
		}

		finalStatus.reasons = append(finalStatus.reasons, s.reasons...)
	}

	return finalStatus
}
