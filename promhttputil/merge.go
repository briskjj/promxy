package promhttputil

import (
	"fmt"
	"reflect"

	"github.com/prometheus/common/model"
)

func ValueAddLabelSet(a model.Value, l model.LabelSet) error {
	switch aTyped := a.(type) {
	case model.Vector:
		for _, item := range aTyped {
			for k, v := range l {
				item.Metric[k] = v
			}
		}

	case model.Matrix:
		for _, item := range aTyped {
			for k, v := range l {
				item.Metric[k] = v
			}
		}
	}

	return nil

}

// TODO: always make copies? Now we sometimes return one, or make a copy, or do nothing
// Merge 2 values and
func MergeValues(a, b model.Value) (model.Value, error) {
	if a.Type() != b.Type() {
		return nil, fmt.Errorf("Error!")
	}

	switch aTyped := a.(type) {
	// TODO: more logic? for now we assume both are correct if they exist
	// In the case where it is a single datapoint, we're going to assume that
	// either is valid, we just need one
	case *model.Scalar:
		bTyped := b.(*model.Scalar)

		if aTyped.Value != 0 && aTyped.Timestamp != 0 {
			return aTyped, nil
		} else {
			return bTyped, nil
		}

	// In the case where it is a single datapoint, we're going to assume that
	// either is valid, we just need one
	case *model.String:
		bTyped := b.(*model.String)

		if aTyped.Value != "" && aTyped.Timestamp != 0 {
			return aTyped, nil
		} else {
			return bTyped, nil
		}

	// List of *model.Sample -- only 1 value (guaranteed same timestamp)
	case model.Vector:
		bTyped := b.(model.Vector)

		newValue := make(model.Vector, 0, len(aTyped)+len(bTyped))
		fingerPrintMap := make(map[model.Fingerprint]int)

		addItem := func(item *model.Sample) {
			finger := item.Metric.Fingerprint()

			// If we've seen this fingerPrint before, lets make sure that a value exists
			if index, ok := fingerPrintMap[finger]; ok {
				// TODO: better? For now we only replace if we have no value (which seems reasonable)
				if newValue[index].Value == model.SampleValue(0) {
					newValue[index].Value = item.Value
				}
			} else {
				newValue = append(newValue, item)
				fingerPrintMap[finger] = len(newValue) - 1
			}
		}

		for _, item := range aTyped {
			addItem(item)
		}

		for _, item := range bTyped {
			addItem(item)
		}
		return newValue, nil

	case model.Matrix:
		bTyped := b.(model.Matrix)

		newValue := make(model.Matrix, 0, len(aTyped)+len(bTyped))
		fingerPrintMap := make(map[model.Fingerprint]int)

		addStream := func(stream *model.SampleStream) {
			finger := stream.Metric.Fingerprint()

			// If we've seen this fingerPrint before, lets make sure that a value exists
			if index, ok := fingerPrintMap[finger]; ok {
				// TODO: check this error? For now the only one is sig collision, which we check
				newValue[index], _ = MergeSampleStream(newValue[index], stream)
			} else {
				newValue = append(newValue, stream)
				fingerPrintMap[finger] = len(newValue) - 1
			}
		}

		for _, item := range aTyped {
			addStream(item)
		}

		for _, item := range bTyped {
			addStream(item)
		}
		return newValue, nil
	}

	return nil, fmt.Errorf("Unknown type! %v", reflect.TypeOf(a))
}

func MergeSampleStream(a, b *model.SampleStream) (*model.SampleStream, error) {
	if a.Metric.Fingerprint() != b.Metric.Fingerprint() {
		return nil, fmt.Errorf("Cannot merge mismatch fingerprints")
	}

	// TODO: really there should be a library method for this in prometheus IMO
	// At this point we have 2 sorted lists of datapoints which we need to merge
	seenTimes := make(map[model.Time]struct{})
	newValues := make([]model.SamplePair, 0, len(a.Values)+len(b.Values))

	ai := 0 // Offset in a
	bi := 0 // Offset in b

	// When combining series from 2 different prometheus hosts we can run into some problems
	// with clock skew (from a variety of sources). The primary one I've run into is issues
	// with the time that prometheus stores. Since the time associated with the datapoint is
	// the *start* time of the scrape, there can be quite a lot of time (which can vary
	// dramatically between hosts) for the exporter to return. In an attempt to mitigate
	// this problem we're going to *not* merge any datapoint within 10s of another point
	// we have. This means we can tolerate 5s on either side (which can be used by either
	// clock skew or from this scrape skew).

	// TODO: config
	antiAffinityBuffer := model.TimeFromUnix(10) // 10s
	var lastTime model.Time

	for {
		if ai >= len(a.Values) && bi >= len(b.Values) {
			break
		}

		var item model.SamplePair

		if ai < len(a.Values) { // If a exists
			if bi < len(b.Values) {
				// both items
				if a.Values[ai].Timestamp < b.Values[bi].Timestamp {
					item = a.Values[ai]
					ai++
				} else {
					item = b.Values[bi]
					bi++
				}
			} else {
				// Only A
				item = a.Values[ai]
				ai++
			}
		} else {
			if bi < len(b.Values) {
				// Only B
				item = b.Values[bi]
				bi++
			}
		}
		// If we've already seen this timestamp, skip
		if _, ok := seenTimes[item.Timestamp]; ok {
			continue
		}

		if lastTime == 0 {
			lastTime = item.Timestamp
		}

		if item.Timestamp-lastTime < antiAffinityBuffer {
			continue
		}
		lastTime = item.Timestamp

		// Otherwise, lets add it
		newValues = append(newValues, item)
		seenTimes[item.Timestamp] = struct{}{}
	}

	return &model.SampleStream{
		Metric: a.Metric,
		Values: newValues,
	}, nil
}