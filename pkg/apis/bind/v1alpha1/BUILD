load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "bind_types.go",
        "doc.go",
        "event_source_types.go",
        "event_type_types.go",
        "register.go",
        "zz_generated.deepcopy.go",
    ],
    importpath = "github.com/elafros/binding/pkg/apis/bind/v1alpha1",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/apis/bind:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
    ],
)
