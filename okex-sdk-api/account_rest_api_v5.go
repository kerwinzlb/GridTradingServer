package okex

/*
钱包账户信息
获取钱包账户所有资产列表，查询各币种的余额、冻结和可用等信息。

HTTP请求
GET /api/v5/account/balance
*/

func (client *Client) GetAccountBalance(currency string) (*map[string]interface{}, error) {
	r := map[string]interface{}{}
	req := NewReqParams()
	req.AddParam("currency", currency)
	url := BuildParams(ACCOUNT_BALANCE, req.Params())

	if _, err := client.Request(GET, url, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
