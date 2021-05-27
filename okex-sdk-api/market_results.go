package okex

/*
 OKEX market contract api response results
 @author Tony Tian
 @date 2018-03-17
 @version 1.0.0
*/

type Ticket struct {
	InstType  string `json:"instType"` //产品类型
	InstId    string `json:"instId"`   //产品ID
	Last      string `json:"last"`     //最新成交价
	LastSz    string `json:"lastSz"`   //最新成交的数量
	AskPx     string `json:"askPx"`    //卖一价
	AskSz     string `json:"askSz"`    //卖一价对应的数量
	BidPx     string `json:"bidPx"`    //买一价
	BidSz     string `json:"bidSz"`    //买一价对应的数量
	Open24h   string `json:"open24h"`  //24小时开盘价
	High24h   string `json:"high24h"`  //24小时最高价
	Low24h    string `json:"low24h"`   //24小时最低价
	VolCcy24h string `json:"low24h"`   //24小时成交量，以币为单位。如果是衍生品合约，数值为结算货币的数量。如果是币币/币币杠杆，数值为计价货币的数量。
	Vol24h    string `json:"vol24h"`   //24小时成交量，以张为单位。如果是衍生品合约，数值为合约的张数。如果是币币/币币杠杆，数值为交易货币的数量。
	SodUtc0   string `json:"sodUtc0"`  //UTC 0 时开盘价
	SodUtc8   string `json:"sodUtc8"`  //UTC+8 时开盘价
	Ts        string `json:"ts"`       //ticker数据产生时间，Unix时间戳的毫秒数格式，如 1597026383085
}

type TicketResult struct {
	PublicResult
	Data []Ticket
}
