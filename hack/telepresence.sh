#!/bin/bash

# Function to check if a command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Default environment file path
env_file="./eventing-controller.env"

# Parse command line arguments
while [ "$#" -gt 0 ]; do
  case "$1" in
    -n=*|--namespace=*) namespace="${1#*=}"; shift 1;;
    -s=*|--service=*) controller="${1#*=}"; shift 1;;
    -p=*|--port=*) port="${1#*=}"; shift 1;;
    -env=*|--env-file=*) env_file="${1#*=}"; shift 1;;
    *) echo "Unknown option: $1"; exit 1;;
  esac
done

# Check for required parameters
if [ -z "$namespace" ]; then
  echo "Error: Namespace parameter is missing. Please use the -n or --namespace flag to specify the namespace."
  exit 1
fi

if [ -z "$controller" ]; then
  echo "Error: Controller parameter is missing. Please use the -s or --service flag to specify the controller."
  exit 1
fi

if [ -z "$port" ]; then
  echo "Error: Port parameter is missing. Please use the -p or --port flag to specify the port."
  exit 1
fi

# Check port format
if [[ ! "$port" =~ ^[0-9]+:[0-9]+$ ]]; then
  echo "Error: Port format is invalid. Please use the format local-port:remote-port (e.g., 8080:8080)."
  exit 1
fi

# Check if Knative Eventing is installed
if ! kubectl get pods -n knative-eventing &>/dev/null; then
  echo "Error: Knative Eventing is not installed. Please install it before running this script."
  exit 1
fi

echo "Knative Eventing is installed."

# Check if Telepresence is installed or not
if command_exists telepresence; then
  echo "Telepresence installation found."
else
  echo "Telepresence not found. Re-run the script after installing it."
  exit 1
fi

# Connect Telepresence to the cluster
echo "Connecting Telepresence to the cluster..."
if ! telepresence connect; then
  echo "Error: Telepresence connection failed."
  exit 1
fi

# Expose Kubernetes service for the component
echo "Exposing Kubernetes service for the component..."
kubectl -n "$namespace" expose deploy/"$controller"

# Intercept the controller with Telepresence
echo "Intercepting the controller with Telepresence..."
if ! telepresence intercept "$controller" --namespace "$namespace" --port "$port" --env-file "$env_file"; then
  echo "Error: Telepresence intercept failed."
  exit 1
fi

echo "Setup complete. Make sure to start your local controller with the environment variables from eventing-controller.env."
