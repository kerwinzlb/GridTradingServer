package okex

/*
获取全部ticker信息
获取平台全部币对的最新成交价、买一价、卖一价和24小时交易量的快照信息。

限速规则：50次/2s
HTTP请求
GET /api/v5/market/tickers
*/
func (client *Client) GetMarketTickers(instType string, uly string) (*map[string]interface{}, error) {
	r := map[string]interface{}{}
	req := NewReqParams()
	req.AddParam("instType", instType)
	req.AddParam("uly", uly)
	url := BuildParams(MARKET_TICKERS, req.Params())
	if _, err := client.Request(GET, url, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

/*
获取某个ticker信息
获取币对的最新成交价、买一价、卖一价和24小时交易量的快照信息。

限速规则：20次/2s
HTTP请求
GET /api/v5/market/ticker
*/
func (client *Client) GetMarketTicker(instId string) (*TicketResult, error) {
	r := new(TicketResult)

	req := NewReqParams()
	req.AddParam("instId", instId)
	url := BuildParams(MARKET_TICKER, req.Params())
	if _, err := client.Request(GET, url, nil, r); err != nil {
		return nil, err
	}
	return r, nil
}
