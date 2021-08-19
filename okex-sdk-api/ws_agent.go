package okex

/*
 OKEX websocket API agent
 @author Lingting Fu
 @date 2018-12-27
 @version 1.0.0
*/

import (
	"bytes"
	"compress/flate"
	"io/ioutil"

	"github.com/gorilla/websocket"
	"github.com/kerwinzlb/GridTradingServer/log"

	"sync"
	"time"
)

type OKWSAgent struct {
	baseUrl string
	config  *Config
	conn    *websocket.Conn

	wCh    chan struct{}
	stopCh chan interface{}

	callback   ReceivedDataCallback
	restart    ErrorCallback
	processMut sync.Mutex
}

func NewAgent(config *Config, c ReceivedDataCallback, f ErrorCallback) *OKWSAgent {
	a := &OKWSAgent{
		baseUrl:  config.WSEndpoint,
		config:   config,
		wCh:      make(chan struct{}, 100),
		stopCh:   make(chan interface{}, 16),
		callback: c,
		restart:  f,
	}
	return a
}

func (a *OKWSAgent) Start() error {
	c, _, err := websocket.DefaultDialer.Dial(a.baseUrl, nil)
	if err != nil {
		log.Error("websocket dial", "err", err)
		return err
	} else {
		if a.config.IsPrint {
			log.Debugf("Connected to %s success", a.baseUrl)
		}
		a.conn = c
		go a.work()
		go a.receive()
	}

	return nil
}

func (a *OKWSAgent) Subscribe(channel, instType, instId string) error {
	a.processMut.Lock()
	defer a.processMut.Unlock()

	// st := SubscriptionTopic{channel, filter}
	bo, err := subscribeOp(channel, instType, instId)
	if err != nil {
		return err
	}

	msg, err := Struct2JsonString(bo)
	if a.config.IsPrint {
		log.Debugf("Send Msg: %s", msg)
	}
	if err := a.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return err
	}
	return nil
}

func (a *OKWSAgent) UnSubscribe(channel, instType string) error {
	a.processMut.Lock()
	defer a.processMut.Unlock()

	// st := SubscriptionTopic{channel, filter}
	bo, err := unsubscribeOp(channel, instType)
	if err != nil {
		return err
	}

	msg, err := Struct2JsonString(bo)
	if a.config.IsPrint {
		log.Debugf("Send Msg: %s", msg)
	}
	if err := a.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return err
	}

	return nil
}

func (a *OKWSAgent) Login() error {

	timestamp := EpochTime()

	preHash := PreHashString(timestamp, GET, "/users/self/verify", "")
	if sign, err := HmacSha256Base64Signer(preHash, a.config.SecretKey); err != nil {
		return err
	} else {
		op, err := loginOp(a.config.ApiKey, a.config.Passphrase, timestamp, sign)
		data, err := Struct2JsonString(op)
		err = a.conn.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (a *OKWSAgent) Stop() error {
	close(a.stopCh)
	return nil
}

func (a *OKWSAgent) ping() {
	msg := "ping"
	err := a.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		log.Error("ping() WriteMessage", "err", err)
	}
}

func (a *OKWSAgent) GzipDecode(in []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(in))
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func (a *OKWSAgent) work() {
	defer func() {
		log.Info("Work End. ")
	}()

	pingTic := time.NewTicker(10 * time.Second)
	defer pingTic.Stop()
	timeoutTic := new(time.Ticker)
	defer timeoutTic.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-pingTic.C:
			pingTic.Stop()
			a.ping()
			timeoutTic = time.NewTicker(5 * time.Second)
		case <-timeoutTic.C:
			timeoutTic.Stop()
			log.Error("work() timeoutTic trigger, Websocket to restart!")
			a.Stop()
			a.restart()
			continue
		case <-a.wCh:
			pingTic = time.NewTicker(10 * time.Second)
			timeoutTic.Stop()
		}
	}
}

func (a *OKWSAgent) receive() {
	defer func() {
		log.Info("Receive End. ")
	}()

	for {
		select {
		case <-a.stopCh:
			if a.conn != nil {
				a.conn.Close()
			}
			return
		default:
		}
		_, message, err := a.conn.ReadMessage()
		if err != nil {
			log.Error("receive() ReadMessage", "message", message, "err", err)
			time.Sleep(3 * time.Second)
			continue
		}
		a.wCh <- struct{}{}
		go a.callback(message)
	}
}
