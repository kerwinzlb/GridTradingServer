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
	"runtime/debug"

	"github.com/gorilla/websocket"
	"github.com/kerwinzlb/GridTradingServer/log"

	"sync"
	"time"
)

type OKWSAgent struct {
	baseUrl string
	config  *Config
	conn    *websocket.Conn

	stopCh chan interface{}
	errCh  chan error

	callback   ReceivedDataCallback
	restart    ErrorCallback
	processMut sync.Mutex
}

func NewAgent(config *Config, f ErrorCallback) *OKWSAgent {
	a := &OKWSAgent{
		baseUrl: config.WSEndpoint,
		config:  config,
		errCh:   make(chan error),
		stopCh:  make(chan interface{}, 16),
		restart: f,
	}
	return a
}

func (a *OKWSAgent) Start() error {
	log.Debugf("Connecting to %s", a.baseUrl)
	c, _, err := websocket.DefaultDialer.Dial(a.baseUrl, nil)

	if err != nil {
		log.Errorf("websocket dial:%+v", err)
		return err
	} else {
		if a.config.IsPrint {
			log.Debugf("Connected to %s success", a.baseUrl)
		}
		a.conn = c
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
	a.callback = cb

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

func (a *OKWSAgent) work() {
	defer func() {
		a := recover()
		log.Infof("Work End. Recover msg: %+v", a)
		debug.PrintStack()
	}()

	defer a.Stop()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.keepalive()
		case <-a.errCh:
			a.Stop()
			a.restart()
		case <-a.stopCh:
			return

		}
	}
}

func (a *OKWSAgent) receive() {
	defer func() {
		a := recover()
		if a != nil {
			log.Infof("Receive End. Recover msg: %+v", a)
			debug.PrintStack()
		}
	}()

	for {
		select {
		case <-a.stopCh:
			return
		default:
		}
		_, message, err := a.conn.ReadMessage()
		if err != nil {
			log.Errorf("receive() ReadMessage error:%v", err)
			a.errCh <- err
		}
		// txtMsg := message
		// switch messageType {
		// case websocket.TextMessage:
		// case websocket.BinaryMessage:
		// 	txtMsg, err = a.GzipDecode(message)
		// }
		go a.callback(message)
		// rsp, err := loadResponse(txtMsg)
		// if err != nil {
		// 	continue
		// }
		// if rsp != nil {
		// 	if a.config.IsPrint {
		// 		log.Debugf("LoadedRep: %+v, err: %+v", rsp, err)
		// 	}
		// }

		// a.wsCh <- message
		// switch rsp.(type) {
		// case *WSOrdersResponse:
		// 	ord := rsp.(*WSOrdersResponse)
		// 	a.wsCh <- ord
		// default:
		// 	//log.Println(rsp)
	}
}
