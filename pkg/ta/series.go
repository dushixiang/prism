package ta

import "github.com/dushixiang/prism/pkg/exchange"

func Last(s []float64, position int) float64 {
	return s[len(s)-1-position]
}

func Crossover(s1, s2 []float64) bool {
	return Last(s1, 0) > Last(s2, 0) && Last(s1, 1) <= Last(s2, 1)
}

func Crossunder(s1, s2 []float64) bool {
	return Last(s1, 0) <= Last(s2, 0) && Last(s1, 1) > Last(s2, 1)
}

func Cross(s1, s2 []float64) bool {
	return Crossunder(s1, s2) || Crossover(s1, s2)
}

func LastValues(s []float64, size int) []float64 {
	if l := len(s); l > size {
		return s[l-size:]
	}
	return s
}

func RemoveLast(arr []float64) []float64 {
	if len(arr) > 0 {
		return arr[:len(arr)-1] // 返回去掉最后一个元素的切片
	}
	return arr // 如果切片为空，直接返回
}

// Lowest 函数用于计算最近 n 根K线中的最低价
func Lowest(low []float64, period int) float64 {
	arr := LastValues(low, period)
	minVal := arr[0]

	for _, value := range arr {
		if value < minVal {
			minVal = value
		}
	}
	return minVal
}

// Highest 函数用于计算最近 n 根K线中的最高价
func Highest(high []float64, period int) float64 {
	arr := LastValues(high, period)
	maxVal := arr[0]

	for _, value := range arr {
		if value > maxVal {
			maxVal = value
		}
	}
	return maxVal
}

// HighLow 函数用于计算K线切片中的最高价和最低价
func HighLow(klines []*exchange.Kline) (float64, float64) {
	if len(klines) == 0 {
		return 0, 0
	}

	high := klines[0].High
	low := klines[0].Low

	for _, k := range klines {
		if k.High > high {
			high = k.High
		}
		if k.Low < low {
			low = k.Low
		}
	}
	return high, low
}
