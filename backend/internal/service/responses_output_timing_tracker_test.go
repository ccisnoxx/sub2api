package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResponsesOutputTimingTracker_MediaAndTokenTimes(t *testing.T) {
	start := time.Unix(100, 0)
	tracker := responsesOutputTimingTracker{start: start}
	require.Equal(t, UsageTiming{}, tracker.snapshot())
	require.False(t, tracker.observe(responsesProtocolOutput{kind: "audio"}, start.Add(-time.Millisecond)))
	require.Equal(t, UsageTiming{}, tracker.snapshot())
	require.True(t, tracker.observe(responsesProtocolOutput{kind: "image"}, start.Add(10*time.Millisecond)))
	media := tracker.snapshot()
	require.Equal(t, 10, *media.FirstOutputMs)
	require.Equal(t, "image", *media.FirstOutputKind)
	require.Nil(t, media.StrictFirstTokenMs)
	require.Nil(t, media.LastTokenMs)
	require.True(t, tracker.observe(responsesProtocolOutput{kind: "reasoning", token: true}, start.Add(20*time.Millisecond)))
	require.True(t, tracker.observe(responsesProtocolOutput{kind: "tool", token: true}, start.Add(30*time.Millisecond)))
	require.True(t, tracker.observe(responsesProtocolOutput{kind: "audio"}, start.Add(40*time.Millisecond)))
	got := tracker.snapshot()
	require.Equal(t, 10, *got.FirstOutputMs)
	require.Equal(t, "image", *got.FirstOutputKind)
	require.Equal(t, 20, *got.StrictFirstTokenMs)
	require.Equal(t, 30, *got.LastTokenMs)
	// 输出组件不推断终态、音频 Token 或 accepted usage 来源。
	require.Empty(t, got.CompletionStatus)
	require.Nil(t, got.AudioOutputTokens)
	require.Empty(t, got.UsageSource)
}

func TestResponsesOutputTimingTracker_SnapshotIsIndependent(t *testing.T) {
	start := time.Unix(100, 0)
	tracker := responsesOutputTimingTracker{start: start}
	tracker.observe(responsesProtocolOutput{kind: "text", token: true}, start)
	first := tracker.snapshot()
	require.Equal(t, 0, *first.FirstOutputMs)
	require.Equal(t, 0, *first.StrictFirstTokenMs)
	require.Equal(t, 0, *first.LastTokenMs)
	*first.FirstOutputMs = 99
	*first.FirstOutputKind = "audio"
	*first.StrictFirstTokenMs = 99
	*first.LastTokenMs = 99
	next := tracker.snapshot()
	require.Equal(t, 0, *next.FirstOutputMs)
	require.Equal(t, "text", *next.FirstOutputKind)
	require.Equal(t, 0, *next.StrictFirstTokenMs)
	require.Equal(t, 0, *next.LastTokenMs)
	tracker.observe(responsesProtocolOutput{kind: "text", token: true}, start.Add(5*time.Millisecond))
	require.Equal(t, 0, *next.LastTokenMs)
	require.Equal(t, 5, *tracker.snapshot().LastTokenMs)
}
