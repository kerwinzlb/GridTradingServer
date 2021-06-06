package server

type Config struct {
	InstId string //产品ID
	BuyAmt float64 //buy交易金额
	SellAmt float64 //sell交易金额
	BuyNum float64 //buy交易数量
	SellNum float64 //sell交易数量
	BuyGridSize float64 //buy格子区间
	SellGridSize float64 //sell格子区间
	GridNum int//格子数量
	Mode int //交易模式：0-U本位；1-币本位
	StopSec int //停服判断周期
	StopCnt int //停服判断交易数量
	Status int //服务状态
}