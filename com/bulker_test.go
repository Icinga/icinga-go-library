package com

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBulk(t *testing.T) {
	noSp := NeverSplit[string]
	var closeContext []string = nil

	subtests := []struct {
		name   string
		input  [][]string
		count  int
		spf    BulkChunkSplitPolicyFactory[string]
		output [][]string
	}{
		{"empty", nil, 1, noSp, nil},
		{"negative", [][]string{{"a"}}, -1, noSp, [][]string{{"a"}}},
		{"a0", [][]string{{"a"}}, 0, noSp, [][]string{{"a"}}},
		{"a1", [][]string{{"a"}}, 1, noSp, [][]string{{"a"}}},
		{"a2", [][]string{{"a"}}, 2, noSp, [][]string{{"a"}}},
		{"ab1", [][]string{{"a", "b"}}, 1, noSp, [][]string{{"a"}, {"b"}}},
		{"ab2", [][]string{{"a", "b"}}, 2, noSp, [][]string{{"a", "b"}}},
		{"ab3", [][]string{{"a", "b"}}, 3, noSp, [][]string{{"a", "b"}}},
		{"abc1", [][]string{{"a", "b", "c"}}, 1, noSp, [][]string{{"a"}, {"b"}, {"c"}}},
		{"abc2", [][]string{{"a", "b", "c"}}, 2, noSp, [][]string{{"a", "b"}, {"c"}}},
		{"abc3", [][]string{{"a", "b", "c"}}, 3, noSp, [][]string{{"a", "b", "c"}}},
		{"abc4", [][]string{{"a", "b", "c"}}, 4, noSp, [][]string{{"a", "b", "c"}}},
		{
			"chunks_by_timeout", [][]string{{"a", "b", "c", "d"}, {"e", "f", "g"}, {"h", "i"}, {"j"}}, 5,
			noSp, [][]string{{"a", "b", "c", "d"}, {"e", "f", "g"}, {"h", "i"}, {"j"}},
		},
		{"chunks_by_spf", [][]string{{"a", "b", "c", "d", "e", "f", "g"}}, 2, func() BulkChunkSplitPolicy[string] {
			return func(string) bool { return true }
		}, [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}}},
		{"close-ctx_a1", [][]string{closeContext, {"a"}}, 1, noSp, nil},
		{"close-ctx_a4", [][]string{closeContext, {"a"}}, 4, noSp, nil},
		{"a_close-ctx_b1", [][]string{{"a"}, closeContext, {"b"}}, 1, noSp, [][]string{{"a"}}},
		{"a_close-ctx_b4", [][]string{{"a"}, closeContext, {"b"}}, 4, noSp, [][]string{{"a"}}},
		{"ab_close-ctx_c1", [][]string{{"a", "b"}, closeContext, {"c"}}, 1, noSp, [][]string{{"a"}, {"b"}}},
		{"ab_close-ctx_c4", [][]string{{"a", "b"}, closeContext, {"c"}}, 4, noSp, [][]string{{"a", "b"}}},
	}

	latencies := []struct {
		name    string
		latency time.Duration
	}{
		{"instantly", 0},
		{"1us", time.Microsecond},
		{"20ms", 20 * time.Millisecond},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for _, l := range latencies {
				t.Run(l.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					bulkCtx, cancelBulk := context.WithCancel(ctx)

					ch := make(chan string, 1)
					go func() {
						defer close(ch)

						for i, chunk := range st.input {
							if i > 0 {
								select {
								case <-time.After(time.Second / 2):
								case <-ctx.Done():
									return
								}
							}

							if chunk == nil {
								cancelBulk()
							}

							for _, v := range chunk {
								if l.latency > 0 {
									select {
									case <-time.After(l.latency):
									case <-ctx.Done():
										return
									}
								}

								select {
								case ch <- v:
								case <-ctx.Done():
									return
								}
							}
						}
					}()

					output := Bulk(bulkCtx, ch, st.count, st.spf)
					require.NotNil(t, output)

					for _, expected := range st.output {
						select {
						case actual, ok := <-output:
							if !ok {
								require.Fail(t, "channel should not be closed, yet")
							}

							require.Equal(t, expected, actual)
						case <-time.After(time.Second):
							require.Fail(t, "channel should not block")
						}
					}

					select {
					case _, ok := <-output:
						if ok {
							require.Fail(t, "channel should be closed")
						}
					case <-time.After(time.Second):
						require.Fail(t, "channel should not block")
					}
				})
			}
		})
	}
}
