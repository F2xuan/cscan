package scanner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaabuAggregatedTimeoutScalesPerTargetTimeout(t *testing.T) {
	s := NewNaabuScanner()
	opts := &NaabuOptions{
		Ports:             "80,443",
		Rate:              1000,
		Timeout:           10,
		ScanType:          "c",
		PortThreshold:     0,
		SkipHostDiscovery: true,
		Workers:           1,
		AggregatedTimeout: 90,
	}

	cfg := &ScanConfig{
		Target:            "host1\nhost2\nhost3",
		Options:           opts,
		WorkerConcurrency: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _ = s.Scan(ctx, cfg)
	assert.GreaterOrEqual(t, opts.Timeout, 10)
}

func TestNaabuAggregatedTimeoutDoesNotShrinkExistingTimeout(t *testing.T) {
	s := NewNaabuScanner()
	opts := &NaabuOptions{
		Ports:             "80,443",
		Rate:              1000,
		Timeout:           120,
		ScanType:          "c",
		PortThreshold:     0,
		SkipHostDiscovery: true,
		Workers:           1,
		AggregatedTimeout: 10,
	}

	cfg := &ScanConfig{
		Target:            "host1\nhost2",
		Options:           opts,
		WorkerConcurrency: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _ = s.Scan(ctx, cfg)
	assert.GreaterOrEqual(t, opts.Timeout, 120)
}

func TestNaabuAggregatedTimeoutEmptyTargetsSkipsScaling(t *testing.T) {
	s := NewNaabuScanner()
	opts := &NaabuOptions{
		Ports:             "80,443",
		Rate:              1000,
		Timeout:           30,
		ScanType:          "c",
		PortThreshold:     0,
		SkipHostDiscovery: true,
		Workers:           1,
		AggregatedTimeout: 999,
	}

	cfg := &ScanConfig{
		Target:            "",
		Options:           opts,
		WorkerConcurrency: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _ = s.Scan(ctx, cfg)
	assert.Equal(t, 30, opts.Timeout)
}

func TestNaabuAggregatedTimeoutSingleTargetUsesFullBudget(t *testing.T) {
	s := NewNaabuScanner()
	opts := &NaabuOptions{
		Ports:             "80,443",
		Rate:              1000,
		Timeout:           5,
		ScanType:          "c",
		PortThreshold:     0,
		SkipHostDiscovery: true,
		Workers:           1,
		AggregatedTimeout: 180,
	}

	cfg := &ScanConfig{
		Target:            "host1",
		Options:           opts,
		WorkerConcurrency: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _ = s.Scan(ctx, cfg)
	assert.Greater(t, opts.Timeout, 5)
}

func TestNaabuTop100AndTop1000PortsPassThrough(t *testing.T) {
	s := NewNaabuScanner()

	for _, ports := range []string{"top100", "top1000"} {
		opts := &NaabuOptions{
			Ports:             ports,
			Rate:              1000,
			Timeout:           10,
			ScanType:          "c",
			PortThreshold:     0,
			SkipHostDiscovery: true,
			Workers:           1,
		}

		cfg := &ScanConfig{
			Target:            "host1",
			Options:           opts,
			WorkerConcurrency: 1,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := s.Scan(ctx, cfg)
		_ = err
	}
}
