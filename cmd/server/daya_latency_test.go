package main

import "testing"

func TestSummarizeDayaLatencyUsesSuccessfulSamplesOnly(t *testing.T) {
	got := summarizeDayaLatency([]dayaLatencySample{
		{HTTPStatus: 403, RTTMillis: 100},
		{HTTPStatus: 401, RTTMillis: 200},
		{HTTPStatus: 0, RTTMillis: 0, Error: "timeout"},
	})
	if got.Count != 2 || got.MinRTTMillis != 100 || got.MedianRTTMillis != 150 || got.MeanRTTMillis != 150 || got.EstimatedOneWayMS != 75 {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if got.P95RTTMillis != 200 {
		t.Fatalf("p95=%d, want 200", got.P95RTTMillis)
	}
}

func TestSummarizeDayaLatencyEmptySamples(t *testing.T) {
	got := summarizeDayaLatency(nil)
	if got.Count != 0 || got.EstimatedOneWayMS != 0 {
		t.Fatalf("unexpected empty summary: %#v", got)
	}
}
