package main

import (
	"sync/atomic"
	"time"

	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	pb "github.com/kerwinzlb/GridTradingServer/proto"
)

var server *Server

type chanMap map[string]chan []byte

type Server struct {
	conf         *okex.Config
	wsClient     *okex.OKWSAgent
	orderChanMap atomic.Value
	stop         chan struct{} // Channel to wait for termination notifications
}

func NewServer(conf *okex.Config) {
	s := new(Server)
	s.conf = conf
	s.stop = make(chan struct{})
	server = s
}

func (s *Server) ReceivedOrdersDataCallback(rspMsg []byte) error {
	if string(rspMsg) == "pong" {
		return nil
	}
	res := new(okex.WSOrdersResponse)
	err := okex.JsonBytes2Struct(rspMsg, res)
	if err != nil {
		log.Error("ReceivedOrdersDataCallback JsonBytes2Struct", "rspMsg", string(rspMsg), "err", err)
		return err
	}

	orderChanMap := s.orderChanMap.Load()
	if orderChanMap != nil {
		instid := ""
		if len(res.Data) != 0 {
			instid = res.Data[0].InstId
		} else {
			return nil
		}
		orderChan := orderChanMap.(chanMap)[instid]
		if orderChan != nil {
			orderChan <- rspMsg
		}
	}
	return nil
}

func (s *Server) Start() error {
	s.wsClient = okex.NewAgent(s.conf, s.ReceivedOrdersDataCallback, s.Start)
	err := s.wsClient.Start()
	for err != nil {
		time.Sleep(time.Second)
		err = s.wsClient.Start()
	}
	err = s.wsClient.Login()
	if err != nil {
		log.Error("Start s.wsClient.Login", "err", err)
		return err
	}
	log.Info("websocket Login success")
	err = s.wsClient.Subscribe(okex.CHNL_OEDERS, okex.SPOT)
	if err != nil {
		log.Error("Start s.wsClient.Subscribe", "err", err)
		return err
	}
	log.Info("websocket Subscribe success")
	return nil
}

func (s *Server) Stop() error {
	s.wsClient.UnSubscribe(okex.CHNL_OEDERS, okex.SPOT)
	s.wsClient.Stop()
	close(s.stop)
	return nil
}

func (s *Server) GetOrderInfo(req *pb.GetOrderRequest, srv pb.Ws_GetOrderInfoServer) error {
	orderChan := make(chan []byte, 10)
	orderChanMap := s.orderChanMap.Load()
	if orderChanMap != nil {
		oChanMap := orderChanMap.(chanMap)
		oChanMap[req.InstId] = orderChan
		s.orderChanMap.Store(oChanMap)
	} else {
		oChanMap := make(chanMap)
		oChanMap[req.InstId] = orderChan
		s.orderChanMap.Store(oChanMap)
	}

	for {
		select {
		case msg := <-orderChan:
			r := new(pb.GetOrderResponse)
			r.Replybody = msg
			err := srv.Send(r)
			if err != nil {
				log.Error("GetOrderInfo stream.Send", "err", err)
				return err
			}
		case <-s.stop:
			break
		}

	}
	return nil
}
