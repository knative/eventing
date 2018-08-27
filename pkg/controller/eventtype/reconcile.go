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

package eventtype

import (
	"context"
	"fmt"
	"github.com/golang/glog"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

const (
	finalizerName = controllerAgentName
)

// Reconcile compares the actual state of a Feed with the desired, and attempts
// to converge the two. It then updates the Status block of the Feed with
// its current state.
// If Reconcile returns a non-nil error, the request will be retried.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
	ctx := context.TODO()

	et := &feedsv1alpha1.EventType{}
	err := r.client.Get(ctx, request.NamespacedName, et)

	// The EventType may have been deleted since it was added to the workqueue. If so
	// there's nothing to be done.
	if errors.IsNotFound(err) {
		glog.Errorf("could not find EventType %v\n", request)
		return reconcile.Result{}, nil
	}

	// If the EventType exists but could not be retrieved, then we should retry.
	if err != nil {
		glog.Errorf("could not fetch EventType %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	// Now that we know the EventType exists, we can reconcile it. An error returned
	// here means the reconcile did not complete and the EventType should be requeued
	// for another attempt.
	// A successful reconcile does not necessarily mean the EventType is in the desired
	// state, it means no more can be done for now. In this case the EventType will
	// not be reconciled again until the resync period or a watched resource changes.
	if err = r.reconcile(ctx, et); err != nil {
		glog.Errorf("error reconciling EventType: %v", err)
	}

	// Since the reconcile is a sequence of steps, earlier steps may complete
	// successfully while later steps fail. The EventType is updated on failure to
	// preserve any useful status or metadata changes the non-failing steps made.
	if updateErr := r.updateEventType(ctx, et); updateErr != nil {
		glog.Errorf("failed to update EventType: %v", updateErr)
		// An error here means the EventType should be reconciled again, regardless of
		// whether the reconcile was successful or not.
		return reconcile.Result{}, updateErr
	}
	return reconcile.Result{}, err
}

// reconcile tries to converge the current state of the given EventType to the desired state. This
// function should not update the EventType in the API server. This method will update 'et'. The
// calling method is responsible for writing back to the API server.
func (r *reconciler) reconcile(ctx context.Context, et *feedsv1alpha1.EventType) error {
	if et.GetDeletionTimestamp() == nil {
		r.addFinalizer(et)
	} else {
		err := r.handleDeletion(ctx, et)
		if err != nil {
			glog.Errorf("Error deleting the EventType: %v", err)
			return err
		}
	}
	return nil
}

func (r *reconciler) addFinalizer(et *feedsv1alpha1.EventType) {
	finalizers := sets.NewString(et.Finalizers...)
	finalizers.Insert(finalizerName)
	et.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(et *feedsv1alpha1.EventType) {
	finalizers := sets.NewString(et.Finalizers...)
	finalizers.Delete(finalizerName)
	et.Finalizers = finalizers.List()
}

// handleDeletion checks the finalizer conditions of an EventType marked for deletion. If the
// conditions are met, then it removes the finalizer. Otherwise it adds a Status saying why it
// can't.
func (r *reconciler) handleDeletion(ctx context.Context, et *feedsv1alpha1.EventType) error {
	feeds, err := r.findFeedsUsingEventType(ctx, et)
	if err != nil {
		glog.Infof("Unable to find Feeds using EventType %s: %v", et.Name, err)
		return err
	}
	if len(feeds) == 0 {
		r.removeFinalizer(et)
	} else {
		glog.Infof("Cannot remove finalizer from EventType %q, %v Feed(s) still use it.", et.Name, len(feeds))
	}
	r.updateInUseStatus(ctx, et, feeds)
	return nil
}

// findFeedsUsingEventTypes finds all the Feeds in the same namespace as et that use et.
func (r *reconciler) findFeedsUsingEventType(ctx context.Context, et *feedsv1alpha1.EventType) (
	[]feedsv1alpha1.Feed, error) {
	allFeeds := &feedsv1alpha1.FeedList{}
	err := r.client.List(ctx, &client.ListOptions{}, allFeeds)
	if err != nil {
		glog.Errorf("Unable to list the feeds: %v", err)
		return nil, err
	}
	feeds := make([]feedsv1alpha1.Feed, 0, len(allFeeds.Items))

	for _, feed := range allFeeds.Items {
		if et.Name == feed.Spec.Trigger.EventType {
			feeds = append(feeds, feed)
		}
	}
	return feeds, nil
}

func (r *reconciler) updateInUseStatus(ctx context.Context, et *feedsv1alpha1.EventType, feedsStillUsingEventType []feedsv1alpha1.Feed) {
	// Filter out the existing InUse condition, if present.
	var newConditions []feedsv1alpha1.CommonEventTypeCondition
	for _, condition := range et.Status.Conditions {
		if condition.Type != feedsv1alpha1.EventTypeInUse {
			newConditions = append(newConditions, condition)
		}
	}

	if len(feedsStillUsingEventType) > 0 {
		// Add the up-to-date InUse condition.
		newConditions = append(newConditions, feedsv1alpha1.CommonEventTypeCondition{
			Type:    feedsv1alpha1.EventTypeInUse,
			Status:  corev1.ConditionTrue,
			Message: fmt.Sprintf("Still in use by the Feeds: %s", getFeedNames(feedsStillUsingEventType)),
		})
	}

	et.Status.Conditions = newConditions
}

// getFeedNames generates a single string with the names of all the feeds.
func getFeedNames(feeds []feedsv1alpha1.Feed) string {
	feedNames := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		feedNames = append(feedNames, feed.Name)
	}
	return strings.Join(feedNames, ", ")
}

func (r *reconciler) updateEventType(ctx context.Context, u *feedsv1alpha1.EventType) error {
	et := &feedsv1alpha1.EventType{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, et)
	if err != nil {
		return err
	}

	updated := false
	if !equality.Semantic.DeepEqual(et.Finalizers, u.Finalizers) {
		et.SetFinalizers(u.ObjectMeta.Finalizers)
		updated = true
	}

	if !equality.Semantic.DeepEqual(et.Status, u.Status) {
		et.Status = u.Status
		updated = true
	}

	if updated == false {
		return nil
	}
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Feed resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return r.client.Update(ctx, et)
}
