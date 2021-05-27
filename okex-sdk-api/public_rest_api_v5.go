package okex

/*
获取所有可交易产品的信息列表。

限速： 20次/2s
限速规则：IP +instType
HTTP请求
GET /api/v5/public/instruments
*/
func (client *Client) GetPublicInstruments(instType, uly, instId string) (*InstrumentResult, error) {
	r := new(InstrumentResult)

	req := NewReqParams()
	req.AddParam("instType", instType)
	req.AddParam("uly", uly)
	req.AddParam("instId", instId)
	url := BuildParams(PUBLIC_INSTRUMENTS, req.Params())
	if _, err := client.Request(GET, url, nil, r); err != nil {
		return nil, err
	}
	return r, nil
}
