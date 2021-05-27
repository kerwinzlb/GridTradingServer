package okex

/*
 OKEX market contract api response results
 @author Tony Tian
 @date 2018-03-17
 @version 1.0.0
*/

type Instrument struct {
	InstType  string `json:"instType"`  //产品类型
	InstId    string `json:"instId"`    //产品id， 如 BTC-USD-SWAP
	Uly       string `json:"uly"`       //合约标的指数，如 BTC-USD，仅适用于交割/永续/期权
	Category  string `json:"category"`  //手续费档位，每个交易产品属于哪个档位手续费
	BaseCcy   string `json:"baseCcy"`   //交易货币币种，如 BTC-USDT 中的 BTC ，仅适用于币币
	QuoteCcy  string `json:"quoteCcy"`  //计价货币币种，如 BTC-USDT 中的USDT ，仅适用于币币
	SettleCcy string `json:"settleCcy"` //盈亏结算和保证金币种，如 BTC 仅适用于交割/永续/期权
	CtVal     string `json:"ctVal"`     //合约面值，仅适用于交割/永续/期权
	CtMult    string `json:"ctMult"`    //合约乘数，仅适用于交割/永续/期权
	CtValCcy  string `json:"ctValCcy"`  //合约面值计价币种，仅适用于交割/永续/期权
	OptType   string `json:"optType"`   //期权类型，C或P 仅适用于期权
	Stk       string `json:"stk"`       //行权价格，仅适用于期权
	ListTime  string `json:"listTime"`  //上线日期，Unix时间戳的毫秒数格式，如 1597026383085
	ExpTime   string `json:"expTime"`   //交割/行权日期，仅适用于交割 和 期权，Unix时间戳的毫秒数格式，如 1597026383085
	Lever     string `json:"lever"`     //杠杆倍数，不适用于币币，用于区分币币和币币杠杆
	TickSz    string `json:"tickSz"`    //下单价格精度，如 0.0001
	LotSz     string `json:"lotSz"`     //下单数量精度，如 BTC-USDT-SWAP：1
	MinSz     string `json:"minSz"`     //最小下单数量
	CtType    string `json:"ctType"`    //linear：正向合约；inverse：反向合约。仅交割/永续有效
	Alias     string `json:"alias"`     //合约日期别名。this_week：本周；next_week：次周；quarter：季度；next_quarter：次季度。仅适用于交割
	State     string `json:"state"`     //产品状态。live：交易中；suspend：暂停中；preopen：预上线
}

type InstrumentResult struct {
	PublicResult
	Data []Instrument
}
