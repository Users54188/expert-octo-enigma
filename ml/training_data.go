package ml

import (
	"errors"
	"time"

	"cloudquant/market"
)

func GenerateLabels(klines []market.KLine, lookAhead int) ([]int, error) {
	if len(klines) == 0 {
		return nil, errors.New("klines is empty")
	}
	if lookAhead <= 0 {
		return nil, errors.New("lookAhead must be positive")
	}
	labels := make([]int, len(klines))
	for i := range klines {
		if i+lookAhead >= len(klines) {
			labels[i] = 1
			continue
		}
		current := klines[i].Close
		future := klines[i+lookAhead].Close
		if current == 0 {
			labels[i] = 1
			continue
		}
		returnRate := (future - current) / current
		switch {
		case returnRate < -0.02:
			labels[i] = 0
		case returnRate > 0.02:
			labels[i] = 2
		default:
			labels[i] = 1
		}
	}
	return labels, nil
}

func BuildTrainingSet(symbol string, startDate, endDate time.Time) (features [][]float64, labels []int, err error) {
	if symbol == "" {
		return nil, nil, errors.New("symbol is required")
	}
	if endDate.Before(startDate) {
		return nil, nil, errors.New("endDate before startDate")
	}

	// approximate days
	days := int(endDate.Sub(startDate).Hours()/24) + 1
	if days <= 0 {
		return nil, nil, errors.New("invalid date range")
	}

	klines, err := market.FetchHistoricalData(symbol, days)
	if err != nil {
		return nil, nil, err
	}
	featuresStruct, err := ExtractFeatures(klines)
	if err != nil {
		return nil, nil, err
	}
	labels, err = GenerateLabels(klines, 3)
	if err != nil {
		return nil, nil, err
	}

	featureVectors := make([][]float64, 0, len(featuresStruct))
	labelValues := make([]int, 0, len(featuresStruct))
	offset := len(klines) - len(featuresStruct)
	for i, feature := range featuresStruct {
		labelIndex := i + offset
		if labelIndex >= len(labels) {
			break
		}
		featureVectors = append(featureVectors, FeatureVector(feature))
		labelValues = append(labelValues, labels[labelIndex])
	}

	return featureVectors, labelValues, nil
}
