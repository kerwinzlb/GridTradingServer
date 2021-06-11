package main

import (
	"context"
	"encoding/hex"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/db"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	pb "github.com/kerwinzlb/GridTradingServer/proto"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc"
)

const (
	MGO_DB_NAME                = "grid"
	MGO_COLLECTION_CONFIG_NAME = "config"
	MGO_COLLECTION_TICKET_NAME = "ticket"
	MGO_COLLECTION_ORDER_NAME  = "order"
)

type Server struct {
	conf       *okex.Config
	mgoConf    atomic.Value
	instId     string
	tickSzN    int
	lotSzN     int
	minSzN     int
	gridNum    int
	restClient *okex.Client
	steam      pb.Ws_GetOrderInfoClient
	mgo        *db.Mongo
	status     atomic.Value
	lock       *sync.RWMutex

	stop chan struct{} // Channel to wait for termination notifications
}

func New(instId string, conf *okex.Config) (*Server, error) {
	confCopy := *conf
	s := &Server{
		conf:       &confCopy,
		instId:     instId,
		restClient: okex.NewClient(*conf),
		mgo:        db.NewMgo(conf.MgoEndpoint),
		lock:       new(sync.RWMutex),
		stop:       make(chan struct{}),
	}

	s.status.Store(0)
	pubRes, err := s.restClient.GetPublicInstruments(okex.SPOT, "", s.instId)
	if err != nil {
		return nil, err
	}
	s.tickSzN = common.FloatRoundLen(pubRes.Data[0].TickSz)
	s.lotSzN = common.FloatRoundLen(pubRes.Data[0].LotSz)
	s.minSzN = common.FloatRoundLen(pubRes.Data[0].MinSz)

	err = s.mgo.Connect()
	if err != nil {
		return nil, err
	}
	mgoConf, err := s.GetMgoConfig()
	if err != nil {
		return nil, err
	}
	s.mgoConf.Store(mgoConf)
	err = s.grpcConnect()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) grpcConnect() error {
	conn, err := grpc.Dial(s.conf.WsServerAddr+":"+strconv.Itoa(s.conf.WsServerPort), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Error("grpcConnect", "grpc.Dial err", err)
		return err
	}
	c := pb.NewWsClient(conn)

	req := new(pb.GetOrderRequest)
	req.InstId = s.instId

	r, err := c.GetOrderInfo(context.Background(), req)
	if err != nil {
		log.Error("grpcConnect", "GetOrderInfo err", err)
		return err
	}
	s.steam = r
	return nil
}

func (s *Server) getSz(pric float64, side string, conf Config) (string, string) {
	sz := ""
	if conf.Mode == 0 {
		if side == "buy" {
			sz = strconv.FormatFloat(conf.BuyNum, 'f', s.lotSzN, 64)
		} else if side == "sell" {
			sz = strconv.FormatFloat(conf.SellNum, 'f', s.lotSzN, 64)
		}
	} else if conf.Mode == 1 {
		if side == "buy" {
			sz = strconv.FormatFloat(conf.BuyAmt/(pric), 'f', s.lotSzN, 64)
		} else if side == "sell" {
			sz = strconv.FormatFloat(conf.SellAmt/(pric), 'f', s.lotSzN, 64)
		}
	}
	px := strconv.FormatFloat(pric, 'f', s.tickSzN, 64)
	return px, sz
}

func (s *Server) initPostOrder() error {
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		return err
	}
	// pric, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	// s.sz = strconv.FormatFloat(s.conf.Amount/pric, 'f', s.lotSzN, 64)
	s.InsertTicker(ticRes.Data[0])
	last, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	index := 0
	conf := s.mgoConf.Load().(Config)
	for i := s.gridNum; i > 0; i-- {
		pric := last / math.Pow((1+conf.BuyGridSize), float64(i))
		px, sz := s.getSz(pric, "buy", conf)
		clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - i)))

		_, err := s.PostBuyTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
		if err != nil {
			return err
		}

		pric = last * math.Pow((1+conf.SellGridSize), float64(i))
		px, sz = s.getSz(pric, "sell", conf)
		px = strconv.FormatFloat(pric, 'f', s.tickSzN, 64)
		clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + i)))
		_, err = s.PostSellTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) GetMgoConfig() (Config, error) {
	conf := new(Config)
	err := s.mgo.FindOne(MGO_DB_NAME, MGO_COLLECTION_CONFIG_NAME, bson.M{"instId": s.instId}, conf)
	if err != nil {
		return Config{}, err
	}
	return *conf, nil
}

func (s *Server) Monitor() {
	mticker := time.NewTicker(15 * time.Second)
	defer mticker.Stop()
	rticker := time.NewTicker(30 * time.Second)
	defer rticker.Stop()

	for {
		select {
		case <-mticker.C:
			conf, err := s.GetMgoConfig()
			if err != nil {
				log.Error("Monitor", "GetMgoConfig err", err)
				continue
			}
			status := s.status.Load()
			if status == 0 {
				if conf.Status == 1 {
					s.Start()
				}
			} else if status == 1 {
				if conf.Status == 0 {
					s.Stop()
				} else if conf.Status == 1 {
					mgoConf := s.mgoConf.Load().(Config)
					if !reflect.DeepEqual(mgoConf, conf) {
						s.mgoConf.Store(conf)
					}
				}
			}
		case <-rticker.C:
			s.monitorOrder()
		case <-s.stop:
			return
		}
	}
}

func (s *Server) monitorOrder() {
	trdp, err := s.GetTradeOrdersPending()
	if err != nil {
		log.Error("monitorOrder", "GetTradeOrdersPending err", err)
	}
	if len(trdp.Data) != s.gridNum*2 {
		s.CancelAllOrders()
		s.initPostOrder()
	}
}

func (s *Server) wsOrderRecv() {
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		r, err := s.steam.Recv()
		if err != nil {
			log.Error("wsOrderRecv", "s.steam.Recv err", err)
			err = s.grpcConnect()
			if err != nil {
				log.Error("wsOrderRecv", "grpcConnect err", err)
			}
		}
		go s.ReceivedOrdersDataCallback(r.Replybody)
	}
}

func (s *Server) Start() error {
	conf := s.mgoConf.Load().(Config)
	s.gridNum = conf.GridNum
	err := s.initPostOrder()
	if err != nil {
		return err
	}
	s.status.Store(1)
	return nil
}

func (s *Server) InsertTicker(ticket okex.Ticket) error {
	last, _ := strconv.ParseFloat(ticket.Last, 64)
	ts, _ := strconv.ParseInt(ticket.Ts, 10, 64)
	err := s.mgo.InsertOne(MGO_DB_NAME, MGO_COLLECTION_TICKET_NAME, bson.M{"instId": ticket.InstId, "last": last, "ts": ts})
	if err != nil {
		log.Error("InsertTicker", "error", err)
		return err
	}
	return nil
}

func (s *Server) InsertOrder(index int, order okex.DataOrder) error {
	px, _ := strconv.ParseFloat(order.Px, 64)
	sz, _ := strconv.ParseFloat(order.Sz, 64)
	avgPx, _ := strconv.ParseFloat(order.AvgPx, 64)
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	fillTime, _ := strconv.ParseInt(order.FillTime, 10, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	err := s.mgo.InsertOne(MGO_DB_NAME, MGO_COLLECTION_ORDER_NAME, bson.M{"index": index, "instId": order.InstId, "side": order.Side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
	if err != nil {
		log.Error("InsertOrder", "error", err)
		return err
	}
	return nil
}

func (s *Server) shutdown(pri float64, conf Config) {
	if pri <= conf.LowerLimit || pri >= conf.UpperLimit {
		s.Stop()
		_, err := s.mgo.UpdateOne(MGO_DB_NAME, MGO_COLLECTION_CONFIG_NAME, bson.M{"instId": s.instId}, bson.M{"status": 0})
		if err != nil {
			log.Error("shutdown", "UpdateOne err", err)
		}
	}
}

func (s *Server) ReceivedOrdersDataCallback(rspMsg []byte) error {
	res := new(okex.WSOrdersResponse)
	err := okex.JsonBytes2Struct(rspMsg, res)
	if err != nil {
		log.Error("ReceivedOrdersDataCallback", "JsonBytes2Struct err", err)
		return err
	}
	for _, order := range res.Data {
		if order.Code == "0" && order.InstType == okex.SPOT && order.OrdType == "post_only" {
			if order.State == "filled" {
				conf := s.mgoConf.Load().(Config)
				buyGridSize := 1 + conf.BuyGridSize
				sellGridSize := 1 + conf.SellGridSize
				clOrdId, _ := hex.DecodeString(strings.Trim(order.ClOrdId, s.instId))
				index, _ := strconv.Atoi(string(clOrdId))
				go s.InsertOrder(index, order)
				pri, _ := strconv.ParseFloat(order.Px, 64)
				s.shutdown(pri, conf)
				if order.Side == "buy" {
					log.Debug("买单成交")
					pric := pri * sellGridSize
					px, sz := s.getSz(pric, "sell", conf)
					clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index + 1)))
					_, err := s.PostSellTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("买单成交，挂卖单失败", "error", err)
						return err
					}
					log.Debug("买单成交，挂卖单成功")

					pric = pri / math.Pow(buyGridSize, float64(s.gridNum))
					px, sz = s.getSz(pric, "buy", conf)
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum)))
					_, err = s.PostBuyTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("买单成交，挂买单失败", "error", err)
						return err
					}
					log.Debug("买单成交，挂买单成功")
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum + 1)))
					_, err = s.restClient.PostTradeCancelOrder(s.instId, "", strings.Split(s.instId, "-")[0]+clOrdId)
					if err != nil {
						log.Error("买单成交，撤销卖单失败", "error", err)
						return err
					}
					log.Debug("买单成交，撤销卖单成功")
				} else if order.Side == "sell" {
					log.Debug("卖单成交")
					pric := pri / buyGridSize
					px, sz := s.getSz(pric, "buy", conf)
					clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - 1)))
					_, err := s.PostBuyTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("卖单成交，挂买单失败", "error", err)
						return err
					}
					log.Debug("卖单成交，挂买单成功")
					pric = pri * math.Pow(sellGridSize, float64(s.gridNum))
					px, sz = s.getSz(pric, "sell", conf)
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum)))
					_, err = s.PostSellTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("卖单成交，挂卖单失败", "error", err)
						return err
					}
					log.Debug("卖单成交，挂卖单成功")
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum - 1)))
					_, err = s.restClient.PostTradeCancelOrder(s.instId, "", strings.Split(s.instId, "-")[0]+clOrdId)
					if err != nil {
						log.Error("卖单成交，撤销买单失败", "error", err)
						return err
					}
					log.Debug("卖单成交，撤销买单成功")
				}
			}
		}
	}

	return nil
}

func (s *Server) PostTradeCancelBatchOrders(ordIds, clOrdIds []string) error {
	reqs := make([]map[string]string, 0)
	for i := 0; i < len(ordIds); i++ {
		req := okex.NewParams()
		req["instId"] = s.instId
		req["ordId"] = ordIds[i]
		req["clOrdId"] = clOrdIds[i]
		reqs = append(reqs, req)
	}
	_, err := s.restClient.PostTradeCancelBatchOrders(&reqs)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) PostBuyTradeOrder(clOrdId, px, sz string) (string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	// req["ccy"] = "USDT"
	req["clOrdId"] = clOrdId
	req["side"] = "buy"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", err
	}
	return res.Data[0].OrdId, nil
}

func (s *Server) PostSellTradeOrder(clOrdId, px, sz string) (string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	// req["ccy"] = "USDT"
	req["clOrdId"] = clOrdId
	req["side"] = "sell"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", err
	}
	return res.Data[0].OrdId, nil
}

func (s *Server) GetTradeOrdersPending() (*okex.PendingOrderResult, error) {
	req := okex.NewReqParams()
	req.AddParam("instType", okex.SPOT)
	req.AddParam("instId", s.instId)
	req.AddParam("ordType", "post_only")
	return s.restClient.GetTradeOrdersPending(req)
}

func (s *Server) CancelAllOrders() {
	trdp, err := s.GetTradeOrdersPending()
	if err != nil {
		log.Error("CancelAllOrders", "GetTradeOrdersPending err", err)
	}
	for _, order := range trdp.Data {
		_, err := s.restClient.PostTradeCancelOrder(s.instId, order.OrdId, "")
		if err != nil {
			log.Error("CancelAllOrders", "PostTradeCancelOrder err", err)
		}
	}
}

func (s *Server) Stop() error {
	s.CancelAllOrders()

	s.mgo.DisConnect()
	s.status.Store(0)
	return nil
}

func (s *Server) Exit() error {
	s.Stop()
	close(s.stop)
	return nil
}

// Wait blocks the thread until the node is stopped. If the node is not running
// at the time of invocation, the method immediately returns.
func (s *Server) Wait() {
	stop := s.stop

	<-stop
	os.Exit(1)
}
