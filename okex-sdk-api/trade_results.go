package okex

/*
 OKEX market contract api response results
 @author Tony Tian
 @date 2018-03-17
 @version 1.0.0
*/

type Order struct {
	OrdId   string `json:"ordId"`   //订单ID
	ClOrdId string `json:"clOrdId"` //客户自定义订单ID
	Tag     string `json:"tag"`     //订单标签
	SCode   string `json:"sCode"`   //事件执行结果的code，0代表成功
	SMsg    string `json:"sMsg"`    //事件执行失败时的msg
}

type OrderResult struct {
	PublicResult
	Data []Order
}

type PendingOrderResult struct {
	PublicResult
	Data []DataOrder
}
