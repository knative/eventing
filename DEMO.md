# Importer Autocreation Demo

**This will create GCP resources that cost money.**

## Prereqs

1. kubectl, gcloud, ko, gsutil in path.
1. gcloud is authenticated and has a project configured with billing.
1. KO_DOCKER_REPO must be set.
1. GOPATH must be set.
1. eventing, eventing-contrib, and gcs repos must be checked out in the proper paths:
    ```
    $GOPATH/src/github.com/knative/eventing
    $GOPATH/src/github.com/knative/eventing-contrib
    $GOPATH/src/github.com/vaikas-google/gcs
    ```
1. These env vars must be set:
    ```sh
    export CLUSTER_NAME=<cluster name>
    export CLUSTER_ZONE=<cluster zone>
    export PROJECT_ID=<project>
    export BUCKET=<bucket name>
    ```

## Setup

```sh
# Create cluster https://knative.dev/docs/install/knative-with-gke/#creating-a-kubernetes-cluster

gcloud beta container clusters create $CLUSTER_NAME --addons=HorizontalPodAutoscaling,HttpLoadBalancing,Istio --machine-type=n1-standard-4 --cluster-version=latest --zone=$CLUSTER_ZONE --enable-stackdriver-kubernetes --enable-ip-alias --enable-autoscaling --min-nodes=1 --max-nodes=10 --enable-autorepair --scopes cloud-platform

kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value core/account)

# Install serving

kubectl apply --selector knative.dev/crd-install=true -f https://github.com/knative/serving/releases/download/v0.7.1/serving-pre-1.14.yaml

kubectl apply -f https://github.com/knative/serving/releases/download/v0.7.1/serving-pre-1.14.yaml

# Checkout nachocano/eventing@wat-demo and cd to it

cd $GOPATH/src/github.com/knative/eventing
git remote add nachocano https://github.com/nachocano/eventing
git fetch nachocano
git branch wat-demo nachocano/wat-demo
git checkout wat-demo

# Install eventing and in-memory provisioner

ko apply -f config
ko apply -f config/provisioners/in-memory-channel


# Checkout nachocano/eventing-contrib@wat-demo and cd to it
cd $GOPATH/src/github.com/knative/eventing-contrib
git remote add nachocano https://github.com/nachocano/eventing-contrib
git fetch nachocano
git branch wat-demo nachocano/wat-demo
git checkout wat-demo


# Install GCPPubsubSource
ko apply -f contrib/gcppubsub/config


# Checkout nachocano/gcs@wat-demo and cd to it
cd $GOPATH/src/github.com/vaikas-google/gcs
git remote add nachocano https://github.com/nachocano/gcs
git fetch nachocano
git branch wat-demo nachocano/wat-demo
git checkout wat-demo

# Install GCSSource
ko apply -f config

# Create a GCP Service Account with PubSub and GCS permissions

gcloud iam service-accounts create knative-source
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:knative-source@$PROJECT_ID.iam.gserviceaccount.com --role roles/pubsub.editor
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:knative-source@$PROJECT_ID.iam.gserviceaccount.com --role roles/storage.admin

# Create credential file

gcloud iam service-accounts keys create knative-source.json --iam-account=knative-source@$PROJECT_ID.iam.gserviceaccount.com

# Give GCS permission to publish to PubSub

export GCS_SERVICE_ACCOUNT=`curl -s -X GET -H "Authorization: Bearer \`GOOGLE_APPLICATION_CREDENTIALS=./knative-source.json gcloud auth application-default print-access-token\`" "https://www.googleapis.com/storage/v1/projects/$PROJECT_ID/serviceAccount" | grep email_address | cut -d '"' -f 4`
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:$GCS_SERVICE_ACCOUNT --role roles/pubsub.publisher

# Create k8s secrets

kubectl -n knative-sources create secret generic gcppubsub-source-key --from-file=key.json=knative-source.json --dry-run -o yaml | kubectl apply --filename -
kubectl create ns gcssource-system
kubectl --namespace gcssource-system create secret generic gcs-source-key --from-file=key.json=knative-source.json
kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=knative-source.json

# Create bucket or choose existing bucket

gsutil mb gs://$BUCKET

# Back to eventing; edit example trigger yaml with correct project and bucket name

cd $GOPATH/src/github.com/knative/eventing

sed "s/PROJECT_ID/$PROJECT_ID/" t4.yaml | sed "s/BUCKET/$BUCKET/" > edited-t4.yaml

# Label default namespace to create a Broker

kubectl label namespace default knative-eventing-injection=enabled

# Verify Broker is Ready

kubectl get brokers
```

## Demo 1: GCS Trigger

Terminal A:
```sh
watch kubectl logs --since=5m --tail=30 --selector serving.knative.dev/service=event-display -c user-container

# OR https://github.com/wercker/stern

stern "event-display.*" -c user-container
```

Terminal B:

```sh
cd $GOPATH/src/github.com/knative/eventing

ko apply -f event-display.yaml

kubectl get eventtypes

kubectl get eventtype gcs-object-finalize -oyaml

kubectl apply -f edited-t4.yaml

gsutil cp t3.yaml gs://$BUCKET

# See event-display pod created, event appear in log (after cold start)
```

## Demo 2: Cron Trigger

```sh
kubectl apply -f et2.yaml

kubectl get eventtypes

kubectl get eventtype cron -oyaml

kubectl apply -f t2.yaml

# Wait up to 1 minute

# See event appear in log alongside GCS event
