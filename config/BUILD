package(default_visibility = ["//visibility:public"])

load("@k8s_object//:defaults.bzl", "k8s_object")

k8s_object(
    name = "controller",
    images = {
        "github.com/elafros/eventing/cmd/controller": "//cmd/controller:image",
    },
    template = "controller.yaml",
)

k8s_object(
    name = "namespace",
    template = "a-namespace.yaml",
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
