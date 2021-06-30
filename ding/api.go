package ding

import (
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/kerwinzlb/GridTradingServer/log"
)

func PostRobotMessage(url, msg string) {
	//模拟一个post提交请求
	resp, err := http.Post(url, "application/json;charset=utf-8", strings.NewReader(`{"msgtype": "text", "text": {"content":"OKEX:`+"["+time.Now().Format("2006-01-02 15:04:05")+"]"+msg+`"}}`))
	if err != nil {
		log.Error("PostRobotMessage http.Post", "err", err)
		return
	}
	//关闭连接
	defer resp.Body.Close()
	//读取报文中所有内容
	body, err := ioutil.ReadAll(resp.Body)
	//输出内容
	log.Warn("PostRobotMessage", "resp.Body", string(body), "err", err)
}
