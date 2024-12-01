// Memory limiter test

package main

import (
	"testing"
)

func testFragmentAppend(t *testing.T, buffer []*HlsFragment, limiter *FragmentBufferMemoryLimiter, data []byte, expectedCanBeAdded bool) []*HlsFragment {
	fragment := &HlsFragment{
		Duration: 1,
		Data:     data,
	}

	newBuffer, canAdd := limiter.CheckBeforeAddingFragment(buffer, fragment)

	if expectedCanBeAdded != canAdd {
		t.Errorf("canAdd does not match. Expected %v, Actual: %v", expectedCanBeAdded, canAdd)
	}

	if !canAdd {
		return newBuffer
	} else {
		return append(newBuffer, fragment)
	}
}

func computeBufferUsage(buffer []*HlsFragment) int64 {
	total := int64(0)

	for _, v := range buffer {
		total += int64(len(v.Data))
	}

	return total
}

func TestFragmentBufferMemoryLimiter(t *testing.T) {
	memoryLimiter := NewFragmentBufferMemoryLimiter(FragmentBufferMemoryLimiterConfig{
		Enabled: true,
		Limit:   10,
	})

	buffer1 := make([]*HlsFragment, 0)
	buffer2 := make([]*HlsFragment, 0)

	buffer1 = testFragmentAppend(t, buffer1, memoryLimiter, []byte{0x00, 0x00, 0x00}, true)

	if memoryLimiter.usage != 3 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 3)
	}

	actualUsage := computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	buffer1 = testFragmentAppend(t, buffer1, memoryLimiter, []byte{}, true)

	if memoryLimiter.usage != 3 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 3)
	}

	actualUsage = computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	buffer2 = testFragmentAppend(t, buffer2, memoryLimiter, []byte{0x01, 0x01, 0x01, 0x01, 0x01}, true)

	if memoryLimiter.usage != 8 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 8)
	}

	actualUsage = computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	buffer1 = testFragmentAppend(t, buffer1, memoryLimiter, []byte{0x02, 0x02, 0x02, 0x02, 0x02}, true)

	if memoryLimiter.usage != 10 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 10)
	}

	actualUsage = computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	buffer1 = testFragmentAppend(t, buffer1, memoryLimiter, []byte{0x02}, true)

	if memoryLimiter.usage != 6 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 6)
	}

	actualUsage = computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	buffer1 = testFragmentAppend(t, buffer1, memoryLimiter, []byte{0x10, 0x10, 0x10, 0x10, 0x10, 0x10}, false)

	if memoryLimiter.usage != 5 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 5)
	}

	actualUsage = computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	buffer1 = testFragmentAppend(t, buffer1, memoryLimiter, []byte{0x10, 0x10, 0x10, 0x10}, true)

	if memoryLimiter.usage != 9 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 9)
	}

	actualUsage = computeBufferUsage(buffer1) + computeBufferUsage(buffer2)

	if memoryLimiter.usage != actualUsage {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, actualUsage)
	}

	memoryLimiter.OnBufferRelease(buffer2)

	if memoryLimiter.usage != 4 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 4)
	}

	memoryLimiter.OnBufferRelease(make([]*HlsFragment, 0))

	if memoryLimiter.usage != 4 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 4)
	}

	memoryLimiter.OnBufferRelease(buffer1)

	if memoryLimiter.usage != 0 {
		t.Errorf("memoryLimiter.usage does not match. Expected %v, Actual: %v", memoryLimiter.usage, 0)
	}
}
