load("@io_bazel_rules_go//go:def.bzl", "gazelle", "go_prefix")

go_prefix("github.com/elafros/eventing")

gazelle(
    name = "gazelle",
    external = "vendored",
)

load("@k8s_object//:defaults.bzl", "k8s_object")

k8s_object(
    name = "controller",
    images = {
        "bind-controller:latest": "//cmd/controller:image",
    },
    template = "controller.yaml",
)

k8s_object(
    name = "namespace",
    template = "namespace.yaml",
)

k8s_object(
    name = "serviceaccount",
    template = "serviceaccount.yaml",
)

k8s_object(
    name = "clusterrolebinding",
    template = "clusterrolebinding.yaml",
)

k8s_object(
    name = "bind",
    template = "bind.yaml",
)

k8s_object(
    name = "eventtype",
    template = "eventtype.yaml",
)

k8s_object(
    name = "eventsource",
    template = "eventsource.yaml",
)

load("@io_bazel_rules_k8s//k8s:objects.bzl", "k8s_objects")

k8s_objects(
    name = "authz",
    objects = [
        ":serviceaccount",
        ":clusterrolebinding",
    ],
)

k8s_objects(
    name = "everything",
    objects = [
        ":namespace",
        ":authz",
        ":bind",
        ":eventtype",
        ":eventsource",
        ":controller",
    ],
)
