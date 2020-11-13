/*
Copyright 2020 Google LLC.

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

package tracing

import (
	"regexp"
	"testing"

	"github.com/openzipkin/zipkin-go/model"
)

var (
	serverKind = model.Server
	clientKind = model.Client
)

type SpanCase struct {
	Span        *model.SpanModel
	ShouldMatch bool
}

func TestSpanMatcher(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		Name    string
		Matcher *SpanMatcher
		Spans   []SpanCase
	}{{
		Name:    "empty matcher",
		Matcher: &SpanMatcher{},
		Spans: []SpanCase{{
			Span:        &model.SpanModel{},
			ShouldMatch: true,
		}, {
			Span: &model.SpanModel{
				Kind: model.Server,
				LocalEndpoint: &model.Endpoint{
					ServiceName: "test-service-name",
				},
				Tags: map[string]string{
					"test-tag":  "test-tag-value",
					"other-tag": "other-tag-value",
				},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "kind matcher",
		Matcher: &SpanMatcher{
			Kind: &serverKind,
		},
		Spans: []SpanCase{{
			Span:        &model.SpanModel{},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				Kind: model.Server,
			},
			ShouldMatch: true,
		}, {
			Span: &model.SpanModel{
				Kind: model.Client,
			},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				Kind: model.Server,
				LocalEndpoint: &model.Endpoint{
					ServiceName: "test-service-name",
				},
				Tags: map[string]string{
					"test-tag":  "test-tag-value",
					"other-tag": "other-tag-value",
				},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "local endpoint service name matcher",
		Matcher: &SpanMatcher{
			LocalEndpointServiceName: "test-service-name",
		},
		Spans: []SpanCase{{
			Span:        &model.SpanModel{},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				LocalEndpoint: &model.Endpoint{
					ServiceName: "test-service-name",
				},
			},
			ShouldMatch: true,
		}, {
			Span: &model.SpanModel{
				LocalEndpoint: &model.Endpoint{
					ServiceName: "other-service-name",
				},
			},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				Kind: model.Server,
				LocalEndpoint: &model.Endpoint{
					ServiceName: "test-service-name",
				},
				Tags: map[string]string{
					"test-tag":  "test-tag-value",
					"other-tag": "other-tag-value",
				},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "tag matcher",
		Matcher: &SpanMatcher{
			Tags: map[string]*regexp.Regexp{
				"test-tag": regexp.MustCompile("^test-tag-value$"),
			},
		},
		Spans: []SpanCase{{
			Span:        &model.SpanModel{},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				Tags: map[string]string{
					"test-tag": "test-tag-value",
				},
			},
			ShouldMatch: true,
		}, {
			Span: &model.SpanModel{
				Tags: map[string]string{
					"test-tag": "other-tag-value",
				},
			},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				Tags: map[string]string{
					"other-tag": "test-tag-value",
				},
			},
			ShouldMatch: false,
		}, {
			Span: &model.SpanModel{
				Kind: model.Server,
				LocalEndpoint: &model.Endpoint{
					ServiceName: "test-service-name",
				},
				Tags: map[string]string{
					"test-tag":  "test-tag-value",
					"other-tag": "other-tag-value",
				},
			},
			ShouldMatch: true,
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			for _, sc := range tc.Spans {
				if err := tc.Matcher.MatchesSpan(sc.Span); err != nil && sc.ShouldMatch {
					t.Errorf("expected matcher %v to match span %v but got %v", tc.Matcher, sc.Span, err)
				} else if err == nil && !sc.ShouldMatch {
					t.Errorf("expected matcher %v not to match span %v but got match", tc.Matcher, sc.Span)
				}

			}
		})
	}
}

type SpanTreeCase struct {
	SpanTree    *SpanTree
	ShouldMatch bool
}

func TestMatchesSubtree(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		Name         string
		TestSpanTree *TestSpanTree
		SpanTrees    []SpanTreeCase
	}{{
		Name:         "empty test tree",
		TestSpanTree: &TestSpanTree{},
		SpanTrees: []SpanTreeCase{{
			SpanTree:    &SpanTree{},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Client,
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"test-tag": "test-tag-value",
						},
					},
				}},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "single node",
		TestSpanTree: &TestSpanTree{
			Span: &SpanMatcher{
				Kind: &serverKind,
			},
		},
		SpanTrees: []SpanTreeCase{{
			SpanTree:    &SpanTree{},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Client,
				},
			},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Client,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Server,
					},
				}},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "single child",
		TestSpanTree: &TestSpanTree{
			Span: &SpanMatcher{
				Kind: &serverKind,
			},
			Children: []TestSpanTree{{
				Span: &SpanMatcher{
					Kind: &clientKind,
				},
			}},
		},
		SpanTrees: []SpanTreeCase{{
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Client,
					},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Client,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Server,
					},
				}},
			},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Server,
					},
					Children: []SpanTree{{
						Span: model.SpanModel{
							Kind: model.Client,
						},
					}},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Children: []SpanTree{{
						Span: model.SpanModel{
							Kind: model.Client,
						},
					}},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Client,
					},
					Children: []SpanTree{{}},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Kind: model.Client,
					},
				}, {}},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "two children",
		TestSpanTree: &TestSpanTree{
			Span: &SpanMatcher{
				Kind: &serverKind,
			},
			Children: []TestSpanTree{{
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^a$"),
					},
				},
			}, {
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^b$"),
					},
				},
			}},
		},
		SpanTrees: []SpanTreeCase{{
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "b",
						},
					},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "b",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}},
			},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
					Children: []SpanTree{{
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "b",
							},
						},
					}},
				}},
			},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					// This span is a red-herring. Although it matches child 'a',
					// it cannot be used as match for 'a' in a complete sub-tree
					// match since it is a parent of child 'b'. The matcher must
					// therefore look for alternative matches for 'a' by recursing
					// into its children.
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
					Children: []SpanTree{{
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "a",
							},
						},
					}, {
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "b",
							},
						},
					}},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Children: []SpanTree{{
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "a",
							},
						},
					}, {
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "b",
							},
						},
					}},
				}},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "two identical children",
		TestSpanTree: &TestSpanTree{
			Span: &SpanMatcher{
				Kind: &serverKind,
			},
			Children: []TestSpanTree{{
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^a$"),
					},
				},
			}, {
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^a$"),
					},
				},
			}},
		},
		SpanTrees: []SpanTreeCase{{
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "b",
						},
					},
				}},
			},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}},
			},
			ShouldMatch: false,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}},
			},
			ShouldMatch: true,
		}},
	}, {
		Name: "three children",
		TestSpanTree: &TestSpanTree{
			Span: &SpanMatcher{
				Kind: &serverKind,
			},
			Children: []TestSpanTree{{
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^a$"),
					},
				},
			}, {
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^b$"),
					},
				},
			}, {
				Span: &SpanMatcher{
					Tags: map[string]*regexp.Regexp{
						"child": regexp.MustCompile("^c$"),
					},
				},
			}},
		},
		SpanTrees: []SpanTreeCase{{
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "a",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "b",
						},
					},
				}, {
					Span: model.SpanModel{
						Tags: map[string]string{
							"child": "c",
						},
					},
				}},
			},
			ShouldMatch: true,
		}, {
			SpanTree: &SpanTree{
				Span: model.SpanModel{
					Kind: model.Server,
				},
				Children: []SpanTree{{
					Children: []SpanTree{{
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "a",
							},
						},
					}, {
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "b",
							},
						},
					}},
				}, {
					Children: []SpanTree{{
						Span: model.SpanModel{
							Tags: map[string]string{
								"child": "c",
							},
						},
					}},
				}},
			},
			ShouldMatch: true,
		}},
	},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			for _, stc := range tc.SpanTrees {
				if matches := tc.TestSpanTree.MatchesSubtree(t, stc.SpanTree); len(matches) == 0 && stc.ShouldMatch {
					t.Errorf("expected test tree %v to match span tree %v", tc.TestSpanTree, stc.SpanTree)
				} else if len(matches) > 0 && !stc.ShouldMatch {
					t.Errorf("expected test tree %v not to match span tree %v but got matches %v", tc.TestSpanTree, stc.SpanTree, matches)
				}
			}
		})
	}
}
