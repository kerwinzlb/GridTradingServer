package main

import (
	"sync/atomic"

	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	pb "github.com/kerwinzlb/GridTradingServer/proto"
)

var server *Server

type Server struct {
	conf        *okex.Config
	wsClient    *okex.OKWSAgent
	connectList atomic.Value
	// ConnectList  map[string]pb.Ws_GetOrderInfoServer
}

func NewServer(conf *okex.Config) {
	s := new(Server)
	s.conf = conf
	server = s
}

func (s *Server) ReceivedOrdersDataCallback(rspMsg []byte) error {
	res := new(okex.WSOrdersResponse)
	err := okex.JsonBytes2Struct(rspMsg, res)
	if err != nil {
		log.Error("ReceivedOrdersDataCallback", "JsonBytes2Struct err", err)
		return err
	}

	list := s.connectList.Load()
	if list != nil {
		stream := list.(map[string]pb.Ws_GetOrderInfoServer)[res.Arg.InstId]
		if stream != nil {
			r := new(pb.GetOrderResponse)
			r.Replybody = rspMsg
			err := stream.Send(r)
			if err != nil {
				log.Error("ReceivedOrdersDataCallback", "stream.Send err", err)
				return err
			}
		}
	}

	return nil
}

func (s *Server) Start() error {
	s.wsClient = okex.NewAgent(s.conf, s.ReceivedOrdersDataCallback, s.Start)
	s.wsClient.Start()
	err := s.wsClient.Login()
	if err != nil {
		return err
	}
	log.Info("websocket Login success")
	err = s.wsClient.Subscribe(okex.CHNL_OEDERS, okex.SPOT)
	if err != nil {
		return err
	}
	log.Info("websocket Subscribe success")
	return nil
}

func (s *Server) Stop() error {
	s.wsClient.UnSubscribe(okex.CHNL_OEDERS, okex.SPOT)
	s.wsClient.Stop()
	return nil
}

func (s *Server) GetOrderInfo(req *pb.GetOrderRequest, srv pb.Ws_GetOrderInfoServer) error {
	connectList := s.connectList.Load()
	if connectList != nil {
		list := connectList.(map[string]pb.Ws_GetOrderInfoServer)
		list[req.InstId] = srv
		s.connectList.Store(list)
	} else {
		list := make(map[string]pb.Ws_GetOrderInfoServer)
		list[req.InstId] = srv
		s.connectList.Store(list)
	}
	return nil
}
