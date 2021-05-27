package okex

/*
 OKEX websocket api wrapper
 @author Lingting Fu
 @date 2018-12-27
 @version 1.0.0
*/

type ArgOrder struct {
	Channel  string `json:"channel"`  //频道名
	Uly      string `json:"uly"`      //合约标的指数
	InstType string `json:"instType"` //产品类型
	InstId   string `json:"instId"`   //产品ID
}

type DataOrder struct {
	InstType    string `json:"instType"`    //产品类型
	InstId      string `json:"instId"`      //产品ID
	Ccy         string `json:"ccy"`         //保证金币种，仅适用于单币种保证金账户下的全仓币币杠杆订单
	OrdId       string `json:"ordId"`       //订单ID
	ClOrdId     string `json:"clOrdId"`     //由用户设置的订单ID来识别您的订单
	Tag         string `json:"tag"`         //订单标签
	Px          string `json:"px"`          //委托价格
	Sz          string `json:"sz"`          //原始委托数量，币币/币币杠杆，以币为单位；交割/永续/期权 ，以张为单位
	OrdType     string `json:"ordType"`     //订单类型 market：市价单；limit：限价单；post_only： 只做maker单；fok：全部成交或立即取消单；ioc：立即成交并取消剩余单
	Side        string `json:"side"`        //订单方向，buy sell
	PosSide     string `json:"posSide"`     //持仓方向 long：双向持仓多头；short：双向持仓空头；net：单向持仓
	TdMode      string `json:"tdMode"`      //交易模式  保证金模式 isolated：逐仓 cross：全仓；非保证金模式 cash：现金
	FillPx      string `json:"fillPx"`      //最新成交价格
	TradeId     string `json:"tradeId"`     //最新成交ID
	FillSz      string `json:"fillSz"`      //最新成交数量
	FillTime    string `json:"fillTime"`    //最新成交时间
	FillFee     string `json:"fillFee"`     //最新一笔成交的手续费
	FillFeeCcy  string `json:"fillFeeCcy"`  //最新一笔成交的手续费币种
	ExecType    string `json:"execType"`    //最新一笔成交的流动性方向 T：taker M maker
	AccFillSz   string `json:"accFillSz"`   //累计成交数量
	AvgPx       string `json:"avgPx"`       //成交均价，如果成交数量为0，该字段也为0
	State       string `json:"state"`       //订单状态 canceled：撤单成功；live：等待成交；partially_filled： 部分成交；filled：完全成交
	Lever       string `json:"lever"`       //杠杆倍数，0.01到125之间的数值，仅适用于 币币杠杆/交割/永续
	TpTriggerPx string `json:"tpTriggerPx"` //止盈触发价
	TpOrdPx     string `json:"tpOrdPx"`     //止盈委托价，止盈委托价格为-1时，执行市价止盈
	SlTriggerPx string `json:"slTriggerPx"` //止损触发价
	SlOrdPx     string `json:"slOrdPx"`     //止损委托价，止损委托价格为-1时，执行市价止损
	FeeCcy      string `json:"feeCcy"`      //交易手续费币种 币币/币币杠杆：如果是买的话，收取的就是BTC；如果是卖的话，收取的就是USDT；交割/永续/期权 收取的就是保证金
	Fee         string `json:"fee"`         //订单交易手续费，平台向用户收取的交易手续费
	RebateCcy   string `json:"rebateCcy"`   //返佣金币种 ，如果没有返佣金，该字段为“”
	Rebate      string `json:"rebate"`      //返佣金额，平台向达到指定lv交易等级的用户支付的挂单奖励（返佣），如果没有返佣金，该字段为“”
	Pnl         string `json:"pnl"`         //收益
	Category    string `json:"category"`    //订单种类分类 normal：普通委托订单种类；twap：TWAP订单种类；adl：ADL订单种类；full_liquidation：爆仓订单种类；partial_liquidation：减仓订单种类
	UTime       string `json:"uTime"`       //订单更新时间， Unix时间戳的毫秒数格式，如 1597026383085
	CTime       string `json:"cTime"`       //订单创建时间， Unix时间戳的毫秒数格式，如 1597026383085
	ReqId       string `json:"reqId"`       //修改订单时使用的request ID，如果没有修改，该字段为""
	AmendResult string `json:"amendResult"` //修改订单的结果 -1： 失败；0：成功；1：自动撤单（因为修改成功导致订单自动撤销）。通过API修改订单时，如果cxlOnFail设置为false且修改失败后，则amendResult返回 -1。通过API修改订单时，如果cxlOnFail设置为true且修改失败后，则amendResult返回1。通过Web/APP修改订单时，如果修改失败后，则amendResult返回-1
	Code        string `json:"code"`        //错误码，默认为0
	Msg         string `json:"msg"`         //错误消息，默认为""
}

type WSOrdersResponse struct {
	Arg  ArgOrder
	Data []DataOrder
}
