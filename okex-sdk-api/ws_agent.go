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

	wsCh   chan interface{}
	stopCh chan interface{}
	errCh  chan error

	subMap     map[string][]ReceivedDataCallback
	processMut sync.Mutex
}

func (a *OKWSAgent) Start(config *Config) error {
	a.baseUrl = config.WSEndpoint
	log.Debugf("Connecting to %s", a.baseUrl)
	c, _, err := websocket.DefaultDialer.Dial(a.baseUrl, nil)

	if err != nil {
		log.Errorf("websocket dial:%+v", err)
		return err
	} else {
		if config.IsPrint {
			log.Debugf("Connected to %s success", a.baseUrl)
		}
		a.conn = c
		a.config = config

		a.wsCh = make(chan interface{})
		a.errCh = make(chan error)
		a.stopCh = make(chan interface{}, 16)
		a.subMap = make(map[string][]ReceivedDataCallback)

		go a.work()
		go a.receive()
		go a.finalize()
	}

	return nil
}

func (a *OKWSAgent) Subscribe(channel, instType string, cb ReceivedDataCallback) error {
	a.processMut.Lock()
	defer a.processMut.Unlock()

	// st := SubscriptionTopic{channel, filter}
	bo, err := subscribeOp(channel, instType)
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

	cbs := a.subMap[channel]
	if cbs == nil {
		cbs = []ReceivedDataCallback{}
	}

	if cb != nil {
		cbs = append(cbs, cb)
		// fullTopic, _ := st.ToString()
		a.subMap[channel] = cbs
		// a.subMap[fullTopic] = cbs
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

	a.subMap[channel] = nil

	return nil
}

func (a *OKWSAgent) Login(apiKey, passphrase string) error {

	timestamp := EpochTime()

	preHash := PreHashString(timestamp, GET, "/users/self/verify", "")
	if sign, err := HmacSha256Base64Signer(preHash, a.config.SecretKey); err != nil {
		return err
	} else {
		op, err := loginOp(apiKey, passphrase, timestamp, sign)
		data, err := Struct2JsonString(op)
		err = a.conn.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (a *OKWSAgent) keepalive() {
	a.ping()
}

func (a *OKWSAgent) Stop() error {
	defer func() {
		a := recover()
		log.Debugf("Stop End. Recover msg: %+v", a)
	}()

	close(a.stopCh)
	return nil
}

func (a *OKWSAgent) finalize() error {
	defer func() {
		log.Info("Finalize End. Connection to WebSocket is closed.")
	}()

	select {
	case <-a.stopCh:
		if a.conn != nil {
			close(a.errCh)
			close(a.wsCh)
			return a.conn.Close()
		}
	}

	return nil
}

func (a *OKWSAgent) ping() {
	msg := "ping"
	// log.Debugf("Send Msg: %s", msg)
	a.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (a *OKWSAgent) GzipDecode(in []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(in))
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func (a *OKWSAgent) handleResponse(r interface{}) error {
	channel := ""
	switch r.(type) {
	case *WSOrdersResponse:
		channel = r.(*WSOrdersResponse).Arg.Channel
	default:
		return nil
	}

	cbs := a.subMap[channel]
	if cbs != nil {
		for i := 0; i < len(cbs); i++ {
			cb := cbs[i]
			go cb(r)
		}
	}
	return nil
}

func (a *OKWSAgent) work() {
	defer func() {
		log.Info("Work End.")
	}()

	defer a.Stop()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.keepalive()
		case tb := <-a.wsCh:
			a.handleResponse(tb)
		case err := <-a.errCh:
			c, _, err := websocket.DefaultDialer.Dial(a.baseUrl, nil)
			if err != nil {
				log.Errorf("websocket dial:%+v", err)
			} else {
				a.conn = c
				a.Login(a.config.ApiKey, a.config.Passphrase)
				a.Subscribe(CHNL_OEDERS, "SPOT", nil)
			}
		case <-a.stopCh:
			return

		}
	}
}

func (a *OKWSAgent) receive() {
	defer func() {
		log.Info("Receive End.")
	}()

	for {
		select {
		case <-a.stopCh:
			return
		default:
		}
		messageType, message, err := a.conn.ReadMessage()
		if err != nil {
			log.Errorf("receive() ReadMessage error:%v", err)
			a.errCh <- err
		}
		txtMsg := message
		switch messageType {
		case websocket.TextMessage:
		case websocket.BinaryMessage:
			txtMsg, err = a.GzipDecode(message)
		}

		rsp, err := loadResponse(txtMsg)
		if err != nil {
			continue
		}
		if rsp != nil {
			if a.config.IsPrint {
				log.Debugf("LoadedRep: %+v, err: %+v", rsp, err)
			}
		}

		switch rsp.(type) {
		case *WSOrdersResponse:
			ord := rsp.(*WSOrdersResponse)
			a.wsCh <- ord
		default:
			//log.Println(rsp)
		}
	}
}
