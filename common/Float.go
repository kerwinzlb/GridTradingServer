package common

import (
	"fmt"
	"strconv"
	"strings"
)

// 截取小数位数
func FloatRound(f float64, n int) float64 {
	format := "%." + strconv.Itoa(n) + "f"
	res, _ := strconv.ParseFloat(fmt.Sprintf(format, f), 64)
	return res
}

func FloatRoundLen(f string) int {
	strArr := strings.Split(f, ".")
	if len(strArr) > 1 {
		return len(strArr[1])
	}
	return 0
}
