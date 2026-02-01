package ml

func FeatureStats(features []MLFeatures) map[string][2]float64 {
	return computeFeatureStats(features)
}
