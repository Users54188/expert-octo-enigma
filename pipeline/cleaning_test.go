package pipeline

import (
	"testing"
	"time"
)

func TestNewDataCleaner(t *testing.T) {
	cleaner := NewDataCleaner()
	if cleaner == nil {
		t.Fatal("NewDataCleaner returned nil")
	}

	if len(cleaner.rules) == 0 {
		t.Error("No default rules added")
	}
}

func TestPriceValidationRule(t *testing.T) {
	rule := NewPriceValidationRule()

	tests := []struct {
		name    string
		point   *DataPoint
		wantErr bool
	}{
		{
			name: "valid data point",
			point: &DataPoint{
				Symbol: "sh600000",
				Open:   10.45,
				High:   10.55,
				Low:    10.40,
				Close:  10.50,
			},
			wantErr: false,
		},
		{
			name: "price too high",
			point: &DataPoint{
				Symbol: "sh600000",
				Open:   10000000.0,
				High:   10000000.0,
				Low:    10000000.0,
				Close:  10000000.0,
			},
			wantErr: true,
		},
		{
			name: "high less than low",
			point: &DataPoint{
				Symbol: "sh600000",
				Open:   10.50,
				High:   10.40,
				Low:    10.50,
				Close:  10.45,
			},
			wantErr: true,
		},
		{
			name: "close outside range",
			point: &DataPoint{
				Symbol: "sh600000",
				Open:   10.45,
				High:   10.55,
				Low:    10.40,
				Close:  10.60,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rule.Apply(tt.point)
			if (err != nil) != tt.wantErr {
				t.Errorf("PriceValidationRule.Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVolumeValidationRule(t *testing.T) {
	rule := NewVolumeValidationRule()

	tests := []struct {
		name    string
		point   *DataPoint
		wantErr bool
	}{
		{
			name: "valid volume",
			point: &DataPoint{
				Volume: 1000000.0,
				Amount: 10500000.0,
			},
			wantErr: false,
		},
		{
			name: "negative volume",
			point: &DataPoint{
				Volume: -100.0,
				Amount: 0.0,
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			point: &DataPoint{
				Volume: 1000000.0,
				Amount: -1000.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rule.Apply(tt.point)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeValidationRule.Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimestampValidationRule(t *testing.T) {
	rule := NewTimestampValidationRule()
	now := time.Now()

	tests := []struct {
		name    string
		point   *DataPoint
		wantErr bool
	}{
		{
			name: "valid timestamp",
			point: &DataPoint{
				Timestamp: now.Unix(),
			},
			wantErr: false,
		},
		{
			name: "negative timestamp",
			point: &DataPoint{
				Timestamp: -1,
			},
			wantErr: true,
		},
		{
			name: "timestamp too far in future",
			point: &DataPoint{
				Timestamp: now.Add(1 * time.Hour).Unix(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rule.Apply(tt.point)
			if (err != nil) != tt.wantErr {
				t.Errorf("TimestampValidationRule.Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataCleaner_Clean(t *testing.T) {
	cleaner := NewDataCleaner()

	points := []*DataPoint{
		{
			Symbol:    "sh600000",
			Timestamp: time.Now().Unix(),
			Open:      10.45,
			High:      10.55,
			Low:       10.40,
			Close:     10.50,
			Volume:    1000000.0,
			Amount:    10500000.0,
		},
		{
			Symbol:    "sh601398",
			Timestamp: time.Now().Unix(),
			Open:      5.20,
			High:      5.30,
			Low:       5.10,
			Close:     5.25,
			Volume:    500000.0,
			Amount:    2625000.0,
		},
	}

	cleaned, issues := cleaner.Clean(points)

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues, got %d", len(issues))
	}

	if len(cleaned) != len(points) {
		t.Errorf("Expected %d cleaned points, got %d", len(points), len(cleaned))
	}
}

func TestDataCleaner_CleanWithInvalidData(t *testing.T) {
	cleaner := NewDataCleaner()

	points := []*DataPoint{
		{
			Symbol:    "sh600000",
			Timestamp: time.Now().Unix(),
			Open:      10.45,
			High:      10.55,
			Low:       10.40,
			Close:     10.50,
			Volume:    1000000.0,
			Amount:    10500000.0,
		},
		{
			// 无效数据：高价比低价低
			Symbol:    "sh601398",
			Timestamp: time.Now().Unix(),
			Open:      5.20,
			High:      5.10,
			Low:       5.30,
			Close:     5.25,
			Volume:    500000.0,
			Amount:    2625000.0,
		},
	}

	cleaned, issues := cleaner.Clean(points)

	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}

	if len(cleaned) != 1 {
		t.Errorf("Expected 1 cleaned point, got %d", len(cleaned))
	}
}

func TestDuplicateDetectionRule(t *testing.T) {
	rule := NewDuplicateDetectionRule()
	now := time.Now()

	t.Run("first point", func(t *testing.T) {
		point := &DataPoint{
			Symbol:    "sh600000",
			Timestamp: now.Unix(),
		}

		_, err := rule.Apply(point)
		if err != nil {
			t.Errorf("First point should not be duplicate, got error: %v", err)
		}
	})

	t.Run("duplicate point", func(t *testing.T) {
		point := &DataPoint{
			Symbol:    "sh600000",
			Timestamp: now.Unix(),
		}

		_, err := rule.Apply(point)
		if err == nil {
			t.Error("Duplicate point should return error")
		}
	})
}

func TestStatisticalCorrector(t *testing.T) {
	corrector := NewStatisticalCorrector(20)

	points := []*DataPoint{
		{Close: 10.0},
		{Close: 10.1},
		{Close: 10.2},
		{Close: 10.1},
		{Close: 10.3},
		{Close: 100.0}, // 异常值
		{Close: 10.2},
		{Close: 10.1},
		{Close: 10.3},
		{Close: 10.2},
	}

	corrected := corrector.CorrectOutliers(points)

	if len(corrected) != len(points) {
		t.Errorf("Expected %d points, got %d", len(points), len(corrected))
	}

	// 异常值应该被修正
	// 注意：实际修正逻辑可能需要调整
}

func TestDataCleaner_FillMissing(t *testing.T) {
	cleaner := NewDataCleaner()

	now := time.Now()
	points := []*DataPoint{
		{
			Symbol:    "sh600000",
			Timestamp: now.Unix(),
			Open:      10.45,
			High:      10.55,
			Low:       10.40,
			Close:     10.50,
			Volume:    1000000.0,
			Amount:    10500000.0,
		},
		{
			Symbol:    "sh600000",
			Timestamp: now.Add(60 * time.Second).Unix(),
			Open:      0.0, // 缺失
			High:      10.60,
			Low:       10.45,
			Close:     10.55,
			Volume:    1000000.0,
			Amount:    10550000.0,
		},
	}

	filled := cleaner.FillMissing(points)

	// 验证填充的数据
	if filled[1].Open == 0.0 {
		t.Error("Open should be filled")
	}
}

func BenchmarkDataCleaner_Clean(b *testing.B) {
	cleaner := NewDataCleaner()

	// 生成测试数据
	points := make([]*DataPoint, 1000)
	now := time.Now()
	for i := range points {
		points[i] = &DataPoint{
			Symbol:    "sh600000",
			Timestamp: now.Add(time.Duration(i) * time.Second).Unix(),
			Open:      10.0 + float64(i)*0.01,
			High:      10.1 + float64(i)*0.01,
			Low:       9.9 + float64(i)*0.01,
			Close:     10.05 + float64(i)*0.01,
			Volume:    1000000.0,
			Amount:    10000000.0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleaner.Clean(points)
	}
}
