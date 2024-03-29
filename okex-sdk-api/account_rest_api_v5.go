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

/*
查看账户配置
查看当前账户的配置信息。

限速：5次/2s
HTTP请求
GET /api/v5/account/config
*/

func (client *Client) GetAccountConfig() (*map[string]interface{}, error) {
	r := map[string]interface{}{}

	if _, err := client.Request(GET, ACCOUNT_CONFIG, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
