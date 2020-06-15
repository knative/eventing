
This is the `CatalogSource` for the
[knative-eventing-operator](https://github.com/openshift-knative/knative-eventing-operator).

WARNING: The `knative-eventing` operator requires a CRD provided by the
`knative-serving` `CatalogSource`, so install it first.

To install this `CatalogSource`:

    OLM=$(kubectl get pods --all-namespaces | grep olm-operator | head -1 | awk '{print $1}')
    kubectl apply -n $OLM -f https://raw.githubusercontent.com/openshift/knative-eventing/release-v0.5.0/openshift/olm/knative-eventing.catalogsource.yaml

To subscribe to it (which will trigger the installation of
knative-eventing), either use the console, or apply the following:

	---
	apiVersion: v1
	kind: Namespace
	metadata:
	  name: knative-eventing
          labels:
            istio-injection: enabled
	---
	apiVersion: operators.coreos.com/v1
	kind: OperatorGroup
	metadata:
	  name: knative-eventing
	  namespace: knative-eventing
	---
	apiVersion: operators.coreos.com/v1alpha1
	kind: Subscription
	metadata:
	  name: knative-eventing-operator-sub
	  generateName: knative-eventing-operator-
	  namespace: knative-eventing
	spec:
	  source: knative-eventing-operator
	  sourceNamespace: olm
	  name: knative-eventing-operator
	  startingCSV: knative-eventing-operator.v0.5.0
	  channel: alpha
