load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "flow_types.go",
        "register.go",
        "zz_generated.deepcopy.go",
    ],
    importpath = "github.com/elafros/eventing/pkg/apis/eventing/v1beta2",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/apis/eventing:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
    ],
)
