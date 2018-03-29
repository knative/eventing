package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "gazelle", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "github.com/elafros/eventing/cmd/controller",
    visibility = ["//visibility:private"],
    deps = [
        "//pkg/client/clientset/versioned:go_default_library",
        "//pkg/client/informers/externalversions:go_default_library",
        "//pkg/controller:go_default_library",
        "//pkg/controller/bind:go_default_library",
        "//pkg/signals:go_default_library",
        "//vendor/github.com/elafros/elafros/pkg/client/clientset/versioned:go_default_library",
        "//vendor/github.com/elafros/elafros/pkg/client/informers/externalversions:go_default_library",
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus/promhttp:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/plugin/pkg/client/auth/gcp:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
    ],
)

go_binary(
    name = "controller",
    embed = [":go_default_library"],
    pure = "on",
    visibility = ["//visibility:public"],
)

load("@io_bazel_rules_docker//go:image.bzl", "go_image")

go_image(
    name = "image",
    binary = ":controller",
)
