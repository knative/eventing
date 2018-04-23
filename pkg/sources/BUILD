load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "event_source.go",
        "event_trigger.go",
        "github.go",
    ],
    importpath = "github.com/elafros/eventing/pkg/sources",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/google/go-github/github:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
    ],
)
