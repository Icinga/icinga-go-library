package com

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBulk(t *testing.T) {
	subtests := []struct {
		name   string
		input  [][]string
		count  int
		spf    BulkChunkSplitPolicyFactory[string]
		output [][]string
	}{
		{"empty", nil, 1, NeverSplit[string], nil},
		{"negative", [][]string{{"a"}}, -1, NeverSplit[string], [][]string{{"a"}}},
		{"a0", [][]string{{"a"}}, 0, NeverSplit[string], [][]string{{"a"}}},
		{"a1", [][]string{{"a"}}, 1, NeverSplit[string], [][]string{{"a"}}},
		{"a2", [][]string{{"a"}}, 2, NeverSplit[string], [][]string{{"a"}}},
		{"ab1", [][]string{{"a", "b"}}, 1, NeverSplit[string], [][]string{{"a"}, {"b"}}},
		{"ab2", [][]string{{"a", "b"}}, 2, NeverSplit[string], [][]string{{"a", "b"}}},
		{"ab3", [][]string{{"a", "b"}}, 3, NeverSplit[string], [][]string{{"a", "b"}}},
		{"abc1", [][]string{{"a", "b", "c"}}, 1, NeverSplit[string], [][]string{{"a"}, {"b"}, {"c"}}},
		{"abc2", [][]string{{"a", "b", "c"}}, 2, NeverSplit[string], [][]string{{"a", "b"}, {"c"}}},
		{"abc3", [][]string{{"a", "b", "c"}}, 3, NeverSplit[string], [][]string{{"a", "b", "c"}}},
		{"abc4", [][]string{{"a", "b", "c"}}, 4, NeverSplit[string], [][]string{{"a", "b", "c"}}},
		{
			"chunks_by_timeout", [][]string{{"a", "b", "c", "d"}, {"e", "f", "g"}, {"h", "i"}, {"j"}}, 5,
			NeverSplit[string], [][]string{{"a", "b", "c", "d"}, {"e", "f", "g"}, {"h", "i"}, {"j"}},
		},
		{"chunks_by_spf", [][]string{{"a", "b", "c", "d", "e", "f", "g"}}, 2, func() BulkChunkSplitPolicy[string] {
			return func(string) bool { return true }
		}, [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}}},
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

					output := Bulk(ctx, ch, st.count, st.spf)
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
